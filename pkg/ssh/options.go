package ssh

import "golang.org/x/crypto/ssh"

type (
	// Option configures SSH operator package
	Option func(stub *argsStub)

	argsStub struct {
		*ssh.ClientConfig
		address string
		user    string
	}
)

func WithAddress() Option {

}


