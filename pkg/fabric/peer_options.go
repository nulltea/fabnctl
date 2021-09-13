package fabric

import (
	"fmt"

	"github.com/spf13/pflag"
	"github.com/timoth-y/fabnctl/pkg/term"
)

type (
	PeerOption func(args *peerArgs)

	peerArgs struct {
		installCA bool
		*sharedArgs
	}
)

func WithCA(installCA bool) PeerOption {
	return func(args *peerArgs) {
		args.installCA = installCA
	}
}

func WithCAFlag(flags *pflag.FlagSet, name string) PeerOption {
	return func(args *peerArgs) {
		var err error
		if args.installCA, err = flags.GetBool("withCA"); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s': %s",
					term.ErrInvalidArgs, name, err),
			)
		}
	}
}

func WithSharedOptionsForPeer(options ...SharedOption) PeerOption {
	return func(args *peerArgs) {
		for i := range options {
			options[i](args.sharedArgs)
		}
	}
}
