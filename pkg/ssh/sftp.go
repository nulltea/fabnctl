package ssh

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kr/fs"
	"github.com/pkg/sftp"
)

// Transfer sends files from given local `path` to remote `target` path over SFTP protocol.
//
// Options allow streaming command output to standard OS output streams or custom ones.
func Transfer(path string, remote string, options ...ExecuteOption) error {
	var (
		args = &execArgsStub{
			ctx: context.Background(),
			stdout: os.Stdout,
			stderr: os.Stderr,
			concurrency: 1,
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

	for i := 0; i < args.concurrency; i++ {
		go func() {
			for {
				select {
				case srcPath := <- ch:
					func (srcPath string) {
						pathRel, _ := filepath.Rel(path, srcPath)
						src, err := os.Open(srcPath)
						if err != nil {
							if args.stream {
								_, _ = fmt.Fprintf(args.stderr, "error opening file %s: %v\n", pathRel, err)
							}
							return
						}

						defer func() {
							_ = src.Close()
						}()

						remotePath := filepath.Join(remote, pathRel)
						trg, err := sftpClient.Create(remotePath)
						if err != nil {
							if args.stream {
								_, _ = fmt.Fprintf(args.stderr,
									"error opening remote file %s: %v\n", remotePath, err,
								)
							}

							return
						}

						defer func() {
							_ = trg.Close()
						}()

						if _, err = trg.ReadFrom(src); err != nil {
							if args.stream {
								_, _ = fmt.Fprintf(args.stderr,
									"error transfering %s file to %s: %v\n", pathRel, remotePath, err,
								)
							}
						}
					}(srcPath)
				case <- args.ctx.Done():
					return
				}
			}
		}()
	}

	for walker := fs.Walk(path); walker.Step(); {
		if err = walker.Err(); err != nil {
			if args.stream {
				_, _ = fmt.Fprintf(args.stderr,
					"error walking though path %s: %v\n", walker.Path(), err,
				)
			}

			continue
		}

		ch <- walker.Path()
	}

	return nil
}
