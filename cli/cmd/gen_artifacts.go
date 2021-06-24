package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/chainmetric-core/utils"
	"github.com/timoth-y/chainmetric-network/cli/shared"
	"github.com/timoth-y/chainmetric-network/cli/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// genCmd represents the gen command.
var artifactsCmd = &cobra.Command{
	Use:   "artifacts",
	Short: "Generates crypto materials and channel artifacts",
	Long: `Generates crypto materials and channel artifacts

Examples:
  # Generate:
  fabnctl gen artifacts -f ./network-config.yaml`,

	RunE: handleErrors(genArtifacts),
}

func init() {
	genCmd.AddCommand(artifactsCmd)
}

func genArtifacts(cmd *cobra.Command, _ []string) error {
	var (
		err error
		configPath string
		values = make(map[string]interface{})
		chartSpec = &helmclient.ChartSpec{
			ReleaseName: "artifacts",
			ChartName: path.Join(chartsPath, "artifacts"),
			Namespace: namespace,
			Wait: true,
			CleanupOnFail: true,
		}
		waitPodName = "artifacts.wait"
		cryptoConfigDir = fmt.Sprintf(".crypto-config.%s", domain)
		channelArtifactsDir = fmt.Sprintf(".channel-artifacts.%s", domain)
	)

	// Parsing flags:
	if configPath, err = cmd.Flags().GetString("config"); err != nil {
		return errors.Wrap(ErrInvalidArgs, "failed to parse 'config' parameter")
	}

	// Preparing additional values for chart installation:
	configValues, err := util.ValuesFromFile(configPath)
	if err != nil {
		return err
	}
	values["config"] = configValues

	if targetArch == "arm64" {
		armValues, err := util.ValuesFromFile(path.Join(chartsPath, "artifacts", "values.arm64.yaml"))
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

	// Installing artifacts helm chart:
	ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("helm.install_timeout"))
	defer cancel()

	if err = shared.DecorateWithInteractiveLog(func() error {
		if err = shared.Helm.InstallOrUpgradeChart(ctx, chartSpec); err != nil {
			return errors.Wrap(err, "failed to install artifacts helm chart")
		}
		return nil
	}, "Installing 'artifacts/artifacts' chart",
		"Chart 'artifacts/artifacts' installed successfully",
	); err != nil {
		return nil
	}

	cancel()

	// Waiting for 'artifacts.generate' job completion:
	if ok, err := util.WaitForJobComplete(
		cmd.Context(),
		utils.StringPointer("artifacts.generate"),
		"fabnctl/cid=artifacts.generate",
		namespace,
	); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Deploying 'artifacts.wait' job,
	// that will span pod for hooking to PV with generated earlier artifacts:
	if err = exec.Command("kubectl", "apply",
		"-n", namespace,
		"-f", path.Join(chartsPath, "artifacts", "artifacts-wait-job.yaml"),
	).Run(); err != nil {
		return errors.Wrap(err, "failed to deploy 'artifacts.wait' pod")
	}

	// Cleaning 'artifacts.wait' job and pod:
	defer func(cmd *cobra.Command) {
		if err = shared.K8s.BatchV1().Jobs(namespace).DeleteCollection(cmd.Context(),
			metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: "fabnctl/cid=artifacts.wait",
			}); err != nil {
			cmd.PrintErrln(errors.Wrap(err, "failed to delete artifacts.wait job"))
		}

		if err = shared.K8s.CoreV1().Pods(namespace).DeleteCollection(cmd.Context(),
			metav1.DeleteOptions{GracePeriodSeconds: utils.Int64Pointer(0)}, metav1.ListOptions{
				LabelSelector: "job-name=artifacts.wait",
			}); err != nil {
			cmd.PrintErrln(errors.Wrap(err, "failed to delete artifacts.wait pod"))
		}
	}(cmd)

	// Waiting for 'artifacts.wait' pod readiness:
	if ok, err := util.WaitForPodReady(
		cmd.Context(),
		&waitPodName,
		"job-name=artifacts.wait",
		namespace,
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
		return errors.Wrap(err, "failed to copy crypto-config")
	}

	cmd.Println(
		viper.GetString("cli.success_emoji"),
		"Files 'crypto-config' has been downloaded to",
		cryptoConfigDir,
	)

	// Downloading generated 'channel-artifacts' artifacts on local file system:
	if _, err = os.Stat(channelArtifactsDir); !os.IsNotExist(err) {
		exec.Command("rm", "-rf", channelArtifactsDir)
	}

	if err = exec.Command("kubectl", "cp",
		fmt.Sprintf("%s:channel-artifacts", waitPodName),
		channelArtifactsDir,
	).Run(); err != nil {
		return errors.Wrap(err, "failed to copy channel-artifacts")
	}

	cmd.Println(
		viper.GetString("cli.success_emoji"),
		"Files 'channel-artifacts' has been downloaded to",
		channelArtifactsDir,
	)

	cmd.Println("🎉 Network artifacts generation done!")

	return nil
}
