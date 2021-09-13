package gen

import (
	"github.com/spf13/cobra"
)

// cmd represents the gen command.
var cmd = &cobra.Command{
	Use:   "gen",
	Short: "Provides method for generation network related configuration and artifacts",
	Long: `Provides method for generation network related configuration and artifacts

Examples:
  # Generate network artifacts:
  fabnctl gen artifacts -f ./network-config.yaml

  # Generate connection config:
  fabnctl gen connection -f ./network-config.yaml
`,
}

func init() {
	cmd.PersistentFlags().StringP("config", "f", "./network-config.yaml",
		"Network structure config file path required for deployment",
	)
}

// AddTo adds generate commands to `root` cobra.Command.
func AddTo(root *cobra.Command) {
	root.AddCommand(cmd)
}
