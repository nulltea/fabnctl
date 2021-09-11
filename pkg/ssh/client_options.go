package ssh

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
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

// WithHostFlag can be used to specify SSH host of device on which commands would be executed.
//
// Default is: 127.0.0.1.
func WithHostFlag(flags *pflag.FlagSet, name string) Option {
	return func(args *clientArgs) {
		var err error

		if args.host, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s': %v",
					term.ErrInvalidArgs, name, err),
			)
		}
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

// WithPortFlag ...
//
// Default is: 22.
func WithPortFlag(flags *pflag.FlagSet, name string) Option {
	return func(args *clientArgs) {
		var err error

		if args.port, err = flags.GetInt(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s': %v",
					term.ErrInvalidArgs, name, err),
			)
		}
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

// WithUserFlag ...
//
// Default is: 22.
func WithUserFlag(flags *pflag.FlagSet, name string) Option {
	return func(args *clientArgs) {
		var err error

		if args.User, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s': %v",
					term.ErrInvalidArgs, name, err),
			)
		}
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

// WithPasswordFlag ...
//
// Disabled by default.
func WithPasswordFlag(flags *pflag.FlagSet, name string) Option {
	return func(args *clientArgs) {
		var err error

		password, err := flags.GetString(name)
		if err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s': %v",
					term.ErrInvalidArgs, name, err),
			)
		}

		WithPassword(password)(args)
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

// WithPublicKeyPathFlag ...
//
// Default is: $HOME/.ssh/id_rsa
func WithPublicKeyPathFlag(flags *pflag.FlagSet, name string) Option {
	return func(args *clientArgs) {
		var err error

		path, err := flags.GetString(name)
		if err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s': %v",
					term.ErrInvalidArgs, name, err),
			)
		}

		WithPublicKeyPath(path)(args)
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
