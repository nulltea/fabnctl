package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var (
	targetArch *string
	domain     *string
	chartsPath *string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "chainet",
	Short: "Tool for deployment and configuration of the Hyperledger Fabric blockchain network",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	targetArch = rootCmd.PersistentFlags().StringP(
		"arch", "a", "arm64", `Deployment target architecture.
Supported are:
 - ARM64: -a=arm64
 - AMD64 (x86) -a=amd64
Default is ARM64.`)

	domain = rootCmd.PersistentFlags().StringP(
		"domain", "d", "chainmetric.network", `Deployment target domain.
Default is 'chainmetric.network'.`)

	chartsPath = rootCmd.PersistentFlags().String(
		"charts", "./charts", `Helm deployment charts path.
Default is './charts'.`)

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".cli")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
