package fabric

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/timoth-y/fabnctl/pkg/term"
)

type (
	SharedOption func(*sharedArgs)

	sharedArgs struct {
		domain       string
		arch         string
		configPath   string
		chartsPath    string
		kubeNamespace string
		logger        *term.Logger
		initErrors    []error
	}
)

// WithDomain ...
func WithDomain(domain string) SharedOption {
	return func(args *sharedArgs) {
		args.domain = domain
	}
}

// WithDomainFlag ...
func WithDomainFlag(flags *pflag.FlagSet, name string) SharedOption {
	return func(args *sharedArgs) {
		var err error

		if args.domain, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse required parameter '%s' (domain name): %s", name, err),
			)
		}
	}
}

// WithNetworkConfig ...
func WithNetworkConfig(path string) SharedOption {
	return func(args *sharedArgs) {
		args.configPath = path
	}
}

// WithNetworkConfigFlag ...
func WithNetworkConfigFlag(flags *pflag.FlagSet, name string) SharedOption {
	return func(args *sharedArgs) {
		var err error

		if args.domain, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse required parameter '%s' (network config path): %s", name, err),
			)
		}
	}
}

// WithKubeNamespace ...
func WithKubeNamespace(namespace string) SharedOption {
	return func(args *sharedArgs) {
		args.kubeNamespace = namespace
	}
}

// WithKubeNamespaceFlag ...
func WithKubeNamespaceFlag(flags *pflag.FlagSet, name string) SharedOption {
	return func(args *sharedArgs) {
		var err error

		if args.kubeNamespace, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse required parameter '%s' (Kubernetes namespace): %s", name, err),
			)
		}
	}
}

// WithCustomDeployCharts ...
func WithCustomDeployCharts(path string) SharedOption {
	return func(args *sharedArgs) {
		var err error
		if args.chartsPath, err = filepath.Abs(path); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("absolute path '%s' of source does not exists: %w", path, err),
			)
		}
	}
}

// WithCustomDeployChartsFlag ...
func WithCustomDeployChartsFlag(flags *pflag.FlagSet, name string) SharedOption {
	return func(args *sharedArgs) {
		path, err := flags.GetString(name)
		if err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse required parameter '%s' (helm charts path): %s", name, err),
			)
		}

		WithCustomDeployCharts(path)(args)
	}
}

// WithArch ...
func WithArch(arch string) SharedOption {
	return func(args *sharedArgs) {
		args.arch = arch
	}
}

// WithArchFlag ...
func WithArchFlag(flags *pflag.FlagSet, name string) SharedOption {
	return func(args *sharedArgs) {
		var err error

		if args.arch, err = flags.GetString(name); err != nil {
			args.initErrors = append(args.initErrors,
				fmt.Errorf("failed to parse required parameter '%s' (architecture): %s", name, err),
			)
		}
	}
}

// WithLogger can be used to pass custom logger for displaying commands output.
func WithLogger(logger *term.Logger, options ...term.LoggerOption) SharedOption {
	return func(args *sharedArgs) {
		if logger != nil {
			args.logger = logger
			return
		}

		args.logger = term.NewLogger(options...)
	}
}

func (a *sharedArgs) Error() error {
	var errs = make([]string, 0, len(a.initErrors))

	for _, err := range a.initErrors {
		errs = append(errs, err.Error())
	}

	return fmt.Errorf("%w: %s", term.ErrInvalidArgs, strings.Join(errs, ", "))
}
