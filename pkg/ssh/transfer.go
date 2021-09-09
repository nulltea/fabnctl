package ssh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kr/fs"
	"github.com/pkg/sftp"
)

// Transfer sends files from given local `path` to remote `target` path over SFTP protocol.
//
// Options allow streaming command output to standard OS output streams or custom ones.
func Transfer(path string, remote string, options ...TransferOption) error {
	var (
		args = &transferArgsStub{
			ctx: context.Background(),
			stdout: os.Stdout,
			stderr: os.Stderr,
			concurrency: 1,
			skip: []string{".git/**/*"},
		}
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

	if err = sftpClient.MkdirAll(remote); err != nil {
		return fmt.Errorf("failed to make directory on path %s: %w", remote, err)
	}

	for i := 0; i < args.concurrency; i++ {
		go func() {
			for {
				select {
				case srcPath := <- ch:
					func (srcPath string) {
						pathRel, _ := filepath.Rel(path, srcPath)
						src, err := os.Open(srcPath)
						if err != nil {
							_, _ = fmt.Fprintf(args.stderr, "error opening file %s: %v\n", pathRel, err)
							return
						}

						defer func() {
							_ = src.Close()
						}()

						remotePath := filepath.Join(remote, pathRel)
						trg, err := sftpClient.Create(remotePath)
						if err != nil {
							_, _ = fmt.Fprintf(args.stderr,
								"error creating remote file %s: %v\n", remotePath, err,
							)

							return
						}

						defer func() {
							_ = trg.Close()
						}()

						if _, err = trg.ReadFrom(src); err != nil {
							_, _ = fmt.Fprintf(args.stderr,
								"error transfering %s file to %s: %v\n", pathRel, remotePath, err,
							)
						}

						_, _ = fmt.Fprintf(args.stdout, "filed transfered to %s\n", remotePath)
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
				"error walking though path %s: %v\n", walker.Path(), err,
			)

			continue
		}

		for i := range args.skip {
			if matched, _ := doublestar.Match(args.skip[i], pathRel); matched {
				continue WALKER
			}
		}

		if walker.Stat().IsDir() {
			dirPath := filepath.Join(remote, pathRel)
			if err = sftpClient.MkdirAll(dirPath); err != nil {
				_, _ = fmt.Fprintf(args.stderr,
					"failed to make directory on path %s: %v\n", dirPath, err,
				)
			}

			continue
		}

		ch <- walker.Path()
	}

	return nil
}
