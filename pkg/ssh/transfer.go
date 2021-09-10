package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kr/fs"
	"github.com/morikuni/aec"
	"github.com/pkg/sftp"
)

// Transfer sends files from given local `path` to remote `target` path over SFTP protocol.
//
// Options allow streaming command output to standard OS output streams or custom ones.
func Transfer(path string, remoteDir string, options ...TransferOption) error {
	var (
		args = &transferArgsStub{
			ctx: context.Background(),
			stdout: os.Stdout,
			stderr: os.Stderr,
			concurrency: 1,
			skip: []string{".git/**/*"},
		}
		wg sync.WaitGroup
	)

	for i := range options {
		options[i](args)
	}

	var ch = make(chan string, args.concurrency)

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed create SFTP client: %w", err)
	}

	defer func() {
		_ = sftpClient.Close()
	}()

	if err = sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to make directory on path %s: %w", remoteDir, err)
	}

	for i := 0; i < args.concurrency; i++ {
		go func() {
			for {
				select {
				case srcPath := <- ch:
					func (srcPath string) {
						defer wg.Done()

						pathRel, _ := filepath.Rel(path, srcPath)
						src, err := os.Open(srcPath)
						if err != nil {
							printTransferError(args.stderr, err, "Error opening file %s", pathRel)
							return
						}

						defer func() {
							_ = src.Close()
						}()

						remotePath := filepath.Join(remoteDir, pathRel)
						trg, err := sftpClient.Create(remotePath)
						if err != nil {
							printTransferError(args.stderr, err, "Error creating remoteDir file %s", remotePath)
							return
						}

						defer func() {
							_ = trg.Close()
						}()

						if _, err = trg.ReadFrom(src); err != nil {
							printTransferError(args.stderr, err, "Error transferring %s file to %s", pathRel, remotePath)
							return
						}

						printTransferProgress(args.stdout, "File transferred to", remoteDir, pathRel)
					}(srcPath)
				case <- args.ctx.Done():
					return
				}
			}
		}()
	}

WALKER:
	for walker := fs.Walk(path); walker.Step(); {
		var pathRel, _ = filepath.Rel(path, walker.Path())

		if err = walker.Err(); err != nil {
			_, _ = fmt.Fprintf(args.stderr,
				"Error walking though path %s: %v\n", walker.Path(), err,
			)

			continue
		}

		for i := range args.skip {
			if matched, _ := doublestar.Match(args.skip[i], pathRel); matched {
				continue WALKER
			}
		}

		if walker.Stat().IsDir() {
			dirPath := filepath.Join(remoteDir, pathRel)
			if err = sftpClient.MkdirAll(dirPath); err != nil {
				printTransferError(args.stderr, err, "Failed to make directory on path %s", dirPath)
			}

			continue
		}

		wg.Add(1)
		ch <- walker.Path()
	}

	wg.Wait()

	printTransferFinished(args.stdout, path, remoteDir)

	return nil
}

func printTransferProgress(out io.Writer, message, basePath, relPath string) {
	if len(basePath) + len(relPath) > 100 {
		var (
			paths = strings.Split(relPath, "/")
		)

		if len(paths) > 2 {
			relPath = filepath.Join(paths[0], "**", paths[len(paths) - 1])
		} else {
			relPath = fmt.Sprintf("%s...", relPath[0:5])
		}
	}

	_, _ = fmt.Fprint(out, aec.EraseLine(aec.EraseModes.Tail),
		fmt.Sprintf("%s %s", message, filepath.Join(basePath, relPath)),
		aec.Up(1), "\n",
	)
}

func printTransferError(out io.Writer, err error, format string, a ...interface{}) {
	_, _ = fmt.Fprint(out, aec.LightRedF,
		fmt.Sprintf("%s: %v", fmt.Sprintf(format, a...), err), aec.DefaultF, "\n",
	)
}

func printTransferFinished(out io.Writer, from, to string) {
	_, _ = fmt.Fprint(out, aec.EraseLine(aec.EraseModes.Tail),
		fmt.Sprintf("Trasfer of %s to %s complete \n", from, to),
	)
}
