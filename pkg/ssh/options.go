package ssh

import (
	"context"
	"io"

	"github.com/timoth-y/fabnctl/pkg/terminal"
	"golang.org/x/crypto/ssh"
)

type (
	// Option configures SSH operator package.
	Option func(*argsStub)

	argsStub struct {
		ssh.ClientConfig
		address string
		port    int
	}
)

// WithAddress can be used to specify SSH address of device on which commands would be executed.
//
// Default is: 127.0.0.1.
func WithAddress(addr string) Option {
	return func(stub *argsStub) {
		stub.address = addr
	}
}

// WithPort can be used to specify SSH port.
//
// Default is: 22.
func WithPort(port int) Option {
	return func(stub *argsStub) {
		stub.port = port
	}
}

// WithUser can be used to specify user under which commands would be executed on target device's system.
//
// Default is: $USER local environmental variable.
func WithUser(user string) Option {
	return func(stub *argsStub) {
		stub.User = user
	}
}

// WithPassword can be used to use password as SSH auth method.
//
// Disabled by default.
func WithPassword(password string) Option {
	return func(stub *argsStub) {
		stub.Auth = append(stub.Auth, ssh.Password(password))
	}
}

// WithPublicKeyPath can be used to specify package to use password as SSH auth method.
//
// Default is: $HOME/.ssh/id_rsa.pub
func WithPublicKeyPath(path string) Option {
	return func(stub *argsStub) {
		am, closeFunc, err := publicKeyAuthMethod(path)
		if err != nil {
			terminal.Logger.Fatal(err)
		}

		closerFns = append(closerFns, closeFunc)
		stub.Auth = append(stub.Auth, am)
	}
}

type (
	// ExecuteOption allows passing options for command execution over SSH.
	ExecuteOption func(*execArgsStub)

	execArgsStub struct {
		ctx context.Context
		stdout io.Writer
		stderr io.Writer
		stream bool
		concurrency int
	}
)

// WithStream can be used to redirect command output to local stdout and stderr.
// Default is false.
func WithStream(stream bool) ExecuteOption {
	return func(stub *execArgsStub) {
		stub.stream = stream
	}
}

// WithStdout can be specified where to stream cmd output.
// Default is local os.Stdout.
func WithStdout(stdout io.Writer) ExecuteOption {
	return func(stub *execArgsStub) {
		stub.stdout = stdout
	}
}

// WithStderr can be specified where to stream cmd error output.
// Default is local os.Stderr.
func WithStderr(stderr io.Writer) ExecuteOption {
	return func(stub *execArgsStub) {
		stub.stderr = stderr
	}
}

// WithConcurrency can be used to allow concurrency while transferring multiply files from directory.
// Synchronous by default.
func WithConcurrency(concurrency int) ExecuteOption {
	return func(stub *execArgsStub) {
		stub.concurrency = concurrency
	}
}

// WithContext can be used to pass context for the remote command execution.
// Default is background context.
func WithContext(ctx context.Context) ExecuteOption {
	return func(stub *execArgsStub) {
		stub.ctx = ctx
	}
}
