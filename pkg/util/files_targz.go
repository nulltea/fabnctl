package util

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/timoth-y/fabnctl/pkg/terminal"
)

// WriteBytesToTarGzip puts bytes from `reader` into the `targetPath` file in tar.gz archive,
// by performing pipeline write to WriteBytesToTar.
func WriteBytesToTarGzip(targetPath string, reader SizedReader, writer io.Writer) error {
	gzipWriter := gzip.NewWriter(writer)
	defer func() {
		if err := gzipWriter.Close(); err != nil {
			terminal.Logger.Error(
				errors.Wrapf(err, "failed to close gzip writer containing '%s' file", targetPath),
			)
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
				terminal.Logger.Error(
					errors.Wrapf(err, "failed to close tar writer containing '%s' file", targetPath),
				)
			}
		}()
	}

	if err := tarWriter.WriteHeader(&tar.Header{
		Name:    targetPath,
		Size:    int64(reader.Len()),
		Mode:    int64(0755),
		ModTime: time.Now(),
	}); err != nil {
		return errors.Wrapf(err, "failed to write header for file '%s'", targetPath)
	}

	_, err := io.Copy(tarWriter, reader); if err != nil {
		return errors.Wrapf(err, "failed to copy the file '%s' data to the tar", targetPath)
	}

	return nil
}
