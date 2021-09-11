package term

import "io"

type (
	LoggerOption func(args *loggerArgs) 
	
	loggerArgs struct {
		stdout io.Writer
		stderr io.Writer
		stream bool
	}
)

// WithStream can be used to redirect command output to local stdout and stderr.
// Default is false.
func WithStream(stream bool) LoggerOption {
	return func(stub *loggerArgs) {
		stub.stream = stream
	}
}

// WithStdout can be specified where to stream cmd output.
// Default is local os.Stdout.
func WithStdout(stdout io.Writer) LoggerOption {
	return func(stub *loggerArgs) {
		stub.stdout = stdout
	}
}

// WithStderr can be specified where to stream cmd error output.
// Default is local os.Stderr.
func WithStderr(stderr io.Writer) LoggerOption {
	return func(stub *loggerArgs) {
		stub.stderr = stderr
	}
}
