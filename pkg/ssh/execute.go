package ssh

import (
	"bytes"
	"io"
	"os"
	"sync"

	"github.com/timoth-y/fabnctl/pkg/terminal"
)

func Execute(command string, options ...ExecuteOption) (io.Reader, io.Reader, error) {
	var args = &execArgsStub{
		stdout: os.Stdout,
		stderr: os.Stderr,
	}

	for i := range options {
		options[i](args)
	}

	session, err := client.NewSession()
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if err = session.Close(); err != nil {
			terminal.Logger.Errorf("failed to close SSH session")
		}
	}()

	sessStdOut, err := session.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	var (
		outputBuffer = bytes.Buffer{}
		errorBuffer  = bytes.Buffer{}
		stdout       io.Writer
		wg           = sync.WaitGroup{}
	)

	if args.stream {
		stdout = io.MultiWriter(args.stdout, &outputBuffer)
	} else {
		stdout = &outputBuffer
	}

	wg.Add(1)
	go func() {
		_, _ = io.Copy(stdout, sessStdOut)
		wg.Done()
	}()

	sessStderr, err := session.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	var stdErrWriter io.Writer
	if args.stream {
		stdErrWriter = io.MultiWriter(args.stderr, &errorBuffer)
	} else {
		stdErrWriter = &errorBuffer
	}

	wg.Add(1)

	go func() {
		_, _ = io.Copy(stdErrWriter, sessStderr)
		wg.Done()
	}()

	if err = session.Run(command); err != nil {
		return nil, nil, err
	}

	wg.Wait()

	return &outputBuffer, &errorBuffer, nil
}
