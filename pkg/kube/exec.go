package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/timoth-y/fabnctl/pkg/term"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecCommandInContainer executes a command in the given `container` of `pod` and return stdout, stderr as error.
func ExecCommandInContainer(
	ctx context.Context,
	pod, container, namespace string,
	cmd ...string,
) (io.Reader, io.Reader, error) {
	var (
		stdout, stderr bytes.Buffer
		req = Client.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(pod).
			Namespace(namespace).
			SubResource("exec").
			Param("container", container).
			VersionedParams(&v1.PodExecOptions{
				Container: container,
				Command:   cmd,
				Stdin:     false,
				Stdout:    true,
				Stderr:    true,
			}, scheme.ParameterCodec)
	)

	err := execute(ctx, "POST", req.URL(), Config, nil, &stdout, &stderr)
	if err != nil {
		if stdErr := term.ErrFromStderr(stderr); stdErr != nil {
			err = stdErr
		}
	}

	return &stdout, &stderr, err
}

// ExecShellInContainer executes a `sh` shell command
// in the given `container` of `pod` and return stdout, stderr and error.
func ExecShellInContainer(
	ctx context.Context,
	podName, containerName, namespace string,
	cmd string,
) (io.Reader, io.Reader, error) {
	return ExecCommandInContainer(ctx, podName, containerName, namespace, "/bin/sh", "-c", cmd)
}

// ExecCommandInPod executes a command in the default container of the given `pod` and return stdout, stderr and error.
func ExecCommandInPod(ctx context.Context, podName, namespace string, cmd ...string) (io.Reader, io.Reader, error) {
	pod, err := Client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("faield to determine container for '%s' pod: %w", podName, err)
	}

	return ExecCommandInContainer(ctx, podName, pod.Spec.Containers[0].Name, namespace, cmd...)
}

// ExecShellInPod executes a `sh` shell command
// in the default container of the given `pod` and return stdout, stderr and error.
func ExecShellInPod(ctx context.Context, podName, namespace string, cmd string)  (io.Reader, io.Reader, error)  {
	return ExecCommandInPod(ctx, podName, namespace,"/bin/sh", "-c", cmd)
}

func execute(_ context.Context, method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer) error {
	// TODO launch exec in the goroutine
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:              stdin,
		Stdout:             stdout,
		Stderr:             stderr,
	})
}

// FormCommand forms command for shell execution `sh -c "cmd"`.
func FormCommand(cmd ...string) string {
	return strings.Join(cmd, " ")
}
