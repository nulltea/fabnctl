package update

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/fabric"
	"github.com/timoth-y/fabnctl/pkg/term"
)

// channelCmd represents the channel command
var updateChannelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Updates channel definition",
	Long: `Updates channel definition:

Examples:
  # Add anchor peers to channel definition:
	fabnctl update channel -c supply-channel --setAnchors -o org1 -o org2`,
	RunE: shared.WithHandleErrors(updateChannel),
}

func init() {
	cmd.AddCommand(updateChannelCmd)

	updateChannelCmd.Flags().StringArrayP("org", "o", nil, "Owner organization names (required)")
	updateChannelCmd.Flags().StringP("channel", "c", "", "Channel name (required)")
	updateChannelCmd.Flags().Bool("setAnchors", true, "Update to setup anchor peers (default option)")

	_ = updateChannelCmd.MarkFlagRequired("org")
	_ = updateChannelCmd.MarkFlagRequired("channel")
}

func updateChannel(cmd *cobra.Command, _ []string) error {
	var (
		err     error
		orgs        []string
		channelName string
		logger = term.NewLogger()
	)

	// Parse flags
	if orgs, err = cmd.Flags().GetStringArray("org"); err != nil {
		return fmt.Errorf("%w: failed to parse required parameter 'org' (organization): %s", term.ErrInvalidArgs, err)
	}

	if channelName, err = cmd.Flags().GetString("channelName"); err != nil {
		return fmt.Errorf("%w: failed to parse required 'channelName' parameter: %s", term.ErrInvalidArgs, err)
	}

	channel, err := fabric.NewChannel(channelName, fabric.WithSharedOptionsForChannel(
		fabric.WithArchFlag(cmd.Flags(), "arch"),
		fabric.WithDomainFlag(cmd.Flags(), "domain"),
		fabric.WithCustomDeployChartsFlag(cmd.Flags(), "charts"),
		fabric.WithKubeNamespaceFlag(cmd.Flags(), "namespace"),
		fabric.WithLogger(logger),
	))

	if err != nil {
		return err
	}

	if err = channel.SetAnchors(cmd.Context(), orgs...); err != nil {
		return err
	}

	return nil
}
