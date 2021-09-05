package util

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	_ "unsafe"

	"github.com/pkg/errors"
	core2 "github.com/timoth-y/chainmetric-network/shared/core"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func CopyToPod(
	ctx context.Context,
	podName, namespace string,
	buffer *bytes.Buffer, destPath string,
) error {
	var (
		pipeReader, pipeWriter = io.Pipe()
		cmd                    = []string{"tar", "-xf", "-"}
	)

	if destPath != "/" && strings.HasSuffix(string(destPath[len(destPath)-1]), "/") {
		destPath = destPath[:len(destPath)-1]
	}
	destDir := path.Dir(destPath)

	if len(destDir) > 0 {
		cmd = append(cmd, "-C", destDir)
	}

	pod, err := core2.K8s.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "faield to determine container for '%s' pod", podName)
	}

	var (
		stdout, stderr bytes.Buffer
		req = core2.K8s.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(namespace).
			SubResource("exec").
			Param("container", pod.Spec.Containers[0].Name).
			VersionedParams(&v1.PodExecOptions{
				Container: pod.Spec.Containers[0].Name,
				Command:   cmd,
				Stdin:     true,
				Stdout:    true,
				Stderr:    true,
			}, scheme.ParameterCodec)
	)

	go func() {
		defer pipeWriter.Close()
		if err = WriteBytesToTar(destPath, buffer, pipeWriter); err != nil {
			core2.Logger.Error(
				errors.Wrapf(err, "failed to write '%s' into pod writer", destPath),
			)
		}
	}()

	err = execute(ctx, "POST", req.URL(), core2.K8sConfig, pipeReader, &stdout, &stderr)
	if err != nil {
		if stdErr := ErrFromStderr(stderr); stdErr != nil {
			err = stdErr
		}

		return err
	}

	return nil
}

func copyFromPod(
	ctx context.Context,
	podName, namespace string,
	srcPath string, destPath string,
) error {
	var (
		reader, writer = io.Pipe()
		cmd            = []string{"tar", "cf", "-", srcPath}
	)

	pod, err := core2.K8s.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "faield to determine container for '%s' pod", podName)
	}

	var (
		stderr bytes.Buffer
		req = core2.K8s.CoreV1().RESTClient().Get().
			Resource("pods").
			Name(podName).
			Namespace(namespace).
			SubResource("exec").
			Param("container", pod.Spec.Containers[0].Name).
			VersionedParams(&v1.PodExecOptions{
				Container: pod.Spec.Containers[0].Name,
				Command:   cmd,
				Stdin:     false,
				Stdout:    true,
				Stderr:    true,
			}, scheme.ParameterCodec)
	)

	go func() {
		defer writer.Close()
		if err = execute(ctx, "POST", req.URL(), core2.K8sConfig, nil, writer, &stderr); err != nil {
			if stdErr := ErrFromStderr(stderr); stdErr != nil {
				err = stdErr
			}
		}
	}()

	prefix := getPrefix(srcPath)
	prefix = path.Clean(prefix)
	prefix = stripPathShortcuts(prefix)
	destPath = path.Join(destPath, path.Base(prefix))
	err = untarAll(reader, destPath, prefix)

	return err
}

func untarAll(reader io.Reader, destDir, prefix string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if !strings.HasPrefix(header.Name, prefix) {
			return fmt.Errorf("tar contents corrupted")
		}

		mode := header.FileInfo().Mode()
		destFileName := filepath.Join(destDir, header.Name[len(prefix):])

		baseName := filepath.Dir(destFileName)
		if err := os.MkdirAll(baseName, 0755); err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(destFileName, 0755); err != nil {
				return err
			}
			continue
		}

		evaledPath, err := filepath.EvalSymlinks(baseName)
		if err != nil {
			return err
		}

		if mode&os.ModeSymlink != 0 {
			linkname := header.Linkname

			if !filepath.IsAbs(linkname) {
				_ = filepath.Join(evaledPath, linkname)
			}

			if err := os.Symlink(linkname, destFileName); err != nil {
				return err
			}
		} else {
			outFile, err := os.Create(destFileName)
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

func getPrefix(file string) string {
	return strings.TrimLeft(file, "/")
}

// stripPathShortcuts removes any leading or trailing "../" from a given path
func stripPathShortcuts(p string) string {
	newPath := path.Clean(p)
	trimmed := strings.TrimPrefix(newPath, "../")

	for trimmed != newPath {
		newPath = trimmed
		trimmed = strings.TrimPrefix(newPath, "../")
	}

	// trim leftover {".", ".."}
	if newPath == "." || newPath == ".." {
		newPath = ""
	}

	if len(newPath) > 0 && string(newPath[0]) == "/" {
		return newPath[1:]
	}

	return newPath
}
