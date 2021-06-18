package cmd

import (
	"github.com/spf13/cobra"
)

// ccCmd represents the cc command
var ccCmd = &cobra.Command{
	Use:   "cc",
	Short: "Performs deployment sequence of the Fabric chaincode package",
	RunE: deployChaincode,
}

func init() {
	deployCmd.AddCommand(ccCmd)

	ccCmd.Flags().StringP("org", "o", "", "Organization owning peer (required)")
	ccCmd.Flags().StringP("peer", "p", "peer0", "Peer hostname")
	ccCmd.Flags().StringP("channel", "C", "", "Channel name (required)")
}

func deployChaincode(cmd *cobra.Command, args []string) error {

	return nil
}
