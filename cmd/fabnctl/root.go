package fabnctl

import (
	"github.com/spf13/cobra"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/gen"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/install"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/update"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "fabnctl",
	Short: "Tool for deployment and configuration of the Hyperledger Fabric blockchain network",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	shared.AddGlobalFlags(rootCmd)

	gen.AddTo(rootCmd)
	install.AddTo(rootCmd)
	update.AddTo(rootCmd)
}


