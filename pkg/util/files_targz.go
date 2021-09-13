package util

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"time"

	"github.com/timoth-y/fabnctl/pkg/term"
)

// WriteBytesToTarGzip puts bytes from `reader` into the `targetPath` file in tar.gz archive,
// by performing pipeline write to WriteBytesToTar.
func WriteBytesToTarGzip(targetPath string, reader SizedReader, writer io.Writer) error {
	gzipWriter := gzip.NewWriter(writer)
	defer func() {
		if err := gzipWriter.Close(); err != nil {
			term.NewLogger().Errorf(err, "failed to close gzip writer containing '%s' file", targetPath)
		}
	}()

	return WriteBytesToTar(targetPath, reader, gzipWriter)
}

// WriteBytesToTar puts bytes from `reader` into the `targetPath` file in tar archive.
func WriteBytesToTar(targetPath string, reader SizedReader, writer io.Writer) error {
	tarWriter, ok := writer.(*tar.Writer)
	if !ok {
		tarWriter = tar.NewWriter(writer)
		defer func() {
			if err := tarWriter.Close(); err != nil {
				term.NewLogger().Errorf(err, "failed to close tar writer containing '%s' file", targetPath)
			}
		}()
	}

	if err := tarWriter.WriteHeader(&tar.Header{
		Name:    targetPath,
		Size:    int64(reader.Len()),
		Mode:    int64(0755),
		ModTime: time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to write header for file '%s': %w", targetPath, err)
	}

	_, err := io.Copy(tarWriter, reader); if err != nil {
		return fmt.Errorf("failed to copy the file '%s' data to the tar: %w", targetPath, err)
	}

	return nil
}
