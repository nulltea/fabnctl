package build

import (
	"github.com/spf13/cobra"
)

// cmd represents the deployment command.
var cmd = &cobra.Command{
	Use:   "build",
	Short: "Provides method for building network components",
	Long: `Provides method for building network components.

Examples:
  # Deploy chaincode (Smart Contracts package)
  fabnctl build cc -d example.com -C supply-channel --cc_name assets -o org1 -p peer0 -o org2 -p peer0 /contracts`,
}


// AddTo adds install commands to `root` cobra.Command.
func AddTo(root *cobra.Command) {
	root.AddCommand(cmd)
}
