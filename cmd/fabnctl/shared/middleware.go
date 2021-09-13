package shared

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/timoth-y/fabnctl/pkg/term"
)

// WithHandleErrors wraps cobra.Command with error handling middleware.
func WithHandleErrors(fn func(cmd *cobra.Command, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := fn(cmd, args); err != nil {
			if errors.Is(err, term.ErrInvalidArgs) {
				return err
			}

			cmd.Println("Error:", err)
		}

		return nil
	}
}
