package ssh

import (
	"context"

	"github.com/timoth-y/fabnctl/pkg/term"
	"golang.org/x/crypto/ssh"
)

type (
	// Option configures SSH operator package.
	Option func(*clientArgs)

	clientArgs struct {
		ssh.ClientConfig
		host       string
		port       int
		logger     *term.Logger
		closers    []context.CancelFunc
		initErrors []error
	}
)

// WithHost can be used to specify SSH host of device on which commands would be executed.
//
// Default is: 127.0.0.1.
func WithHost(addr string) Option {
	return func(args *clientArgs) {
		args.host = addr
	}
}

// WithPort can be used to specify SSH port.
//
// Default is: 22.
func WithPort(port int) Option {
	return func(args *clientArgs) {
		args.port = port
	}
}

// WithUser can be used to specify user under which commands would be executed on target device's system.
//
// Default is: $USER local environmental variable.
func WithUser(user string) Option {
	return func(args *clientArgs) {
		args.User = user
	}
}

// WithPassword can be used to use password as SSH auth method.
//
// Disabled by default.
func WithPassword(password string) Option {
	return func(args *clientArgs) {
		args.Auth = append(args.Auth, ssh.Password(password))
	}
}

// WithPublicKeyPath can be used to specify package to use password as SSH auth method.
//
// Default is: $HOME/.ssh/id_rsa
func WithPublicKeyPath(path string) Option {
	return func(args *clientArgs) {
		am, closeFunc, err := publicKeyAuthMethod(path)
		if err != nil {
			args.initErrors = append(args.initErrors, err)
			return
		}

		args.closers = append(args.closers, closeFunc)
		args.Auth = append(args.Auth, am)
	}
}

// WithLogger can be used to pass custom logger for displaying remote commands output.
func WithLogger(logger *term.Logger, options ...term.LoggerOption) Option {
	return func(args *clientArgs) {
		if logger != nil {
			args.logger = logger
			return
		}

		args.logger = term.NewLogger(options...)
	}
}
