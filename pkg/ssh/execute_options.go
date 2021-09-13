package ssh

import "io"

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

