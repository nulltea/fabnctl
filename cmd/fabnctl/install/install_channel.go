package install

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/terminal"
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
		err           error
		orgs          []string
		peers         []string
		channel       string
		channelExists bool
		orgPeers = make(map[string]string)
	)

	// Parse flags
	if orgs, err = cmd.Flags().GetStringArray("org"); err != nil {
		return fmt.Errorf("%w: failed to parse required parameter 'org' (organization): %s", shared.ErrInvalidArgs, err)
	}

	if peers, err = cmd.Flags().GetStringArray("peer"); err != nil {
		return fmt.Errorf("%w: failed to parse 'peer' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return fmt.Errorf("%w: failed to parse required 'channel' parameter: %s", shared.ErrInvalidArgs, err)
	}

	// Bind organizations arguments along with peers:
	for i, org := range orgs {
		if len(peers) < i + 1 {
			return fmt.Errorf("%w: some passed organizations missing corresponding peer parameter: %s", org, shared.ErrInvalidArgs)
		}
		orgPeers[org] = peers[i]
	}

	for org, peer := range orgPeers {
		var (
			peerPodName = fmt.Sprintf("%s.%s.org", peer, org)
			cliPodName  = fmt.Sprintf("cli.%s.%s.org", peer, org)
		)

		cmd.Printf(
			"%s Going to setup channel on '%s' peer of '%s' organization:\n",
			viper.GetString("cli.info_emoji"),
			peer, org,
		)

		// Waiting for 'org.peer' pod readiness:
		if ok, err := kube.WaitForPodReady(
			cmd.Context(),
			&peerPodName,
			fmt.Sprintf("fabnctl/app=%s.%s.org", peer, org), shared.Namespace,
		); err != nil {
			return err
		} else if !ok {
			return nil
		}

		// Waiting for 'org.peer.cli' pod readiness:
		if ok, err := kube.WaitForPodReady(
			cmd.Context(),
			&cliPodName,
			fmt.Sprintf("fabnctl/app=cli.%s.%s.org", peer, org),
			shared.Namespace,
		); err != nil {
			return err
		} else if !ok {
			return nil
		}

		var (
			joinCmd = kube.FormCommand(
				"peer channel join",
				"-b", fmt.Sprintf("%s.block", channel),
			)

			fetchCmd = kube.FormCommand(
				"peer channel fetch config", fmt.Sprintf("%s.block", channel),
				"-c", channel,
				"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
				"--tls", "--cafile", "$ORDERER_CA",
			)

			createCmd = kube.FormCommand(
				"peer channel create",
				"-c", channel,
				"-f", fmt.Sprintf("./channel-artifacts/%s.tx", channel),
				"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
				"--tls", "--cafile", "$ORDERER_CA",
			)
		)

		if !channelExists {
			// Checking whether specified channel is already created or not,
			// by trying to fetch in genesis block:
			if _, _, err = kube.ExecShellInPod(cmd.Context(), cliPodName, shared.Namespace, fetchCmd); err == nil {
				channelExists = true
				cmd.Println(viper.GetString("cli.info_emoji"),
					fmt.Sprintf("Channel '%s' already created, fetched its genesis block", channel),
				)
			} else if errors.Is(err, terminal.ErrRemoteCmdFailed) {
				return fmt.Errorf("Failed to execute command on '%s' pod: %w", cliPodName, err)
			}
		}

		var stderr io.Reader

		// Creating channel in case it wasn't yet:
		if !channelExists {
			if err = terminal.DecorateWithInteractiveLog(func() error {
				if _, stderr, err = kube.ExecShellInPod(cmd.Context(), cliPodName, shared.Namespace, createCmd); err != nil {
					if errors.Is(err, terminal.ErrRemoteCmdFailed){
						return fmt.Errorf("failed to create channel")
					}

					return fmt.Errorf("Failed to execute command on '%s' pod: %w", cliPodName, err)
				}
				return nil
			}, "Creating channel",
				fmt.Sprintf("Channel '%s' successfully created", channel),
			); err != nil {
				return terminal.WrapWithStderrViewPrompt(err, stderr, false)
			}
		}

		// Joining peer to channel:
		if err = terminal.DecorateWithInteractiveLog(func() error {
			if _, stderr, err = kube.ExecShellInPod(cmd.Context(), cliPodName, shared.Namespace, joinCmd); err != nil {
				if errors.Is(err, terminal.ErrRemoteCmdFailed){
					return fmt.Errorf("Failed to join channel: %w", err)
				}

				return fmt.Errorf("Failed to execute command on '%s' pod: %w", cliPodName, err)
			}
			return nil
		}, fmt.Sprintf("Joinging '%s' organization to '%s' channel", org, channel),
			fmt.Sprintf("Organization '%s' successfully joined '%s' channel", org, channel),
		); err != nil {
			return terminal.WrapWithStderrViewPrompt(err, stderr, false)
		}

		cmd.Println()
	}

	cmd.Printf("🎉 Channel '%s' successfully deployed!\n", channel)

	return nil
}
