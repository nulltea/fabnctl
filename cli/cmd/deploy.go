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
