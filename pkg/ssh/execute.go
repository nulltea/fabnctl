package ssh

import (
	"bytes"
	"io"
	"os"
	"sync"

	"github.com/timoth-y/fabnctl/pkg/term"
)

// Execute performs remote execution of the given `command`.
//
// Options allow streaming command output to standard OS output streams or custom ones.
func (o *RemoteOperator) Execute(command string, options ...ExecuteOption) ([]byte, []byte, error) {
	var args = &execArgsStub{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}

	for i := range options {
		options[i](args)
	}

	session, err := o.NewSession()
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if err = session.Close(); err != nil {
			term.Logger.Errorf("failed to close SSH session")
		}
	}()

	sessionStdout, err := session.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	var (
		outputBuffer = bytes.Buffer{}
		errorBuffer  = bytes.Buffer{}
		stdoutWriter io.Writer
		stderrWriter io.Writer
		wg           = sync.WaitGroup{}
	)

	if args.stream {
		stdoutWriter = io.MultiWriter(args.stdout, &outputBuffer)
	} else {
		stdoutWriter = &outputBuffer
	}

	wg.Add(1)
	go func() {
		_, _ = io.Copy(stdoutWriter, sessionStdout)
		wg.Done()
	}()

	sessionStderr, err := session.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	if args.stream {
		stderrWriter = io.MultiWriter(args.stderr, &errorBuffer)
	} else {
		stderrWriter = &errorBuffer
	}

	wg.Add(1)

	go func() {
		_, _ = io.Copy(stderrWriter, sessionStderr)
		wg.Done()
	}()

	if err = session.Run(command); err != nil {
		return nil, nil, err
	}

	wg.Wait()

	return outputBuffer.Bytes(), errorBuffer.Bytes(), nil
}

