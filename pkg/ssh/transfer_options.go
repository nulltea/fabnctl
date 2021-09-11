package ssh

import (
	"context"
	"io"
)

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
