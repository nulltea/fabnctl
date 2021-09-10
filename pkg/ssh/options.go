package ssh

import (
	"context"
	"io"

	"github.com/timoth-y/fabnctl/pkg/term"
	"golang.org/x/crypto/ssh"
)

type (
	// Option configures SSH operator package.
	Option func(*argsStub)

	argsStub struct {
		ssh.ClientConfig
		host    string
		port    int
		closers []context.CancelFunc
	}
)

// WithHost can be used to specify SSH host of device on which commands would be executed.
//
// Default is: 127.0.0.1.
func WithHost(addr string) Option {
	return func(stub *argsStub) {
		stub.host = addr
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
			term.Logger.Fatal(err)
		}

		stub.closers = append(stub.closers, closeFunc)
		stub.Auth = append(stub.Auth, am)
	}
}

type (
	// ExecuteOption allows passing options for command execution over SSH.
	ExecuteOption func(*execArgsStub)

	execArgsStub struct {
		stdout io.Writer
		stderr io.Writer
		stream bool
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

type (
	// TransferOption allows passing options for file transfer over SSH.
	TransferOption func(*transferArgsStub)

	transferArgsStub struct {
		ctx context.Context
		stdout io.Writer
		stderr      io.Writer
		skip        []string
		concurrency int
	}
)

// WithStdoutLog can be specified where to stream cmd output.
// Default is local os.Stdout.
func WithStdoutLog(stdout io.Writer) TransferOption {
	return func(stub *transferArgsStub) {
		stub.stdout = stdout
	}
}

// WithStderrLog can be specified where to stream cmd error output.
// Default is local os.Stderr.
func WithStderrLog(stderr io.Writer) TransferOption {
	return func(stub *transferArgsStub) {
		stub.stderr = stderr
	}
}

// WithConcurrency can be used to allow concurrency while transferring multiply files from directory.
// Synchronous by default.
func WithConcurrency(concurrency int) TransferOption {
	return func(stub *transferArgsStub) {
		stub.concurrency = concurrency
	}
}

// WithContext can be used to pass context for the remote command execution.
// Default is background context.
func WithContext(ctx context.Context) TransferOption {
	return func(stub *transferArgsStub) {
		stub.ctx = ctx
	}
}

// WithSkip can be used to pass file patterns which will be skipped during transfer.
// Default is [.git/*].
func WithSkip(skip ...string) TransferOption {
	return func(stub *transferArgsStub) {
		stub.skip = append(stub.skip, skip...)
	}
}
