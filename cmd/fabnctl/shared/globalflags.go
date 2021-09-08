package shared

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	TargetArch string
	Domain     string
	ChartsPath string
	Namespace  string
)

func AddGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(
		&TargetArch,
		"arch", "a",
		"amd64",
		`Deployment target architecture.
Supported are:
 - ARM64: -a=arm64
 - AMD64 (x86) -a=amd64`,
	)

	cmd.PersistentFlags().StringVarP(
		&Domain,
		"domain", "d",
		"",
		"Deployment target domain",
	)

	cmd.PersistentFlags().StringVar(
		&ChartsPath,
		"charts",
		viper.GetString("helm.charts_path"),
		"Helm deployment charts path",
	)

	cmd.PersistentFlags().StringVarP(
		&Namespace,
		"namespace", "n",
		"network",
		"namespace scope for the deployment request",
	)

	cmd.MarkFlagRequired("domain")
}
