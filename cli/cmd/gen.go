package cmd

import (
	"context"
	"fmt"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/spf13/cobra"
	"github.com/timoth-y/chainmetric-network/cli/shared"
)

// genCmd represents the gen command
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generates crypto materials and channel artifacts",
	Run: gen,
}

func init() {
	rootCmd.AddCommand(genCmd)

	rootCmd.Flags().StringP("config", "f", "./network-config.yaml",
		`Network structure config file path required for deployment. Default is './network-config.yaml'`,
	)
}

func gen(cmd *cobra.Command, args []string) {
	var (
		chartSpec = &helmclient.ChartSpec{
			ReleaseName: "artifacts",
			ChartName: fmt.Sprintf("%s/artifacts", *chartsPath),
			Namespace: "network",
			Wait: true,
		}
	)

	if *targetArch == "arm64" {
		// TODO -f=charts/artifacts/values.arm64.yaml \
		//     --set=domain=$DOMAIN
	}

	shared.Helm.InstallOrUpgradeChart(context.Background(), chartSpec)
}
