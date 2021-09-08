package update

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/terminal"
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
		orgs    []string
		channel string
	)

	// Parse flags
	if orgs, err = cmd.Flags().GetStringArray("org"); err != nil {
		return fmt.Errorf("%w: failed to parse required parameter 'org' (organization): %s", err, shared.ErrInvalidArgs)
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return fmt.Errorf("%w: failed to parse required 'channel' parameter: %s", err, shared.ErrInvalidArgs)
	}

	for _, org := range orgs {
		var cliPodName string

		cmd.Printf(
			"%s Going to setup anchor peers of '%s' organization to the channel definition:\n",
			viper.GetString("cli.info_emoji"), org,
		)

		if pods, err := kube.Client.CoreV1().Pods(shared.Namespace).List(cmd.Context(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("fabnctl/cid=org-peer-cli,fabnctl/org=%s", org),
		}); err != nil {
			return fmt.Errorf("failed to find CLI pod for '%s' organization: %w", org, err)
		} else if pods == nil || pods.Size() == 0 {
			return fmt.Errorf("failed to find CLI pod for '%s' organization", org)
		} else {
			cliPodName = pods.Items[0].Name
		}

		var updateCmd = kube.FormShellCommand(
			"peer channel update",
			"-c", channel,
			"-f", fmt.Sprintf("./channel-artifacts/%s-anchors.tx", org),
			"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
			"--tls", "--cafile", "$ORDERER_CA",
		)

		// Update channel with org's anchor peers:
		var stderr io.Reader
		if err = terminal.DecorateWithInteractiveLog(func() error {
			if _, stderr, err = kube.ExecShellInPod(cmd.Context(), cliPodName, shared.Namespace, updateCmd); err != nil {
				if errors.Is(err, terminal.ErrRemoteCmdFailed){
					return fmt.Errorf("Failed to update channel: %w", err)
				}

				return fmt.Errorf("Failed to execute command on '%s' pod: %w", cliPodName, err)
			}
			return nil
		}, "Updating channel",
			fmt.Sprintf("Channel '%s' successfully updated", channel),
		); err != nil {
			return terminal.WrapWithStderrViewPrompt(err, stderr, false)
		}

		cmd.Println()
	}

	cmd.Printf("🎉 Channel '%s' successfully updated!\n", channel)

	return nil
}
