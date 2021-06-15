package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/chainmetric-network/cli/shared"
	"github.com/timoth-y/chainmetric-network/cli/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/podutils"
	"sigs.k8s.io/yaml"
)

// genCmd represents the gen command.
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generates crypto materials and channel artifacts",
	RunE: gen,
}

func init() {
	rootCmd.AddCommand(genCmd)

	rootCmd.Flags().StringP("config", "f", "./network-config.yaml",
		`Network structure config file path required for deployment. Default is './network-.cli-config.yaml'`,
	)
}

func gen(cmd *cobra.Command, _ []string) error {
	var (
		values = make(map[string]interface{})
		chartSpec = &helmclient.ChartSpec{
			ReleaseName: "artifacts",
			ChartName: fmt.Sprintf("%s/artifacts", chartsPath),
			Namespace: namespace,
			Wait: true,
		}
		waitPodName string
		cryptoConfigDir = fmt.Sprintf(".crypto-config.%s", domain)
		channelArtifactsDir = fmt.Sprintf(".channel-artifacts.%s", domain)
	)

	if targetArch == "arm64" {
		armValues, err := util.ValuesFromFile(fmt.Sprintf("%s/artifacts/values.arm64.yaml", chartsPath))
		if err != nil {
			return err
		}
		values = armValues
	}

	values["domain"] = domain

	valuesYaml, err := yaml.Marshal(values)
	if err != nil {
		return errors.Wrap(err, "failed to encode additional values")
	}
	chartSpec.ValuesYaml = string(valuesYaml)

	shared.Helm.InstallOrUpgradeChart(cmd.Context(), chartSpec)

	ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("k8s.wait_timeout"))
	watcher, err := shared.K8s.BatchV1().Jobs(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "fabnetd/cid=artifacts.generate",
	})
	if err != nil {
		return errors.Wrap(err, "failed to wait for 'artifacts.generate' job completion")
	}

LOOP: for {
		select {
		case event := <- watcher.ResultChan():
			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				continue
			}

			if job.Status.Succeeded == 1 {
				cmd.Println("Job succeeded", job.Name)
				cancel()
				break LOOP
			}
		case <- ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				cmd.PrintErr(errors.Wrap(ctx.Err(), "timeout waiting for 'artifacts.generate' job completion"))
			}
			break LOOP
		}
	}

	if err = exec.Command("kubectl", "apply",
		"-n", namespace,
		"-f", fmt.Sprintf("%s/artifacts/artifacts-wait-job.yaml", chartsPath),
	).Run(); err != nil {
		return errors.Wrap(err, "failed to deploy 'artifacts.wait' pod")
	}

	ctx, cancel = context.WithTimeout(cmd.Context(), viper.GetDuration("k8s.wait_timeout"))
	watcher, err = shared.K8s.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "job-name=artifacts.wait",
	})
	if err != nil {
		return errors.Wrap(err, "failed to wait for 'artifacts.wait' pod readiness")
	}

LOOP2: for {
	select {
	case event := <- watcher.ResultChan():
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}

		if podutils.IsPodReady(pod) {
			cmd.Println("Pod is ready", pod.Name)
			cancel()
			waitPodName = pod.Name
			break LOOP2
		}
	case <- ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return errors.Wrap(ctx.Err(), "timeout waiting for 'artifacts.wait' pod readiness")
		}
		break LOOP2
	}
}

	if _, err = os.Stat(cryptoConfigDir); !os.IsNotExist(err) {
		exec.Command("rm", "-rf", cryptoConfigDir)
	}

	if err = exec.Command("kubectl", "cp",
		fmt.Sprintf("%s:crypto-config", waitPodName),
		cryptoConfigDir,
	).Run(); err != nil {
		return errors.Wrap(err, "failed to copy crypto-config")
	}

	cmd.Println("crypto-config has been downloaded to", cryptoConfigDir)

	if _, err = os.Stat(channelArtifactsDir); !os.IsNotExist(err) {
		exec.Command("rm", "-rf", channelArtifactsDir)
	}

	if err = exec.Command("kubectl", "cp",
		fmt.Sprintf("%s:channel-artifacts", waitPodName),
		channelArtifactsDir,
	).Run(); err != nil {
		return errors.Wrap(err, "failed to copy channel-artifacts")
	}

	cmd.Println("channel-artifacts has been downloaded to", channelArtifactsDir)

	if err = shared.K8s.BatchV1().Jobs(namespace).DeleteCollection(cmd.Context(),
		metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: "fabnetd/cid=artifacts.wait",
	}); err != nil {
		return errors.Wrap(err, "failed to delete artifacts.wait job")
	}

	if err = shared.K8s.CoreV1().Pods(namespace).DeleteCollection(cmd.Context(),
		metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: "job-name=artifacts.wait",
		}); err != nil {
		return errors.Wrap(err, "failed to delete artifacts.wait pod")
	}

	cmd.Println("Network artifacts generation done done!")

	return nil
}
