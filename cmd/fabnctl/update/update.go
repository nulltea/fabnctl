package update

import (
	"fmt"

	"github.com/spf13/cobra"
)

// cmd represents the update command
var cmd = &cobra.Command{
	Use:   "update",
	Short: "Provides methods for updating network component",
	Long: `Provides methods for updating network component:

Examples:
  # Update channel definition:
	fabnctl update channel -c supply-channel --setAnchors -o org1 -o org2`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("update called")
	},
}

// AddTo adds update commands to `root` cobra.Command.
func AddTo(root *cobra.Command) {
	root.AddCommand(cmd)
}
