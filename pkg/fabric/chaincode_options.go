package fabric

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/buildx/build"
	"github.com/spf13/pflag"
	"github.com/timoth-y/fabnctl/pkg/docker"
	"github.com/timoth-y/fabnctl/pkg/ssh"
)

type (
	// ChaincodeOption allows passing additional arguments for Chaincode.
	ChaincodeOption func(*chaincodeArgs)

	chaincodeArgs struct {
		channel  string
		orgpeers map[string][]string
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
			err         error
		)

		if orgs, err = flags.GetStringArray(orgsFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s' (organization): %s", orgsFlag, err),
			)
		}

		if peers, err = flags.GetStringArray(peersFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s' (peers): %s", peersFlag, err),
			)
		}

		for i, org := range orgs {
			if len(peers) < i+1 {
				args.initErrors = append(args.initErrors,
					fmt.Errorf("some passed organizations missing corresponding peer parameter: %s", org),
				)
			}
			args.orgpeers[org] = strings.Split(peers[i], ",")
		}
	}
}

// WithChannel ...
func WithChannel(channel string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		args.channel = channel
	}
}

// WithChannelFlag ...
func WithChannelFlag(flags *pflag.FlagSet, name string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		var err error

		if args.channel, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s': %s", name, err),
			)
		}
	}
}

// WithSharedOptionsForChaincode ...
func WithSharedOptionsForChaincode(options ...SharedOption) ChaincodeOption {
	return func(args *chaincodeArgs) {
		for i := range options {
			options[i](args.sharedArgs)
		}
	}
}

type (
	// ChaincodeInstallOption allows passing additional arguments for building chaincodes.
	ChaincodeInstallOption func(*installArgs)

	installArgs struct {
		imageName     string
		withSource    bool
		sourcePath    string
		sourcePathAbs string
		update        bool
		customVersion bool
		version       float64
		sequence      int
		initErrorArgs
	}
)

// WithImage ...
func WithImage(name string) ChaincodeInstallOption {
	return func(args *installArgs) {
		args.imageName = name
	}
}

// WithImageFlag ...
func WithImageFlag(flags *pflag.FlagSet, name string) ChaincodeInstallOption {
	return func(args *installArgs) {
		var err error

		if args.imageName, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s' (image): %s", name, err),
			)
		}
	}
}

// WithSource ...
func WithSource(path string) ChaincodeInstallOption {
	return func(args *installArgs) {
		var err error

		if len(path) == 0 {
			return
		}

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
func WithSourceFlag(flags *pflag.FlagSet, name string) ChaincodeInstallOption {
	return func(args *installArgs) {
		var err error

		if args.sourcePath, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s' (source): %s", name, err),
			)
		}

		WithSource(args.sourcePath)(args)
	}
}


// WithVersion ...
func WithVersion(version float64) ChaincodeInstallOption {
	return func(args *installArgs) {
		args.customVersion = true
		args.version = version
	}
}

// WithVersionFlag ...
func WithVersionFlag(flags *pflag.FlagSet, name string) ChaincodeInstallOption {
	return func(args *installArgs) {
		var err error

		if args.version, err = flags.GetFloat64(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s' (version): %s", name, err),
			)
		}

		if flags.Changed(name) {
			args.customVersion = true
		}
	}
}

type (
	// ChaincodeBuildOption allows passing additional arguments for building chaincodes.
	ChaincodeBuildOption func(*buildArgs)

	buildArgs struct {
		sourcePath     string
		sourcePathAbs string
		target        string
		useSSH        bool
		sshOperator    *ssh.RemoteOperator
		useDocker      bool
		dockerfile     string
		dockerDriver   []build.DriverInfo
		pushImage      bool
		dockerRegistry string
		dockerAuth     string
		ignore         []string
		initErrorArgs
	}
)

// WithRemoteBuild ...
func WithRemoteBuild(options ...ssh.Option) ChaincodeBuildOption {
	return func(args *buildArgs) {
		var err error
		if args.sshOperator, err = ssh.New(options...); err != nil {
			args.initErrors = append(args.initErrors, err)
		}

		args.useSSH = true
	}
}

// WithTarget ...
func WithTarget(path string) ChaincodeBuildOption {
	return func(args *buildArgs) {
		args.target = path
	}
}

// WithTargetFlag ...
func WithTargetFlag(flags *pflag.FlagSet, name string) ChaincodeBuildOption {
	return func(args *buildArgs) {
		var err error

		if !flags.Changed(name) {
			return
		}

		if args.target, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s' (target path): %s", name, err),
			)
		}
	}
}

// WithDockerBuild ...
func WithDockerBuild(dockerfile string) ChaincodeBuildOption {
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
func WithDockerfileFlag(flags *pflag.FlagSet, name string) ChaincodeBuildOption {
	return func(args *buildArgs) {
		var err error

		if !flags.Changed(name) {
			return
		}

		if args.dockerfile, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s' (dockerfile): %s", name, err),
			)
		}

		WithDockerBuild(args.dockerfile)(args)
	}
}

// WithDockerPush ...
func WithDockerPush(registry, auth string) ChaincodeBuildOption {
	return func(args *buildArgs) {
		args.pushImage = true
		args.dockerRegistry = registry
		args.dockerAuth = auth
	}
}

// WithDockerPushFlag ...
func WithDockerPushFlag(flags *pflag.FlagSet, registryFlag, authFlag string) ChaincodeBuildOption {
	return func(args *buildArgs) {
		var err error

		if args.dockerRegistry, err = flags.GetString(registryFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s' (docker registry): %s", registryFlag, err),
			)
		}

		if args.dockerAuth, err = flags.GetString(authFlag); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s' (docker auth): %s", authFlag, err),
			)
		}

		args.pushImage = true
	}
}

// WithIgnore ...
func WithIgnore(patterns ...string) ChaincodeBuildOption {
	return func(args *buildArgs) {
		args.ignore = append(args.ignore, patterns...)
	}
}

// WithIgnoreFlag ...
func WithIgnoreFlag(flags *pflag.FlagSet, name string) ChaincodeBuildOption {
	return func(args *buildArgs) {
		patterns, err := flags.GetStringSlice(name)
		if err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse parameter '%s': %v", name, err),
			)
		}

		args.ignore = append(args.ignore, patterns...)
	}
}
