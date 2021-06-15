package cmd

import (
	"github.com/spf13/cobra"
)

var (
	targetArch string
	domain     string
	chartsPath string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "fabnetd",
	Short: "Tool for deployment and configuration of the Hyperledger Fabric blockchain network",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	targetArch = *rootCmd.PersistentFlags().StringP(
		"arch", "a", "arm64", `Deployment target architecture.
Supported are:
 - ARM64: -a=arm64
 - AMD64 (x86) -a=amd64
Default is ARM64.`)

	domain = *rootCmd.PersistentFlags().StringP(
		"domain", "d", "chainmetric.network", `Deployment target domain.
Default is 'chainmetric.network'.`)

	chartsPath = *rootCmd.PersistentFlags().String(
		"charts", "./charts", `Helm deployment charts path.
Default is './charts'.`)

	namespace = *deployCmd.PersistentFlags().StringP("namespace", "n", "network",
		"namespace scope for the deployment request. Default is: network.",
	)
}
