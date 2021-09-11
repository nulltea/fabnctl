package ssh

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
	"k8s.io/kubectl/pkg/cmd/util"
)

type RemoteOperator struct {
	*ssh.Client
	*clientArgs
}

// New creates new RemoteOperator instance.
func New(options ...Option) (*RemoteOperator, error) {
	var op = &RemoteOperator{
		clientArgs: &clientArgs{
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
		options[i](op.clientArgs)
	}

	if len(op.initErrors) > 0 {
		return nil, fmt.Errorf(util.MultipleErrors("invalid args", op.clientArgs.initErrors))
	}

	if op.Client, err = ssh.Dial("tcp",
		fmt.Sprintf("%s:%d", op.clientArgs.host, op.clientArgs.port),
		&op.clientArgs.ClientConfig,
	); err != nil {
		return nil, err
	}

	return op, nil
}

// Close closes SSH connection and other allocated resources.
func (o *RemoteOperator) Close() error {
	for i := range o.closers {
		o.closers[i]()
	}

	return o.Client.Close()
}
