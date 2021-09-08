package install

import (
	"fmt"

	"github.com/spf13/cobra"
)

// cmd represents the deployment command.
var cmd = &cobra.Command{
	Use:   "install",
	Short: "Provides method for installing network components",
	Long: `Provides method for installing network components.

Examples:
  # Deploy orderer service
  fabnctl install orderer -d example.com

  # Deploy peer
  fabnctl install -d example.com peer -o org1

  # Deploy channel
  fabnctl install channel -d example.com -C supply-channel -o org1 -p peer0 -o org2 -p peer0  

  # Deploy chaincode (Smart Contracts package)
  fabnctl deploy cc -d example.com -C supply-channel --cc_name assets -o org1 -p peer0 -o org2 -p peer0 /contracts`,
	RunE: deploy,
}

func deploy(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("You must specify which component to deploy. See help:\n")
	}

	if _, ok := map[string]bool {
		ordererCmd.Use: true,
	}[args[0]]; !ok {
		return  fmt.Errorf("Component '%s' is unknown and can't be deploy. See help:\n\n", args[0])
	}

	return cmd.Help()
}

// AddTo adds install commands to `root` cobra.Command.
func AddTo(root *cobra.Command) {
	root.AddCommand(cmd)
}
