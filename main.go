package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/joushou/sshmux"

	"golang.org/x/crypto/ssh"
)

func usage() {
	fmt.Printf("Usage: \n")
	fmt.Printf("   %s address hostkey authkeys servermap\n", os.Args[0])
	fmt.Printf("\n")
	fmt.Printf("   address    The address for the server to bind to\n")
	fmt.Printf("   hostkey    The private key for the server\n")
	fmt.Printf("   authkeys   The authorized_keys file to allow access for\n")
	fmt.Printf("   servermap  The list of servers and users that may access them\n")
}

func main() {
	if len(os.Args) != 5 {
		usage()
		return
	}

	address, hostKey, authKeys, serverMap := os.Args[1], os.Args[2], os.Args[3], os.Args[4]

	hostPrivateKey, err := ioutil.ReadFile(hostKey)
	if err != nil {
		panic(err)
	}

	hostSigner, err := ssh.ParsePrivateKey(hostPrivateKey)
	if err != nil {
		panic(err)
	}

	remotes, err := parseRemotes(serverMap)
	if err != nil {
		panic(err)
	}

	users, err := parseAuthFile(remotes, authKeys)
	if err != nil {
		panic(err)
	}

	var defaultRemotes []*sshmux.Remote
	for _, s := range remotes {
		for _, o := range s.Options {
			if o == "noauth" {
				defaultRemotes = append(defaultRemotes, s)
				break
			}
		}
	}

	auth := func(_ ssh.ConnMetadata, key ssh.PublicKey) (*sshmux.User, error) {
		t := key.Type()
		k := key.Marshal()
		for i := range users {
			candidate := users[i].PublicKey
			if t == candidate.Type() && bytes.Compare(k, candidate.Marshal()) == 0 {
				return users[i], nil
			}
		}

		if len(defaultRemotes) != 0 {
			return nil, nil
		}

		return nil, errors.New("access denied")
	}

	setup := func(session *sshmux.Session) error {
		session.Remotes = append(session.Remotes, session.User.Remotes...)
		session.Remotes = append(session.Remotes, defaultRemotes...)
		return nil
	}

	server := sshmux.New(hostSigner, auth, setup)
	// Set up listener

	l, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}

	server.Serve(l)
}
