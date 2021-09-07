package fabnctl

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/chainmetric-network/cmd/fabnctl/gen"
	"github.com/timoth-y/chainmetric-network/cmd/fabnctl/install"
	"github.com/timoth-y/chainmetric-network/cmd/fabnctl/update"
	"github.com/timoth-y/chainmetric-network/pkg/core"
)

var (
	targetArch string
	domain     string
	chartsPath string
)

var (
	ErrInvalidArgs = errors.New("invalid command arguments")
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "fabnctl",
	Short: "Tool for deployment and configuration of the Hyperledger Fabric blockchain network",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	core.InitCore()

	rootCmd.PersistentFlags().StringVarP(
		&targetArch,
		"arch", "a",
		"amd64",
		`Deployment target architecture.
Supported are:
 - ARM64: -a=arm64
 - AMD64 (x86) -a=amd64`,
 	)

	 rootCmd.PersistentFlags().StringVarP(
	 	&domain,
	 	"domain", "d",
	 	"",
	 	"Deployment target domain",
	 )

	rootCmd.PersistentFlags().StringVar(
		&chartsPath,
		"charts",
		viper.GetString("helm.charts_path"),
		"Helm deployment charts path",
	)

	rootCmd.MarkFlagRequired("domain")

	rootCmd.AddCommand(gen.Cmd)
	rootCmd.AddCommand(install.Cmd)
	rootCmd.AddCommand(update.Cmd)
}

func handleErrors(fn func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := fn(cmd, args); err != nil {
			if errors.Cause(err) == ErrInvalidArgs {
				return err
			}

			cmd.Println(viper.GetString("cli.error_emoji"), "Error:", err)
		}

		return nil
	}
}
