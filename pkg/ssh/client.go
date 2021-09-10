package ssh

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

type RemoteOperator struct {
	*ssh.Client
	*argsStub

}

// New creates new RemoteOperator instance.
func New(options ...Option) (*RemoteOperator, error) {
	var op = &RemoteOperator{
		argsStub: &argsStub{
			host: "127.0.0.1", port: 22,
			ClientConfig: ssh.ClientConfig{
				User:            os.Getenv("USER"),
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
		},
	}


	defaultMethod, err := sshAgentAuthMethod()
	if err != nil {
		return nil, fmt.Errorf("failed to init default SSH auth method: %w", err)
	}

	op.Auth = append(op.Auth, defaultMethod)

	for i := range options {
		options[i](op.argsStub)
	}

	if op.Client, err = ssh.Dial("tcp",
		fmt.Sprintf("%s:%d", op.argsStub.host, op.argsStub.port),
		&op.argsStub.ClientConfig,
	); err != nil {
		return nil, err
	}

	return op, nil
}

// Close closes SSH connection and other allocated resources.
func (o *RemoteOperator) Close() er {
	for i := range o.closers {
		o.closers[i]()
	}

	return  o.Client.Close()
}
