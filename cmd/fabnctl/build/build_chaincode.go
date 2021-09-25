package build

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/fabric"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/ssh"
	"github.com/timoth-y/fabnctl/pkg/term"
)

// chaincodeCmd represents the cc command
var chaincodeCmd = &cobra.Command{
	Use:   "cc [name] [path]",
	Short: "Performs building of the Fabric chaincode package",
	Long: `Performs building sequence of the Fabric chaincode package

Examples:
  # Build chaincode over SSH:
  fabnctl build assets --shh --host=192.168.69.8 -u=root -t smartcontracts/assets .

  # Build chaincode local with Docker:
  fabnctl build assets -f ./docker/assets.Dockerfile -t registry/assets-contract .

  # Set custom image registry and Dockerfile path:
  fabnctl build assets -f ./docker/assets.Dockerfile -r my-registry.io -f docker_files/assets_new.Dockerfile .`,

	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("%q requires exactly 2 argument: [name] (chaincode name) [path] (chaincode source code path)",
				cmd.CommandPath())
		}
		return nil
	},
	RunE: shared.WithHandleErrors(func(cmd *cobra.Command, args []string) error {
		return buildChaincode(cmd, args[0], args[1])
	}),
}

func init() {
	cmd.AddCommand(chaincodeCmd)
	chaincodeCmd.Flags().StringP("target", "t", "smartcontracts/[name]", "Bazel build target or Docker image tag")
	chaincodeCmd.Flags().StringP("registry", "r", "",
		"Image registry that would be used to tag and push chaincode image (default: search in docker config)")
	chaincodeCmd.Flags().String("registry-auth", "", `Registry auth credentials formatted as 'username:password'.
If nothing passed docker auth config would be searched for credentials by given domain. (default: search in docker config)"`)
	chaincodeCmd.Flags().StringP("dockerfile", "f", "docker/{chaincode}.Dockerfile",
		"Dockerfile path relative to working path",
	)
	chaincodeCmd.Flags().Bool("push", false, "Push image to remote registry")
	chaincodeCmd.Flags().Bool("ssh", true, "Build over SSH")
	chaincodeCmd.Flags().String("host", kube.Config.Host, "Remote host for SSH connection (default: get from .kube config)")
	chaincodeCmd.Flags().Int("port", 22, "Remote port for SSH connection")
	chaincodeCmd.Flags().StringP("user", "u", os.Getenv("USER"), "User from remote host for SSH connection")
	chaincodeCmd.Flags().StringSliceP("ignore", "i", nil, "File patterns to skip during transfer")
}

func buildChaincode(cmd *cobra.Command, chaincodeName, srcPath string) error {
	var (
		logger = term.NewLogger()
		err error
	)

	chaincode, err := fabric.NewChaincode(chaincodeName,
		fabric.WithSharedOptionsForChaincode(
			fabric.WithArchFlag(cmd.Flags(), "arch"),
			fabric.WithDomainFlag(cmd.Flags(), "domain"),
			fabric.WithCustomDeployChartsFlag(cmd.Flags(), "charts"),
			fabric.WithKubeNamespaceFlag(cmd.Flags(), "namespace"),
			fabric.WithLogger(logger),
		),
	)

	if err != nil {
		return err
	}

	var (
		options = make([]fabric.ChaincodeBuildOption, 0)
	)

	if useSSH, _ := cmd.Flags().GetBool("ssh"); useSSH {
		options = append(options,
			fabric.WithRemoteBuild(
				ssh.WithHostFlag(cmd.Flags(), "host"),
				ssh.WithPortFlag(cmd.Flags(), "port"),
				ssh.WithUserFlag(cmd.Flags(), "user"),
			),
			fabric.WithIgnoreFlag(cmd.Flags(), "ignore"),
		)
	} else {
		options = append(options,
			fabric.WithDockerfileFlag(cmd.Flags(), "dockerfile"),
		)
	}

	if pushImage, _ := cmd.Flags().GetBool("push"); pushImage {
		options = append(options,
			fabric.WithDockerPushFlag(cmd.Flags(), "registry", "registry-auth"),
		)
	}

	if cmd.Flags().Changed("target") {
		options = append(options, fabric.WithTargetFlag(cmd.Flags(), "target"))
	}

	if err = chaincode.Build(cmd.Context(), srcPath, options...); err != nil {
		return err
	}


	return nil
}
