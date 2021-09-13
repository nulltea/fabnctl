package install

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/fabric"
	"github.com/timoth-y/fabnctl/pkg/term"
)

// peerCmd represents the peer command
var peerCmd = &cobra.Command{
	Use:   "peer",
	Short: "Performs deployment sequence of the Fabric peer",
	Long: `Performs deployment sequence of the Fabric peer

Examples:
  # Deploy peer:
  fabnctl deploy peer -d example.com -o org1 -p peer0

  # Deploy peer but skip CA service installation:
  fabnctl deploy peer -d example.com -o org1 -p peer0 --withCA=false`,

	RunE: shared.WithHandleErrors(installPeer),
}

func init() {
	cmd.AddCommand(peerCmd)

	peerCmd.Flags().StringP("org", "o", "", "Organization owning peer (required)")
	peerCmd.Flags().StringP("peer", "p", "peer0", "Peer hostname")
	peerCmd.Flags().Bool("withCA", true,
		"Deploy CA service along with peer",
	)

	peerCmd.MarkFlagRequired("org")
}

func installPeer(cmd *cobra.Command, args []string) error {
	var (
		err error
		org      string
		peerName string
		logger   = term.NewLogger()
	)

	// Parse flags
	if org, err = cmd.Flags().GetString("org"); err != nil {
		return fmt.Errorf("%w: failed to parse required parameter 'org' (organization): %s", term.ErrInvalidArgs, err)
	}

	if peerName, err = cmd.Flags().GetString("peerName"); err != nil {
		return fmt.Errorf("%w: failed to parse 'peerName' parameter: %s", term.ErrInvalidArgs, err)
	}

	installer, err := fabric.NewPeer(org, peerName,
		fabric.WithCAFlag(cmd.Flags(), "withCA"),
		fabric.WithSharedOptionsForPeer(
			fabric.WithArchFlag(cmd.Flags(), "arch"),
			fabric.WithDomainFlag(cmd.Flags(), "domain"),
			fabric.WithCustomDeployChartsFlag(cmd.Flags(), "charts"),
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
