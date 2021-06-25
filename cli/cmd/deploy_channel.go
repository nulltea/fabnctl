package cmd

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/chainmetric-network/cli/shared"
	"github.com/timoth-y/chainmetric-network/cli/util"
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

	RunE: deployChannel,
}

func init() {
	deployCmd.AddCommand(channelCmd)

	channelCmd.Flags().StringArrayP("org", "o", nil,
		"Organization owning peer. Can be used multiply time to pass list of organizations (required)",
	)
	channelCmd.Flags().StringArrayP("peer", "p", nil,
		"Peer hostname. Can be used multiply time to pass list of peers by (required)",
	)
	channelCmd.Flags().StringP("channel", "c", "", "Channel name (required)")

	channelCmd.MarkFlagRequired("org")
	channelCmd.MarkFlagRequired("peer")
	channelCmd.MarkFlagRequired("channel")
}

func deployChannel(cmd *cobra.Command, args []string) error {
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
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse required parameter 'org' (organization): %s", err)
	}

	if peers, err = cmd.Flags().GetStringArray("peer"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse 'peer' parameter: %s", err)
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse required 'channel' parameter: %s", err)
	}

	// Bind organizations arguments along with peers:
	for i, org := range orgs {
		if len(peers) < i + 1 {
			return errors.WithMessagef(ErrInvalidArgs, "some passed organizations missing corresponding peer parameter: %s", org)
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
		if ok, err := util.WaitForPodReady(
			cmd.Context(),
			&peerPodName,
			fmt.Sprintf("fabnctl/app=%s.%s.org", peer, org), namespace,
		); err != nil {
			return err
		} else if !ok {
			return nil
		}

		// Waiting for 'org.peer.cli' pod readiness:
		if ok, err := util.WaitForPodReady(
			cmd.Context(),
			&cliPodName,
			fmt.Sprintf("fabnctl/app=cli.%s.%s.org", peer, org),
			namespace,
		); err != nil {
			return err
		} else if !ok {
			return nil
		}

		var (
			joinCmd = util.FormShellCommand(
				"peer channel join",
				"-b", fmt.Sprintf("%s.block", channel),
			)

			fetchCmd = util.FormShellCommand(
				"peer channel fetch config", fmt.Sprintf("%s.block", channel),
				"-c", channel,
				"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), domain),
				"--tls", "--cafile", "$ORDERER_CA",
			)

			createCmd = util.FormShellCommand(
				"peer channel create",
				"-c", channel,
				"-f", fmt.Sprintf("./channel-artifacts/%s.tx", channel),
				"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), domain),
				"--tls", "--cafile", "$ORDERER_CA",
			)
		)

		if !channelExists {
			// Checking whether specified channel is already created or not,
			// by trying to fetch in genesis block:
			if _, _, err = util.ExecShellInPod(cmd.Context(), cliPodName, namespace, fetchCmd); err == nil {
				channelExists = true
				cmd.Println(viper.GetString("cli.info_emoji"),
					fmt.Sprintf("Channel '%s' already created, fetched its genesis block", channel),
				)
			} else if errors.Cause(err) != util.ErrRemoteCmdFailed {
				return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
			}
		}

		var stderr io.Reader

		// Creating channel in case it wasn't yet:
		if !channelExists {
			if err = shared.DecorateWithInteractiveLog(func() error {
				if _, stderr, err = util.ExecShellInPod(cmd.Context(), cliPodName, namespace, createCmd); err != nil {
					if errors.Cause(err) == util.ErrRemoteCmdFailed {
						return errors.New("Failed to create channel")
					}

					return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
				}
				return nil
			}, "Creating channel",
				fmt.Sprintf("Channel '%s' successfully created", channel),
			); err != nil {
				return util.WrapWithStderrViewPrompt(err, stderr, false)
			}
		}

		// Joining peer to channel:
		if err = shared.DecorateWithInteractiveLog(func() error {
			if _, stderr, err = util.ExecShellInPod(cmd.Context(), cliPodName, namespace, joinCmd); err != nil {
				if errors.Cause(err) == util.ErrRemoteCmdFailed {
					return errors.Wrap(err, "Failed to join channel")
				}

				return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
			}
			return nil
		}, fmt.Sprintf("Joinging '%s' organization to '%s' channel", org, channel),
			fmt.Sprintf("Organization '%s' successfully joined '%s' channel", org, channel),
		); err != nil {
			return util.WrapWithStderrViewPrompt(err, stderr, false)
		}

		cmd.Println()
	}

	cmd.Printf("🎉 Channel '%s' successfully deployed!\n", channel)

	return nil
}
