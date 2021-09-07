package install

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cmd2 "github.com/timoth-y/chainmetric-network/cmd"
	"github.com/timoth-y/chainmetric-network/cmd/fabnctl"
	util2 "github.com/timoth-y/chainmetric-network/pkg/terminal"
	"github.com/timoth-y/chainmetric-network/pkg/kube"
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
	Cmd.AddCommand(channelCmd)

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
		return errors.WithMessagef(fabnctl.ErrInvalidArgs, "failed to parse required parameter 'org' (organization): %s", err)
	}

	if peers, err = cmd.Flags().GetStringArray("peer"); err != nil {
		return errors.WithMessagef(fabnctl.ErrInvalidArgs, "failed to parse 'peer' parameter: %s", err)
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return errors.WithMessagef(fabnctl.ErrInvalidArgs, "failed to parse required 'channel' parameter: %s", err)
	}

	// Bind organizations arguments along with peers:
	for i, org := range orgs {
		if len(peers) < i + 1 {
			return errors.WithMessagef(fabnctl.ErrInvalidArgs, "some passed organizations missing corresponding peer parameter: %s", org)
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
			fmt.Sprintf("fabnctl/app=%s.%s.org", peer, org), cmd2.namespace,
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
			cmd2.namespace,
		); err != nil {
			return err
		} else if !ok {
			return nil
		}

		var (
			joinCmd = kube.FormShellCommand(
				"peer channel join",
				"-b", fmt.Sprintf("%s.block", channel),
			)

			fetchCmd = kube.FormShellCommand(
				"peer channel fetch config", fmt.Sprintf("%s.block", channel),
				"-c", channel,
				"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), cmd2.domain),
				"--tls", "--cafile", "$ORDERER_CA",
			)

			createCmd = kube.FormShellCommand(
				"peer channel create",
				"-c", channel,
				"-f", fmt.Sprintf("./channel-artifacts/%s.tx", channel),
				"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), cmd2.domain),
				"--tls", "--cafile", "$ORDERER_CA",
			)
		)

		if !channelExists {
			// Checking whether specified channel is already created or not,
			// by trying to fetch in genesis block:
			if _, _, err = kube.ExecShellInPod(cmd.Context(), cliPodName, cmd2.namespace, fetchCmd); err == nil {
				channelExists = true
				cmd.Println(viper.GetString("cli.info_emoji"),
					fmt.Sprintf("Channel '%s' already created, fetched its genesis block", channel),
				)
			} else if errors.Cause(err) != util2.ErrRemoteCmdFailed {
				return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
			}
		}

		var stderr io.Reader

		// Creating channel in case it wasn't yet:
		if !channelExists {
			if err = util2.DecorateWithInteractiveLog(func() error {
				if _, stderr, err = kube.ExecShellInPod(cmd.Context(), cliPodName, cmd2.namespace, createCmd); err != nil {
					if errors.Cause(err) == util2.ErrRemoteCmdFailed {
						return errors.New("Failed to create channel")
					}

					return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
				}
				return nil
			}, "Creating channel",
				fmt.Sprintf("Channel '%s' successfully created", channel),
			); err != nil {
				return util2.WrapWithStderrViewPrompt(err, stderr, false)
			}
		}

		// Joining peer to channel:
		if err = util2.DecorateWithInteractiveLog(func() error {
			if _, stderr, err = kube.ExecShellInPod(cmd.Context(), cliPodName, cmd2.namespace, joinCmd); err != nil {
				if errors.Cause(err) == util2.ErrRemoteCmdFailed {
					return errors.Wrap(err, "Failed to join channel")
				}

				return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
			}
			return nil
		}, fmt.Sprintf("Joinging '%s' organization to '%s' channel", org, channel),
			fmt.Sprintf("Organization '%s' successfully joined '%s' channel", org, channel),
		); err != nil {
			return util2.WrapWithStderrViewPrompt(err, stderr, false)
		}

		cmd.Println()
	}

	cmd.Printf("🎉 Channel '%s' successfully deployed!\n", channel)

	return nil
}
