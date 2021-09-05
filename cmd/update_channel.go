package cmd

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	core2 "github.com/timoth-y/chainmetric-network/shared/core"
	"github.com/timoth-y/chainmetric-network/shared/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// channelCmd represents the channel command
var updateChannelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Updates channel definition",
	Long: `Updates channel definition:

Examples:
  # Add anchor peers to channel definition:
	fabnctl update channel -c supply-channel --setAnchors -o org1 -o org2`,
	RunE: handleErrors(updateChannel),
}

func init() {
	updateCmd.AddCommand(updateChannelCmd)

	updateChannelCmd.Flags().StringArrayP("org", "o", nil, "Owner organization names (required)")
	updateChannelCmd.Flags().StringP("channel", "c", "", "Channel name (required)")
	updateChannelCmd.Flags().Bool("setAnchors", true, "Update to setup anchor peers (default option)")

	_ = updateChannelCmd.MarkFlagRequired("org")
	_ = updateChannelCmd.MarkFlagRequired("channel")
}

func updateChannel(cmd *cobra.Command, _ []string) error {
	var (
		err     error
		orgs    []string
		channel string
	)

	// Parse flags
	if orgs, err = cmd.Flags().GetStringArray("org"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse required parameter 'org' (organization): %s", err)
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse required 'channel' parameter: %s", err)
	}

	for _, org := range orgs {
		var cliPodName string

		cmd.Printf(
			"%s Going to setup anchor peers of '%s' organization to the channel definition:\n",
			viper.GetString("cli.info_emoji"), org,
		)

		if pods, err := core2.K8s.CoreV1().Pods(namespace).List(cmd.Context(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("fabnctl/cid=org-peer-cli,fabnctl/org=%s", org),
		}); err != nil {
			return errors.Wrapf(err, "failed to find CLI pod for '%s' organization", org)
		} else if pods == nil || pods.Size() == 0 {
			return errors.Errorf("failed to find CLI pod for '%s' organization", org)
		} else {
			cliPodName = pods.Items[0].Name
		}

		var updateCmd = util.FormShellCommand(
			"peer channel update",
			"-c", channel,
			"-f", fmt.Sprintf("./channel-artifacts/%s-anchors.tx", org),
			"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), domain),
			"--tls", "--cafile", "$ORDERER_CA",
		)

		// Update channel with org's anchor peers:
		var stderr io.Reader
		if err = core2.DecorateWithInteractiveLog(func() error {
			if _, stderr, err = util.ExecShellInPod(cmd.Context(), cliPodName, namespace, updateCmd); err != nil {
				if errors.Cause(err) == util.ErrRemoteCmdFailed {
					return errors.Wrap(err, "Failed to update channel")
				}

				return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
			}
			return nil
		}, "Updating channel",
			fmt.Sprintf("Channel '%s' successfully updated", channel),
		); err != nil {
			return util.WrapWithStderrViewPrompt(err, stderr, false)
		}

		cmd.Println()
	}

	cmd.Printf("🎉 Channel '%s' successfully updated!\n", channel)

	return nil
}
