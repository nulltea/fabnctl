package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command.
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Provides method for deploying network components",
	Long: `Provides method for deploying network components.

Examples:
  # Deploy orderer service
  fabnctl deploy orderer -d example.com

  # Deploy peer
  fabnctl deploy -d example.com peer -o org1

  # Deploy channel
  fabnctl deploy channel -d example.com -C supply-channel -o org1 -p peer0 -o org2 -p peer0  

  # Deploy chaincode (Smart Contracts package)
  fabnctl deploy cc -d example.com -C supply-channel --cc_name assets -o org1 -p peer0 -o org2 -p peer0 /contracts`,
	RunE: deploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

func deploy(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("You must specify which component to deploy. See help:\n")
	}

	if _, ok := map[string]bool {
		ordererCmd.Use: true,
	}[args[0]]; !ok {
		return  errors.Errorf("Component '%s' is unknown and can't be deploy. See help:\n\n", args[0])
	}

	return cmd.Help()
}
