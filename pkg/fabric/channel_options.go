package fabric

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

type (
	ChannelOption func(*channelArgs)

	channelArgs struct {
		orgpeers      map[string][]string
		initErrors    []error
		*sharedArgs
	}
)

// WithChannelPeers ...
func WithChannelPeers(org string, peers ...string) ChannelOption {
	return func(args *channelArgs) {
		args.orgpeers[org] = append(args.orgpeers[org], peers...)

		if len(peers) == 0 {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("organization %s missing corresponding peers parameter", org),
			)
		}
	}
}

// WithChannelPeersFlag ...
func WithChannelPeersFlag(flags *pflag.FlagSet, orgsFlag, peersFlag string) ChannelOption {
	return func(args *channelArgs) {
		var (
			orgs, peers []string
			err error
		)

		if orgs, err = flags.GetStringArray(orgsFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse required parameter '%s' (organization): %s", orgsFlag, err),
			)
		}

		if peers, err = flags.GetStringArray(peersFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse required parameter '%s' (peers): %s", peersFlag, err),
			)
		}

		for i, org := range orgs {
			if len(peers) < i + 1 {
				args.initErrors = append(args.initErrors,
					fmt.Errorf("some passed organizations missing corresponding peer parameter: %s", org),
				)
			}
			args.orgpeers[org] = strings.Split(peers[i], ",")
		}
	}
}

func WithSharedOptionsForChannel(options ...SharedOption) ChannelOption {
	return func(args *channelArgs) {
		for i := range options {
			options[i](args.sharedArgs)
		}
	}
}
