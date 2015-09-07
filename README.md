# sshmuxd

A SSH "jump host" style proxy, based off the https://github.com/joushou/sshmux library.

So, why not just a jump host? Well, if it's just you and no one else needing access, go ahead. If you, however, want to give more than one person SSH access through your public IP on port N (N often being 22), then you might want something with a bit more access control. Sure, you can make really complicated SSH configs that limit a lot of things for the other users, but they'll always be able to poke around more than you want them to, and it'll be a pain in the butt to maintain.

Thinking it could be done simpler, sshmux and sshmuxd got written. It allow you to have a proxy that will *only* permit forwarding to user-specific servers, regardless of method. No other poking around is possible, and no having to allow actual login for anyone to the server running sshmuxd.

# Installation
sshmuxd can be installed from source (super simple with Go).
To download source:

      go get github.com/joushou/sshmuxd
As long as $GOPATH/bin is in your PATH, you can run it with:
 		 
	sshmuxd conf.json

Otherwise:

      cd $GOPATH/src/github.com/joushou/sshmuxd
      go build
      ./sshmuxd example_conf.json

# What does it do?

It acts like a regular SSH server, waiting for either session channel requests (regular ssh) or direct tcp connection requests (ssh -W).

If it gets a regular session channel request, it will figure out what servers the user is allowed to connect to. If the user is only permitted access to one server, it writes out which server it is connecting to, and connects immediately. If the user is permitted access to multiple servers, it will present the user with an interactive prompt, asking which server the user wishes to connect to.

If it gets a direct tcp connection request, it will simply check if this connection is permitted for the user, and if yes, execute the connection.

## Just show me how it looks!

Using the "regular ssh"-mode with interactive selection (That is, more than one permitted remote host for that user):

      $ ssh sshmux.example.com
      Welcome to sshmux, joushou
          [0] server1.example.com:22
          [1] server2.example.com:22
          [2] secret.example.com:65432
      Please select remote server:

If you then enter a number, it'll look like this:

      Please select remote server: 1
      Connecting to server2.example.com:22
      $ hostname
      serve2.example.com

If there were only one permitted host, sshmuxd will skip right to showing "Connecting to...". In direct tcp mode (ssh -W), you don't see any difference at all.

## But what's ssh -W?

ssh -W asks the SSH server to make a raw TCP connection, and forward stdin/stdout of the local client over the ssh connection to the raw TCP connection. How do you use that to jump hosts? With ProxyCommand! Put the following in your ~/.ssh/config (see the ssh_config manpage):

      Host server1.example.com
          ProxyCommand ssh -W %h:%p sshmux.example.com

Followed by running ssh from your command-line:

      ssh server1.example.com

You can also do this directly on the command-line, without ssh_config, with:

      ssh -oProxyCommand="ssh -W %h:%p sshmux.example.com" server1.example.com

This technique works is the general approach to jump hosts, and not related to sshmux. sshmux simply implements it with fine-grained controls. For more info, see the the wiki page (https://github.com/joushou/sshmuxd/wiki/ProxyCommand) or the ssh manpage.

# Limitations
sshmux, and by extension, sshmuxd, can only forward normal sessions (ssh'ing directly to sshmuxd without a ProxyCommand) if agent forwarding is enabled. This is because your normal session authenticates to sshmux, but sshmux then has to authenticate you with the remote host, requiring a additional access to your agent. sshmux will, however, not forward your agent to the final remote host. Doing this is simple if wanted, but I have yet to decide on how this is toggled.

Please note that the sftp and scp clients bundled with openssh cannot use normal session forwarding thus you must use a ProxyCommand for them. If you want them to work normal session forwarding, try to revive this *very* old bug report about it: https://bugzilla.mindrot.org/show_bug.cgi?id=831.

Using a "ssh -W" ProxyCommand circumvents this limitation, both for ssh and sftp/scp, and also bypasses the interactive server selection, as the client will inform sshmux of the wanted target directly. If the target is permitted, the user will be connected. This also provides more protection for the paranoid, as the connection to the final host is encrypted end-to-end, rather than being plaintext in the memory of sshmux.

# Configuration
sshmuxd requres 3 things:
* An authorized_keys-style file ("authkeys"), with the public key of all permitted users. Do note that the comment after the public key will be used as name of the user internally (this does not affect usernames over SSH, though).
* A private key for the server to use ("hostkey").
* A JSON configuration file. The format of the file is as follows (Do note that, due to the presence of comments, this is not actually a valid JSON file. Remove comments before use, or refer to example_conf.json)

      {
         // Listening address as given directly to net.Listen.
         "address": ":22",

         // Private key to use for built-in SSH server.
         "hostkey": "hostkey",

         // Authorized keys to use for authenticating users. An important note
         // is that the comment (the part after the key itself in an entry)
         // will  be used as name for the user internally.
         "authkeys": "authkeys",

         // The list of remote hosts that can be used through this proxy.
         "hosts": [
            {
               // The address of the remote host. This address must include the
               // port.
               "address": "ssh1.example.com:22",

               // The list of users permitted to access this host.
               "users": [ "boss", "me", "granny" ]

               // Whether or not this server can be accessed by anyone,
               // regardless of public key and presence in user list.
               // Defaults to false.
               "noAuth": false
            },
            {
               "address": "public.example.com:22",
               "noAuth": true
            }
            {
               "address": "secret.example.com:22",
               "users": [ "me" ]
            },
         ]
      }

# More info
For more details about this project, see the underlying library: http://github.com/joushou/sshmux
