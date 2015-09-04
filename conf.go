package main

import (
	"bufio"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/joushou/sshmux"

	"golang.org/x/crypto/ssh"
)

func parseRemotes(filename string) ([]*sshmux.Remote, error) {
	var servers []*sshmux.Remote

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(f)
	done := false
	for !done {

		str, err := reader.ReadString('\n')
		if err != nil {
			done = true
		}

		str = strings.TrimSpace(str)
		if len(str) == 0 || str[0] == '#' {
			continue
		}

		parts := strings.Split(str, " ")

		var server *sshmux.Remote
		switch len(parts) {
		case 2:
			server = &sshmux.Remote{
				Address: parts[0],
				Users:   strings.Split(parts[1], ","),
			}
		case 3:
			server = &sshmux.Remote{
				Address: parts[0],
				Users:   strings.Split(parts[1], ","),
				Options: strings.Split(parts[2], ","),
			}
			for i := range server.Options {
				server.Options[i] = strings.TrimSpace(server.Options[i])
			}
		default:
			return nil, errors.New("incomplete line")
		}

		for i := range server.Users {
			server.Users[i] = strings.TrimSpace(server.Users[i])
		}
		servers = append(servers, server)
	}

	return servers, nil
}

func parseAuthFile(servers []*sshmux.Remote, filename string) ([]*sshmux.User, error) {
	var users []*sshmux.User

	authFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Parse authfile as authorized_key

	for len(authFile) > 0 {
		var (
			pk      ssh.PublicKey
			options []string
			comment string
		)

		pk, comment, options, authFile, err = ssh.ParseAuthorizedKey(authFile)
		if err != nil {
			return nil, err
		}

		u := &sshmux.User{
			PublicKey: pk,
			Name:      comment,
			Options:   options,
			Remotes:   make([]*sshmux.Remote, 0),
		}

		for _, s := range servers {
			for _, u2 := range s.Users {
				if u2 == u.Name {
					u.Remotes = append(u.Remotes, s)
				}
			}
		}
		users = append(users, u)
	}

	return users, nil
}
