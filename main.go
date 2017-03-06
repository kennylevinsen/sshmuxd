package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"

	"github.com/fsnotify/fsnotify"
	"github.com/joushou/sshmux"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

type Host struct {
	Address string   `json:"address"`
	Users   []string `json:"users"`
	NoAuth  bool     `json:"noAuth"`
}

var configFile = flag.String("config", "", "User-supplied configuration file to use")

func parseAuthFile(filename string) ([]*sshmux.User, error) {
	var users []*sshmux.User

	authFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Parse authfile as authorized_key

	for len(authFile) > 0 {
		switch authFile[0] {
		case '\n', '\r', '\t', ' ':
			authFile = authFile[1:]
			continue
		}

		var (
			pk      ssh.PublicKey
			comment string
		)

		pk, comment, _, authFile, err = ssh.ParseAuthorizedKey(authFile)
		if err != nil {
			return nil, err
		}

		u := &sshmux.User{
			PublicKey: pk,
			Name:      comment,
		}

		users = append(users, u)
	}

	return users, nil
}

func main() {
	flag.Parse()
	viper.SetDefault("address", ":22")
	viper.SetDefault("hostkey", "hostkey")
	viper.SetDefault("authkeys", "authkeys")

	viper.SetConfigName("sshmuxd")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.sshmuxd")
	viper.AddConfigPath("/etc/sshmuxd/")

	viper.SetConfigFile(*configFile)

	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Error parsing the config file: %s\n", err))
	}
	log.Printf("Config File used: %s", viper.ConfigFileUsed())

	hosts := make([]Host, 0)
	err = viper.UnmarshalKey("hosts", &hosts)
	if err != nil {
		panic(fmt.Errorf("Error parsing the config file hosts list: %s\n", err))
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Println("Config file changed:", e.Name)
		nh := make([]Host, 0)
		err = viper.UnmarshalKey("hosts", &nh)
		if err != nil {
			log.Printf("Error parsing the config file hosts list: %s\n"+
				"Keeping current host list", err)
		} else {
			hosts = nh
			log.Printf("New hosts list: %+v\n", hosts)
		}
	})

	hostPrivateKey, err := ioutil.ReadFile(viper.GetString("hostkey"))
	if err != nil {
		panic(err)
	}

	hostSigner, err := ssh.ParsePrivateKey(hostPrivateKey)
	if err != nil {
		panic(err)
	}

	users, err := parseAuthFile(viper.GetString("authkeys"))
	if err != nil {
		panic(err)
	}

	hasDefaults := false
	for _, h := range hosts {
		if h.NoAuth {
			hasDefaults = true
			break
		}
	}

	// sshmux setup
	auth := func(c ssh.ConnMetadata, key ssh.PublicKey) (*sshmux.User, error) {
		t := key.Type()
		k := key.Marshal()
		for i := range users {
			candidate := users[i].PublicKey
			if t == candidate.Type() && bytes.Compare(k, candidate.Marshal()) == 0 {
				return users[i], nil
			}
		}

		if hasDefaults {
			return nil, nil
		}

		log.Printf("%s: access denied (username: %s)", c.RemoteAddr(), c.User())
		return nil, errors.New("access denied")
	}

	setup := func(session *sshmux.Session) error {
		var username string
		if session.User != nil {
			username = session.User.Name
		} else {
			username = "unknown user"
		}
		log.Printf("%s: %s authorized (username: %s)", session.Conn.RemoteAddr(), username, session.Conn.User())

	outer:
		for _, h := range hosts {
			if h.NoAuth {
				session.Remotes = append(session.Remotes, h.Address)
				continue outer
			}

			if session.User == nil {
				continue
			}

			for _, u := range h.Users {
				if u == session.User.Name {
					session.Remotes = append(session.Remotes, h.Address)
					continue outer
				}
			}
		}
		return nil
	}

	server := sshmux.New(hostSigner, auth, setup)
	server.Selected = func(session *sshmux.Session, remote string) error {
		var username string
		if session.User != nil {
			username = session.User.Name
		} else {
			username = "unknown user"
		}
		log.Printf("%s: %s connecting to %s", session.Conn.RemoteAddr(), username, remote)
		return nil
	}

	// Set up listener
	l, err := net.Listen("tcp", viper.GetString("address"))
	if err != nil {
		panic(err)
	}

	server.Serve(l)
}
