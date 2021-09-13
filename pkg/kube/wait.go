package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/pkg/term"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubectl/pkg/util/podutils"
)

// WaitForJobComplete waits for job with given 'selector' and 'name' in given 'namespace' to complete.
func WaitForJobComplete(
	ctx context.Context,
	name *string,
	selector string, namespace string,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, viper.GetDuration("k8s.wait_timeout"))
	defer cancel()

	watcher, err := Client.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return false, fmt.Errorf("failed to wait for job completion: %w", err)
	}

	return WaitForEvent(ctx, cancel,
		watcher,
		func(event watch.Event) bool {
			if job, ok := event.Object.(*batchv1.Job); ok {
				return job.Status.Succeeded == 1
			}
			return false
		},
		func() string {
			return fmt.Sprintf("Waiting for '%s' job completion", *name)
		},
		func() string {
			return fmt.Sprintf("Job '%s' succeeded", *name)
		},
		func() string {
			return fmt.Sprintf("Job '%s' taking too long to complete," +
				"please check for possible problems with 'kubectl get pod -w'",
				*name,
			)
		},
		func() string {
			return fmt.Sprintf("Timeout waiting for '%s' job completion", *name)
		},
	)
}

// WaitForPodReady waits for pod with given 'selector' and 'name' in given 'namespace' to become ready.
func WaitForPodReady(
	ctx context.Context,
	name *string,
	selector string, namespace string,
) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, viper.GetDuration("k8s.wait_timeout"))
	defer cancel()

	watcher, err := Client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return false, fmt.Errorf("failed to wait for pod readiness: %w", err)
	}

	return WaitForEvent(ctx, cancel,
		watcher,
		func(event watch.Event) bool {
			if pod, ok := event.Object.(*corev1.Pod); ok {
				*name = pod.Name
				return podutils.IsPodReady(pod)
			}
			return false
		},
		func() string {
			return fmt.Sprintf("Waiting for '%s' pod readiness", *name)
		},
		func() string {
			return fmt.Sprintf("Pod '%s' is ready", *name)
		},
		func() string {
			return fmt.Sprintf("Pod '%s' taking too long to get ready," +
				"please check for possible problems with 'kubectl get pod -w'",
				*name,
			)
		},
		func() string {
			return fmt.Sprintf("Timeout waiting for '%s' pod readiness", *name)
		},
	)
}

// WaitForEvent waits for custom event occurrence.
func WaitForEvent(
	ctx context.Context, cancel context.CancelFunc,
	watcher watch.Interface,
	onEvent func(watch.Event) bool,
	msgStart, msgSuccess, msgWarning, msgTimeout func() string,
) (bool, error) {
	var (
		timer = time.NewTimer(15 * time.Second)
		logger = term.NewLogger()
	)
	logger.Streamer.Text(msgStart())
	logger.Streamer.Start()

	LOOP: for {
		select {
		case event := <- watcher.ResultChan():
			if onEvent(event) {
				logger.Streamer.PersistWith(logger.StreamSpinners[term.LogStreamOk], " " + msgSuccess())
				logger.Streamer.Stop()
				cancel()
				break LOOP
			}
		case <- timer.C:
			logger.Streamer.Spinner(logger.StreamSpinners[term.LogStreamError])
			logger.Streamer.Text(" " + msgWarning())
		case <- ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				logger.Streamer.PersistWith(logger.StreamSpinners[term.LogStreamError], msgTimeout())
				logger.Streamer.Stop()
				return false, nil
			}
			break LOOP
		}
	}

	return true, nil
}
