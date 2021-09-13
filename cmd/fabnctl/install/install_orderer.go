package install

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/fabric"
	"github.com/timoth-y/fabnctl/pkg/term"
)

// ordererCmd represents the orderer command.
var ordererCmd = &cobra.Command{
	Use:   "orderer",
	Short: "Performs deployment sequence of the Fabric orderer service",
	Long: `Performs deployment sequence of the Fabric orderer service

Examples:
  # Deploy orderer:
  fabnctl deploy orderer -d example.com`,

	RunE: shared.WithHandleErrors(installOrderer),
}

func init() {
	cmd.AddCommand(ordererCmd)
}

func installOrderer(cmd *cobra.Command, _ []string) error {
	var logger = term.NewLogger()

	installer, err := fabric.NewOrderer(viper.GetString("fabric.orderer_hostname_name"),
		fabric.WithArchFlag(cmd.Flags(), "arch"),
		fabric.WithDomainFlag(cmd.Flags(), "domain"),
		fabric.WithCustomDeployChartsFlag(cmd.Flags(), "charts"),
		fabric.WithKubeNamespaceFlag(cmd.Flags(), "namespace"),
		fabric.WithLogger(logger),

	)

	if err != nil {
		return err
	}

	if err = installer.Install(cmd.Context()); err != nil {
		return err
	}
	return nil
}
