package install

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/timoth-y/fabnctl/pkg/fabric"
	"github.com/timoth-y/fabnctl/pkg/term"
)

// channelCmd represents the channel command
var channelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Performs setup sequence of the Fabric channel for organization",
	Long: `Performs setup sequence of the Fabric channel for organization

Examples:
  # Deploy channel:
  fabnctl deploy channel -d example.com -C supply-channel -o org1 -p peer0

  # Deploy channel on multiply organization and peers:
  fabnctl deploy channel -d example.com -C supply-channel -o org1 -p peer0 -o org2 -p peer1`,

	RunE: installChannel,
}

func init() {
	cmd.AddCommand(channelCmd)

	channelCmd.Flags().StringArrayP("org", "o", nil,
		"Organization names. Can be used multiply time to pass list of organizations (required)",
	)
	channelCmd.Flags().StringArrayP("peer", "p", nil,
		"Peer hostname. Can be used multiply time to pass list of peers by (required)",
	)
	channelCmd.Flags().StringP("channel", "c", "", "Channel name (required)")

	_ = channelCmd.MarkFlagRequired("org")
	_ = channelCmd.MarkFlagRequired("peer")
	_ = channelCmd.MarkFlagRequired("channel")
}

func installChannel(cmd *cobra.Command, _ []string) error {
	var (
		logger = term.NewLogger()
		channelName string
		err error
	)

	if channelName, err = cmd.Flags().GetString("channel"); err != nil {
		return fmt.Errorf("%w: failed to parse required 'channel' parameter: %s", term.ErrInvalidArgs, err)
	}

	installer, err := fabric.NewChannelInstaller(channelName,
		fabric.WithChannelPeersFlag(cmd.Flags(), "org", "peer"),
		fabric.WithSharedOptionsForChannel(
			fabric.WithArchFlag(cmd.Flags(), "arch"),
			fabric.WithKubeNamespaceFlag(cmd.Flags(), "namespace"),
			fabric.WithLogger(logger),
		),
	)

	if err != nil {
		return err
	}

	if err = installer.Install(cmd.Context()); err != nil {
		return err
	}

	return nil
}
