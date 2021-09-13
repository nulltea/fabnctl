package gen

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/mittwald/go-helm-client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/helm"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/term"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// cmd represents the gen command.
var artifactsCmd = &cobra.Command{
	Use:   "artifacts",
	Short: "Generates crypto materials and channel artifacts",
	Long: `Generates crypto materials and channel artifacts

Examples:
  # Generate:
  fabnctl gen artifacts -f ./network-config.yaml`,

	RunE: shared.WithHandleErrors(genArtifacts),
}

func init() {
	cmd.AddCommand(artifactsCmd)
}

func genArtifacts(cmd *cobra.Command, _ []string) error {
	var (
		err error
		configPath string
		values = make(map[string]interface{})
		chartSpec = &helmclient.ChartSpec{
			ReleaseName:   "artifacts",
			ChartName:     path.Join(shared.ChartsPath, "artifacts"),
			Namespace:     shared.Namespace,
			Wait:          true,
			CleanupOnFail: true,
		}
		waitPodName = "artifacts.wait"
		genJobName = "artifacts.generate"
		cryptoConfigDir = fmt.Sprintf(".crypto-config.%s", shared.Domain)
		channelArtifactsDir = fmt.Sprintf(".channel-artifacts.%s", shared.Domain)
		logger = term.NewLogger()
	)

	// Parsing flags:
	if configPath, err = cmd.Flags().GetString("config"); err != nil {
		return fmt.Errorf("%w: failed to parse 'config' parameter", term.ErrInvalidArgs)
	}

	// Preparing additional values for chart installation:
	if shared.TargetArch == "arm64" {
		armValues, err := helm.ValuesFromFile(path.Join(shared.ChartsPath, "artifacts", "values.arm64.yaml"))
		if err != nil {
			return err
		}
		values = armValues
	}

	configValues, err := helm.ValuesFromFile(configPath)
	if err != nil {
		return err
	}
	values["config"] = configValues

	values["domain"] = shared.Domain

	valuesYaml, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("failed to encode additional values: %w", err)
	}
	chartSpec.ValuesYaml = string(valuesYaml)

	// Installing artifacts helm chart:
	ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("helm.install_timeout"))
	defer cancel()

	if err = logger.Stream(func() error {
		if err = helm.Client.InstallOrUpgradeChart(ctx, chartSpec); err != nil {
			return fmt.Errorf("failed to install artifacts helm chart: %w", err)
		}
		return nil
	}, "Installing 'artifacts/artifacts' chart",
		"Chart 'artifacts/artifacts' installed successfully",
	); err != nil {
		return nil
	}

	cancel()

	// Waiting for 'artifacts.generate' job completion:
	if ok, err := kube.WaitForJobComplete(
		cmd.Context(),
		&genJobName,
		"fabnctl/cid=artifacts.generate",
		shared.Namespace,
	); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Deploying 'artifacts.wait' job,
	// that will span pod for hooking to PV with generated earlier artifacts:
	if err = exec.Command("kubectl", "apply",
		"-n", shared.Namespace,
		"-f", path.Join(shared.ChartsPath, "artifacts", "artifacts-wait-job.yaml"),
	).Run(); err != nil {
		return fmt.Errorf("failed to deploy 'artifacts.wait' pod: %w", err)
	}

	// Cleaning 'artifacts.wait' job and pod:
	defer func(cmd *cobra.Command) {
		if err = kube.Client.BatchV1().Jobs(shared.Namespace).DeleteCollection(cmd.Context(),
			metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: "fabnctl/cid=artifacts.wait",
			}); err != nil {
			cmd.PrintErrln(fmt.Errorf("failed to delete artifacts.wait job: %w", err))
		}

		var zero int64 = 0

		if err = kube.Client.CoreV1().Pods(shared.Namespace).DeleteCollection(cmd.Context(),
			metav1.DeleteOptions{GracePeriodSeconds: &zero}, metav1.ListOptions{
				LabelSelector: "job-name=artifacts.wait",
			}); err != nil {
			cmd.PrintErrln(fmt.Errorf("failed to delete artifacts.wait pod: %w", err))
		}
	}(cmd)

	// Waiting for 'artifacts.wait' pod readiness:
	if ok, err := kube.WaitForPodReady(
		cmd.Context(),
		&waitPodName,
		"job-name=artifacts.wait",
		shared.Namespace,
	); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Downloading generated 'crypto-config' artifacts on local file system:
	if _, err = os.Stat(cryptoConfigDir); !os.IsNotExist(err) {
		exec.Command("rm", "-rf", cryptoConfigDir)
	}

	if err = exec.Command("kubectl", "cp",
		fmt.Sprintf("%s:crypto-config", waitPodName),
		cryptoConfigDir,
	).Run(); err != nil {
		return fmt.Errorf("failed to copy crypto-config: %w", err)
	}

	logger.Successf("Files 'crypto-config' has been downloaded to %s", cryptoConfigDir)

	// Downloading generated 'channel-artifacts' artifacts on local file system:
	if _, err = os.Stat(channelArtifactsDir); !os.IsNotExist(err) {
		exec.Command("rm", "-rf", channelArtifactsDir)
	}

	if err = exec.Command("kubectl", "cp",
		fmt.Sprintf("%s:channel-artifacts", waitPodName),
		channelArtifactsDir,
	).Run(); err != nil {
		return fmt.Errorf("failed to copy channel-artifacts: %w", err)
	}

	logger.Successf("Files 'channel-artifacts' has been downloaded to %s", channelArtifactsDir)

	cmd.Println("🎉 Network artifacts generation done!")

	return nil
}
