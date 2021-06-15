package cmd

import (
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command.
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Provides method for deploying network components",
	Long: `Provides method for deploying network components.

Examples:
  # Deploy orderer service
  fabnetd deploy orderer

  # Deploy peer
  fabnetd deploy peer -o chipa-inu

  # Deploy channel
  fabnetd deploy channel -o chipa-inu -p peer0 -C supply-channel 

  # Deploy chaincode (Smart Contracts package)
  fabnetd deploy cc eploy cc -o chipa-inu -p peer0 -C supply-channel --cc_name assets
`,
	RunE: deploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

func deploy(cmd *cobra.Command, args []string) error {

	return nil
}
