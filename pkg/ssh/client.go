package ssh

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)


var (
	client    *ssh.Client
	closerFns []context.CancelFunc
)

// Init performs initialization of the SSH package.
func Init(options ...Option) error {
	var (
		args = &argsStub{
			host: "127.0.0.1", port: 22,
			ClientConfig: ssh.ClientConfig{
				User: os.Getenv("USER"),
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
		}
	)

	defaultMethod, err := sshAgentAuthMethod()
	if err != nil {
		return fmt.Errorf("failed to init default SSH auth method: %w", err)
	}

	args.Auth = append(args.Auth, defaultMethod)

	for i := range options {
		options[i](args)
	}

	if client, err = ssh.Dial("tcp",
		fmt.Sprintf("%s:%d", args.host, args.port),
		&args.ClientConfig,
	); err != nil {
		return err
	}

	return nil
}

// Close closes SSH auth agents and other allocated resources.
func Close() {
	for i := range closerFns {
		closerFns[i]()
	}
}
