package shared

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	ErrInvalidArgs = fmt.Errorf("invalid command arguments")
)

// WithHandleErrors wraps cobra.Command with error handling middleware.
func WithHandleErrors(fn func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := fn(cmd, args); err != nil {
			if errors.Is(err, ErrInvalidArgs) {
				return err
			}

			cmd.Println(viper.GetString("cli.error_emoji"), "Error:", err)
		}

		return nil
	}
}
