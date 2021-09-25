package fabric

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/term"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Channel struct {
	channelName string
	*channelArgs
}

func NewChannel(name string, options ...ChannelOption) (*Channel, error) {
	var args = &channelArgs{
		orgpeers: make(map[string][]string),
		sharedArgs: &sharedArgs{
			arch: "amd64",
			kubeNamespace: "network",
			logger: term.NewLogger(),
			chartsPath: "./network-config.yaml",
		},
	}

	for i := range options {
		options[i](args)
	}

	if len(args.initErrors) > 0 {
		return nil, args.Error()
	}

	return &Channel{
		channelName: name,
		channelArgs: args,
	}, nil
}

func (c *Channel) Install(ctx context.Context) error {
	var channelExists bool

	for org, peers := range c.orgpeers {
		for _, peer := range peers {
			var (
				peerPodName = fmt.Sprintf("%s.%s.org", peer, org)
				cliPodName  = fmt.Sprintf("cli.%s.%s.org", peer, org)
			)

			c.logger.Infof("Going to setup channel on '%s' peer of '%s' organization:", peer, org)

			// Waiting for 'org.peer' pod readiness:
			if ok, err := kube.WaitForPodReady(ctx,
				&peerPodName,
				fmt.Sprintf("fabnctl/app=%s.%s.org", peer, org), c.kubeNamespace,
			); err != nil {
				return err
			} else if !ok {
				return nil
			}

			// Waiting for 'org.peer.cli' pod readiness:
			if ok, err := kube.WaitForPodReady(
				ctx,
				&cliPodName,
				fmt.Sprintf("fabnctl/app=cli.%s.%s.org", peer, org),
				c.kubeNamespace,
			); err != nil {
				return err
			} else if !ok {
				return nil
			}

			var (
				joinCmd = kube.FormCommand(
					"peer channel join",
					"-b", fmt.Sprintf("%s.block", c.channelName),
				)

				fetchCmd = kube.FormCommand(
					"peer channel fetch config", fmt.Sprintf("%s.block", c.channelName),
					"-c", c.channelName,
					"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
					"--tls", "--cafile", "$ORDERER_CA",
				)

				createCmd = kube.FormCommand(
					"peer channel create",
					"-c", c.channelName,
					"-f", fmt.Sprintf("./channel-artifacts/%s.tx", c.channelName),
					"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
					"--tls", "--cafile", "$ORDERER_CA",
				)
			)

			if !channelExists {
				// Checking whether specified channel is already created or not,
				// by trying to fetch in genesis block:
				if _, _, err := kube.ExecShellInPod(ctx, cliPodName, c.kubeNamespace, fetchCmd); err == nil {
					channelExists = true
					c.logger.Infof("Channel '%s' already created, fetched its genesis block", c.channelName)
				} else if !errors.Is(err, term.ErrRemoteCmdFailed) {
					return fmt.Errorf("failed to execute command on '%s' pod: %w", cliPodName, err)
				}
			}

			var stderr io.Reader

			// Creating channel in case it wasn't yet:
			if !channelExists {
				if err := c.logger.Stream(func() (err error) {
					if _, stderr, err = kube.ExecShellInPod(ctx, cliPodName, c.kubeNamespace, createCmd); err != nil {
						if errors.Is(err, term.ErrRemoteCmdFailed) {
							return fmt.Errorf("failed to create channel")
						}

						return fmt.Errorf("failed to execute command on '%s' pod: %w", cliPodName, err)
					}
					return nil
				}, "Creating channel",
					fmt.Sprintf("Channel '%s' successfully created", c.channelName),
				); err != nil {
					return c.logger.WrapWithStderrViewPrompt(err, stderr, false)
				}
			}

			// Joining peer to channel:
			if err := c.logger.Stream(func() (err error) {
				if _, stderr, err = kube.ExecShellInPod(ctx, cliPodName, c.kubeNamespace, joinCmd); err != nil {
					if errors.Is(err, term.ErrRemoteCmdFailed) {
						return fmt.Errorf("failed to join channel: %w", err)
					}

					return fmt.Errorf("failed to execute command on '%s' pod: %w", cliPodName, err)
				}
				return nil
			}, fmt.Sprintf("Joinging '%s' organization to '%s' channel", org, c.channelName),
				fmt.Sprintf("Organization '%s' successfully joined '%s' channel", org, c.channelName),
			); err != nil {
				return c.logger.WrapWithStderrViewPrompt(err, stderr, false)
			}

			c.logger.NewLine()
		}
	}

	c.logger.Successf("Channel '%s' successfully deployed!", c.channelName)

	return nil
}

func (c *Channel) SetAnchors(ctx context.Context, orgs ...string) error {
	for _, org := range orgs {
		var cliPodName string

		c.logger.Infof("Going to setup anchor peers of '%s' organization to the channel definition:", org)

		if pods, err := kube.Client.CoreV1().Pods(shared.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("fabnctl/cid=org-peer-cli,fabnctl/org=%s", org),
		}); err != nil {
			return fmt.Errorf("failed to find CLI pod for '%s' organization: %w", org, err)
		} else if pods == nil || pods.Size() == 0 {
			return fmt.Errorf("failed to find CLI pod for '%s' organization", org)
		} else {
			cliPodName = pods.Items[0].Name
		}

		var updateCmd = kube.FormCommand(
			"peer channel update",
			"-c", c.channelName,
			"-f", fmt.Sprintf("./channel-artifacts/%s-anchors.tx", org),
			"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
			"--tls", "--cafile", "$ORDERER_CA",
		)

		// Update channel with org's anchor peers:
		var stderr io.Reader
		if err := c.logger.Stream(func() (err error) {
			if _, stderr, err = kube.ExecShellInPod(ctx, cliPodName, shared.Namespace, updateCmd); err != nil {
				if errors.Is(err, term.ErrRemoteCmdFailed){
					return fmt.Errorf("Failed to update channel: %w", err)
				}

				return fmt.Errorf("Failed to execute command on '%s' pod: %w", cliPodName, err)
			}
			return nil
		}, "Updating channel",
			fmt.Sprintf("Channel '%s' successfully updated", c.channelName),
		); err != nil {
			return c.logger.WrapWithStderrViewPrompt(err, stderr, false)
		}

		c.logger.NewLine()
	}

	c.logger.Successf("Channel '%s' successfully updated!", c.channelName)

	return nil
}
