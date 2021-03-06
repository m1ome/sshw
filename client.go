package sshw

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"time"
)

type Client interface {
	Login()
}

type defaultClient struct {
	clientConfig *ssh.ClientConfig
	node         *Node
}

func NewClient(node *Node) Client {
	u, err := user.Current()
	if err != nil {
		l.Error(err)
		return nil
	}

	var authMethods []ssh.AuthMethod

	var pemBytes []byte
	if node.KeyPath == "" {
		pemBytes, err = ioutil.ReadFile(path.Join(u.HomeDir, ".ssh/id_rsa"))
	} else {
		pemBytes, err = ioutil.ReadFile(node.KeyPath)
	}

	if err == nil {
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err == nil {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	password := node.password()

	if password != nil {
		authMethods = append(authMethods, password)
	}

	config := &ssh.ClientConfig{
		User:            node.user(),
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second * 10,
	}

	config.SetDefaults()
	config.Ciphers = append(config.Ciphers, "aes128-cbc", "3des-cbc", "blowfish-cbc", "cast128-cbc", "aes192-cbc", "aes256-cbc")

	return &defaultClient{
		clientConfig: config,
		node:         node,
	}
}

func (c *defaultClient) Login() {
	host := c.node.Host
	port := c.node.port()
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), c.clientConfig)
	if err != nil {
		l.Error(err)
		return
	}
	defer client.Close()

	l.Infof("connect server ssh -p %d %s@%s version: %s\n", port, c.node.user(), host, string(client.ServerVersion()))

	session, err := client.NewSession()
	if err != nil {
		l.Error(err)
		return
	}
	defer session.Close()

	fd := int(os.Stdin.Fd())
	state, err := terminal.MakeRaw(fd)
	if err != nil {
		l.Error(err)
		return
	}
	defer terminal.Restore(fd, state)

	w, h, err := terminal.GetSize(fd)
	if err != nil {
		l.Error(err)
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	err = session.RequestPty("xterm", h, w, modes)
	if err != nil {
		l.Error(err)
		return
	}

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin

	err = session.Shell()
	if err != nil {
		l.Error(err)
		return
	}

	session.Wait()
}
