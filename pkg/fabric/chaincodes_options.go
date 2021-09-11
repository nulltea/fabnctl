package fabric

import (
	"fmt"
	"path/filepath"

	"github.com/docker/buildx/build"
	"github.com/timoth-y/fabnctl/pkg/docker"
	"github.com/timoth-y/fabnctl/pkg/ssh"
	"github.com/timoth-y/fabnctl/pkg/term"
)

type (
	// ChaincodeOption allows passing additional arguments for ChaincodeInstaller.
	ChaincodeOption func(*chaincodeArgs)

	chaincodeArgs struct {
		orgpeers      map[string][]string
		imageName     string
		withSource    bool
		sourcePath    string
		sourcePathAbs string
		arch          string
		update        bool
		customVersion bool
		version       float64
		sequence      int
		logger        *term.Logger
		initErrors    []error
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

// WithImage ...
func WithImage(name string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		args.imageName = name
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

// WithVersion ...
func WithVersion(version float64, sequence int) ChaincodeOption {
	return func(args *chaincodeArgs) {
		args.customVersion = true
		args.version = version
		args.sequence = sequence
	}
}

// WithArch ...
func WithArch(arch string) ChaincodeOption {
	return func(args *chaincodeArgs) {
		args.arch = arch
	}
}

// WithLogger can be used to pass custom logger for displaying commands output.
func WithLogger(logger *term.Logger, options ...term.LoggerOption) ChaincodeOption {
	return func(args *chaincodeArgs) {
		if logger != nil {
			args.logger = logger
			return
		}

		args.logger = term.NewLogger(options...)
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

// WithDockerPush ...
func WithDockerPush(registry, auth string) BuildOption {
	return func(args *buildArgs) {
		args.dockerPush = true
		args.dockerRegistry = registry
		args.dockerAuth = auth
	}
}

// WithIgnore ...
func WithIgnore(patterns ...string) BuildOption {
	return func(args *buildArgs) {
		args.ignore = append(args.ignore, patterns...)
	}
}



