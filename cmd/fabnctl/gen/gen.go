package gen

import (
	"github.com/spf13/cobra"
)

// Cmd represents the gen command.
var Cmd = &cobra.Command{
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
	Cmd.PersistentFlags().StringP("config", "f", "./network-config.yaml",
		"Network structure config file path required for deployment",
	)
}
