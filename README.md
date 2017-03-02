# sshmuxd [![Go Report Card](https://goreportcard.com/badge/joushou/sshmuxd)](https://goreportcard.com/report/joushou/sshmuxd)

A SSH "jump host" style proxy, based off the https://github.com/joushou/sshmux library.

So, why not just a jump host? Well, if it's just you and no one else needing access, go ahead. If you, however, want to give more than one person SSH access through your public IP on port N (N often being 22), then you might want something with a bit more access control. Sure, you can make really complicated SSH configs that limit a lot of things for the other users, but they'll always be able to poke around more than you want them to, and it'll be a pain in the butt to maintain.

Thinking it could be done simpler, sshmux and sshmuxd got written. It allows you to have a proxy that will *only* permit forwarding to user-specific servers, regardless of method. No other poking around is possible, and no having to allow actual login for anyone to the server running sshmuxd.

# Installation
sshmuxd can be installed from source (super simple with Go).
To download source:

	go get github.com/joushou/sshmuxd

As long as $GOPATH/bin is in your PATH, you can run it with:

	sshmuxd

Otherwise:

	cd $GOPATH/src/github.com/joushou/sshmuxd
	go build
	./sshmuxd

# What does it do?

It acts like a regular SSH server, waiting for either session channel requests (regular ssh) or direct tcp connection requests (ssh -W).

If it gets a regular session channel request, it will figure out what servers the user is allowed to connect to. If the user is only permitted access to one server, it writes out which server it is connecting to, and connects immediately. If the user is permitted access to multiple servers, it will present the user with an interactive prompt, asking which server the user wishes to connect to.

If it gets a direct tcp connection request, it will simply check if this connection is permitted for the user, and if yes, execute the connection.

## Just show me what it looks like!

Using the "regular ssh"-mode with interactive selection (that is, more than one permitted remote host for that user):

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
	server2.example.com

If there were only one permitted host, sshmuxd will skip right to showing "Connecting to...". In direct tcp mode (ssh -W), you don't see any difference at all.

## But agent forwarding is dangerous!

In the general sense, yes. If you don't just ignore SSH's warnings about changed host keys, then you can be sure that you're connecting to the real host. That means that, to abuse agents in the common sense, the root user must be compromised on that machine. The root user will then be able to sign things with our private key by accessing the agent socket that got forwarded. The result is the same as if the root user of your local machine got compromised, with a private key that is password protected very well, but the agent had the key unlocked, signing requests as it is told.

With sshmux, agent forwarding isn't handled as a socket, but in-memory. Depending on OS, this doesn't necessarily protect you against an evil root, due to peculiar interfaces such as /dev/mem, but it sure does increase the barrier of entry: Rather than just reading an easily found socket, you need to inspect arbitrary process memory, finding the right component and interfacing with it accordingly.

A good note, however, is that if you are concerned about the hosts you log into having arbitrary process memory or root login compromised, you shouldn't really log in to them at all, but take them offline immediately and use a serial console (virtual or physical) to salvage what is necessary, followed by a total wipe of the machine. Using them, or even just leaving them online in this state, will not bring you any good.

With this, I am not saying that agent forwarding isn't dangerous. Don't use agent-forwarding to arbitrary machines you don't trust. If you have these concerns, you can use ssh -W, which provides guarantees of security against any issues with the jump host (but not the remote host). ssh -W as ProxyCommand in ssh_config is also much more convenient for hosts you log into often, rather than having to make interactive selections.

Personally, I wish that ssh agents would ask the user before signing. This way, agent forwarding could be used a bit more relaxed, as users would be prompted with what and who is trying to sign before accepting. If signing is attempted without you using it, you could simply deny it. Blindly signing things is a bad idea.

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
sshmuxd requires 3 things:
* An authorized_keys-style file ("authkeys"), with the public key of all permitted users. Do note that the comment after the public key will be used as name of the user internally (this does not affect usernames over SSH, though).
* A private key for the server to use ("hostkey").
* A JSON configuration file named sshdmuxd.json in one of the following places:
  - Working dir
	- $HOME/.sshmuxd/
	- /etc/sshmuxd/
	- or pass the path on command line using --config filename.json

The format of the file is as follows (note that, due to the presence of comments, this is not actually a valid JSON file. Remove comments before use, or refer to sshmuxd.json)

```
{
	// Listening address as given directly to net.Listen.
	"address": ":22",

	// Private key to use for built-in SSH server.
	"hostkey": "hostkey",

	// Authorized keys to use for authenticating users. An important note
	// is that the comment (the part after the key itself in an entry)
	// will	be used as name for the user internally.
	"authkeys": "authkeys",

	// The list of remote hosts that can be used through this proxy.
	"hosts": [
		{
			// The address of the remote host. This address must include the
			// port.
			"address": "ssh1.example.com:22",

			// The list of users permitted to access this host.
			"users": [ "boss", "me", "granny" ],

			// Whether or not this server can be accessed by anyone,
			// regardless of public key and presence in user list.
			// Defaults to false.
			"noAuth": false
		},
		{
			"address": "public.example.com:22",
			"noAuth": true
		},
		{
			"address": "secret.example.com:22",
			"users": [ "me" ]
		}
	]
}
```

# More info
For more details about this project, see the underlying library: http://github.com/joushou/sshmux
