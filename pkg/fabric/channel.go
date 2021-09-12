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
)

type ChannelInstaller struct {
	channelName string
	*channelArgs
}

func NewChannelInstaller(name string, options ...ChannelOption) (*ChannelInstaller, error) {
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

	return &ChannelInstaller{
		channelName: name,
		channelArgs: args,
	}, nil
}

func (ci *ChannelInstaller) Install(ctx context.Context) error {
	var channelExists bool

	for org, peers := range ci.orgpeers {
		for _, peer := range peers {
			var (
				peerPodName = fmt.Sprintf("%s.%s.org", peer, org)
				cliPodName  = fmt.Sprintf("cli.%s.%s.org", peer, org)
			)

			ci.logger.Infof("Going to setup channel on '%s' peer of '%s' organization:", peer, org)

			// Waiting for 'org.peer' pod readiness:
			if ok, err := kube.WaitForPodReady(ctx,
				&peerPodName,
				fmt.Sprintf("fabnctl/app=%s.%s.org", peer, org), ci.kubeNamespace,
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
				ci.kubeNamespace,
			); err != nil {
				return err
			} else if !ok {
				return nil
			}

			var (
				joinCmd = kube.FormCommand(
					"peer channel join",
					"-b", fmt.Sprintf("%s.block", ci.channelName),
				)

				fetchCmd = kube.FormCommand(
					"peer channel fetch config", fmt.Sprintf("%s.block", ci.channelName),
					"-c", ci.channelName,
					"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
					"--tls", "--cafile", "$ORDERER_CA",
				)

				createCmd = kube.FormCommand(
					"peer channel create",
					"-c", ci.channelName,
					"-f", fmt.Sprintf("./channel-artifacts/%s.tx", ci.channelName),
					"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
					"--tls", "--cafile", "$ORDERER_CA",
				)
			)

			if !channelExists {
				// Checking whether specified channel is already created or not,
				// by trying to fetch in genesis block:
				if _, _, err := kube.ExecShellInPod(ctx, cliPodName, ci.kubeNamespace, fetchCmd); err == nil {
					channelExists = true
					ci.logger.Infof("Channel '%s' already created, fetched its genesis block", ci.channelName)
				} else if errors.Is(err, term.ErrRemoteCmdFailed) {
					return fmt.Errorf("failed to execute command on '%s' pod: %w", cliPodName, err)
				}
			}

			var stderr io.Reader

			// Creating channel in case it wasn't yet:
			if !channelExists {
				if err := ci.logger.Stream(func() (err error) {
					if _, stderr, err = kube.ExecShellInPod(ctx, cliPodName, ci.kubeNamespace, createCmd); err != nil {
						if errors.Is(err, term.ErrRemoteCmdFailed) {
							return fmt.Errorf("failed to create channel")
						}

						return fmt.Errorf("failed to execute command on '%s' pod: %w", cliPodName, err)
					}
					return nil
				}, "Creating channel",
					fmt.Sprintf("Channel '%s' successfully created", ci.channelName),
				); err != nil {
					return ci.logger.WrapWithStderrViewPrompt(err, stderr, false)
				}
			}

			// Joining peer to channel:
			if err := ci.logger.Stream(func() (err error) {
				if _, stderr, err = kube.ExecShellInPod(ctx, cliPodName, ci.kubeNamespace, joinCmd); err != nil {
					if errors.Is(err, term.ErrRemoteCmdFailed) {
						return fmt.Errorf("failed to join channel: %w", err)
					}

					return fmt.Errorf("failed to execute command on '%s' pod: %w", cliPodName, err)
				}
				return nil
			}, fmt.Sprintf("Joinging '%s' organization to '%s' channel", org, ci.channelName),
				fmt.Sprintf("Organization '%s' successfully joined '%s' channel", org, ci.channelName),
			); err != nil {
				return ci.logger.WrapWithStderrViewPrompt(err, stderr, false)
			}

			ci.logger.NewLine()
		}
	}

	ci.logger.Successf("Channel '%s' successfully deployed!", ci.channelName)

	return nil
}
