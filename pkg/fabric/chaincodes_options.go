package fabric

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/buildx/build"
	"github.com/spf13/pflag"
	"github.com/timoth-y/fabnctl/pkg/docker"
	"github.com/timoth-y/fabnctl/pkg/ssh"
	"github.com/timoth-y/fabnctl/pkg/term"
)

type (
	// ChaincodeOption allows passing additional arguments for Chaincode.
	ChaincodeOption func(*chaincodeArgs)

	chaincodeArgs struct {
		orgpeers      map[string][]string
		imageName     string
		withSource    bool
		sourcePath    string
		sourcePathAbs string
		update        bool
		customVersion bool
		version       float64
		sequence      int
		*sharedArgs
	}
)

// WithChaincodePeers ...
func WithChaincodePeers(org string, peers ...string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		args.orgpeers[org] = append(args.orgpeers[org], peers...)

		if len(peers) == 0 {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("organization %s missing corresponding peers parameter", org),
			)
		}
	}
}

// WithChaincodePeersFlag ...
func WithChaincodePeersFlag(flags *pflag.FlagSet, orgsFlag, peersFlag string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		var (
			orgs, peers []string
			err error
		)

		if orgs, err = flags.GetStringArray(orgsFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s' (organization): %s",
					term.ErrInvalidArgs, orgsFlag, err),
			)
		}

		if peers, err = flags.GetStringArray(peersFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s' (peers): %s",
					term.ErrInvalidArgs, peersFlag, err),
			)
		}

		for i, org := range orgs {
			if len(peers) < i + 1 {
				args.initErrors = append(args.initErrors,
					fmt.Errorf("%w: some passed organizations missing corresponding peer parameter: %s",
						term.ErrInvalidArgs, org),
				)
			}
			args.orgpeers[org] = strings.Split(peers[i], ",")
		}
	}
}

// WithImage ...
func WithImage(name string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		args.imageName = name
	}
}

// WithImageFlag ...
func WithImageFlag(flags *pflag.FlagSet, name string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		var err error

		if args.imageName, err =flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s' (image): %s",
					term.ErrInvalidArgs, name, err),
			)
		}
	}
}

// WithSource ...
func WithSource(path string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		var err error
		if args.sourcePathAbs, err = filepath.Abs(path); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("absolute path '%s' of source does not exists: %w", args.sourcePathAbs, err),
			)
		}

		args.withSource = true
		args.sourcePath = path
	}
}

// WithSourceFlag ...
func WithSourceFlag(flags *pflag.FlagSet, name string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		var err error

		if args.sourcePath, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s' (source): %s",
					term.ErrInvalidArgs, name, err),
			)
		}

		WithSource(args.sourcePath)(args)
	}
}

func WithSharedOptionsForChaincode(options ...SharedOption) ChaincodeOption {
	return func(args *chaincodeArgs) {
		for i := range options {
			options[i](args.sharedArgs)
		}
	}
}

// WithVersion ...
func WithVersion(version float64) ChaincodeOption {
	return func(args *chaincodeArgs) {
		args.customVersion = true
		args.version = version
	}
}

// WithVersionFlag ...
func WithVersionFlag(flags *pflag.FlagSet, name string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		var err error

		if args.version, err = flags.GetFloat64(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s' (version): %s",
					term.ErrInvalidArgs, name, err),
			)
		}

		args.customVersion = true
	}
}

type (
	// BuildOption allows passing additional arguments for building chaincodes.
	BuildOption func(*buildArgs)

	buildArgs struct {
		sourcePath     string
		sourcePathAbs  string
		useSSH         bool
		sshOperator    *ssh.RemoteOperator
		useDocker      bool
		dockerfile     string
		dockerDriver   []build.DriverInfo
		dockerPush     bool
		dockerRegistry string
		dockerAuth     string
		ignore         []string
		initErrors     []error
	}
)

// WithRemoteBuild ...
func WithRemoteBuild(options ...ssh.Option) BuildOption {
	return func(args *buildArgs) {
		var err error
		if args.sshOperator, err = ssh.New(options...); err != nil {
			args.initErrors = append(args.initErrors, err)
		}

		args.useSSH = true
	}
}

// WithDockerBuild ...
func WithDockerBuild(dockerfile string) BuildOption {
	return func(args *buildArgs) {
		var err error
		args.dockerfile = dockerfile
		if args.dockerDriver, err = docker.BuildDrivers(args.sourcePathAbs); err != nil {
			args.initErrors = append(args.initErrors, err)
		}

		args.useDocker = true
	}
}

// WithDockerfileFlag ...
func WithDockerfileFlag(flags *pflag.FlagSet, name string) BuildOption {
	return func(args *buildArgs) {
		var err error

		if args.dockerfile, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s' (dockerfile): %s",
					term.ErrInvalidArgs, name, err),
			)
		}

		WithDockerBuild(args.dockerfile)(args)
	}
}

// WithDockerPush ...
func WithDockerPush(registry, auth string) BuildOption {
	return func(args *buildArgs) {
		args.dockerPush = true
		args.dockerRegistry = registry
		args.dockerAuth = auth
	}
}

// WithDockerPushFlag ...
func WithDockerPushFlag(flags *pflag.FlagSet, registryFlag, authFlag string) BuildOption {
	return func(args *buildArgs) {
		var err error

		if args.dockerRegistry, err = flags.GetString(registryFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s' (docker registry): %s",
					term.ErrInvalidArgs, registryFlag, err),
			)
		}

		if args.dockerAuth, err = flags.GetString(authFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s' (docker auth): %s",
					term.ErrInvalidArgs, authFlag, err),
			)
		}

		args.dockerPush = true
	}
}

// WithIgnore ...
func WithIgnore(patterns ...string) BuildOption {
	return func(args *buildArgs) {
		args.ignore = append(args.ignore, patterns...)
	}
}

// WithIgnoreFlag ...
func WithIgnoreFlag(flags *pflag.FlagSet, name string) BuildOption {
	return func(args *buildArgs) {
		patterns, err := flags.GetStringSlice(name)
		if err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("%w: failed to parse required parameter '%s': %v",
					term.ErrInvalidArgs, name, err),
			)
		}

		args.ignore = append(args.ignore, patterns...)
	}
}
