package install

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/fabric"
	"github.com/timoth-y/fabnctl/pkg/term"
)

// chaincodeCmd represents the cc command
var chaincodeCmd = &cobra.Command{
	Use:   "cc [name]",
	Short: "Performs deployment sequence of the Fabric chaincode package",
	Long: `Performs deployment sequence of the Fabric chaincode package

Examples:
  # Deploy chaincode:
  fabnctl deploy cc assets -d example.com -C supply-channel -o org1 -p peer0

  # Deploy chaincode on multiply organization and peers:
  fabnctl deploy cc assets -d example.com -C supply-channel -o org1 -p peer0 -o org2 -p peer1

  # Set custom image registry and Dockerfile path:
  fabnctl deploy cc assets -d example.com -C supply-channel -o org1 -p peer0 -r my-registry.io -f docker_files/assets_new.Dockerfile

  # Set custom version for new chaincode or it's update:
  fabnctl deploy cc assets -d example.com -C supply-channel -o org1 -p peer0 -v 2.2

  # Disable image rebuild and automatic update:
  fabnctl deploy cc assets -d example.com -C supply-channel -o org1 -p peer0 --rebuild=false --update=false`,

	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("%q requires exactly 1 argument: [name] (chaincode name)",
				cmd.CommandPath())
		}
		return nil
	},
	RunE: shared.WithHandleErrors(func(cmd *cobra.Command, args []string) error {
		return installChaincode(cmd, args[0])
	}),
}

func init() {
	cmd.AddCommand(chaincodeCmd)

	chaincodeCmd.Flags().StringArrayP("org", "o", nil,
		"Organization owning chaincode. Can be used multiple times to pass list of organizations (required)")
	chaincodeCmd.Flags().StringArrayP("peers", "p", nil,
		"Peer hostname. Can be used multiply time to pass list of peers by (required)")
	chaincodeCmd.Flags().StringP("channel", "C", "", "Channel name (required)")
	chaincodeCmd.Flags().String("image", "", "Chaincode image")
	chaincodeCmd.Flags().String("source", "", "Chaincode source path")
	chaincodeCmd.Flags().Float64P("version", "v", 1.0,
		"Version for chaincode commit. If not set and update will be required it will be automatically incremented",
	)

	_ = chaincodeCmd.MarkFlagRequired("org")
	_ = chaincodeCmd.MarkFlagRequired("peers")
	_ = chaincodeCmd.MarkFlagRequired("channel")
}

func installChaincode(cmd *cobra.Command, name string) error {
	var (
		logger = term.NewLogger()
		err error
	)

	chaincode, err := fabric.NewChaincode(name,
		fabric.WithChannelFlag(cmd.Flags(), "channel"),
		fabric.WithChaincodePeersFlag(cmd.Flags(), "org", "peers"),
		fabric.WithSharedOptionsForChaincode(
			fabric.WithArchFlag(cmd.Flags(), "arch"),
			fabric.WithDomainFlag(cmd.Flags(), "domain"),
			fabric.WithCustomDeployChartsFlag(cmd.Flags(), "charts"),
			fabric.WithKubeNamespaceFlag(cmd.Flags(), "namespace"),
			fabric.WithLogger(logger),
		),
	)

	if err != nil {
		return err
	}

	if err = chaincode.Install(cmd.Context(),
		fabric.WithImageFlag(cmd.Flags(), "image"),
		fabric.WithSourceFlag(cmd.Flags(), "source"),
		fabric.WithVersionFlag(cmd.Flags(), "version"),
	); err != nil {
		return err
	}

	return nil
}
