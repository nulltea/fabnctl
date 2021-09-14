package install

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
	Use:   "cc [path]",
	Short: "Performs deployment sequence of the Fabric chaincode package",
	Long: `Performs deployment sequence of the Fabric chaincode package

Examples:
  # Deploy chaincode:
  fabnctl deploy cc -d example.com -c assets -C supply-channel -o org1 -p peer0 /contracts

  # Deploy chaincode on multiply organization and peers:
  fabnctl deploy cc -d example.com -c assets -C supply-channel -o org1 -p peer0 -o org2 -p peer1 /contracts

  # Set custom image registry and Dockerfile path:
  fabnctl deploy cc -d example.com -c assets -C supply-channel -o org1 -p peer0 -r my-registry.io -f docker_files/assets_new.Dockerfile

  # Set custom version for new chaincode or it's update:
  fabnctl deploy cc -d example.com -c assets -C supply-channel -o org1 -p peer0 -v 2.2

  # Disable image rebuild and automatic update:
  fabnctl deploy cc -d example.com -c assets -C supply-channel -o org1 -p peer0 --rebuild=false --update=false`,

	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("%q requires exactly 1 argument: [path] (chaincode source code path)",
				cmd.CommandPath())
		}
		return nil
	},
	RunE: shared.WithHandleErrors(func(cmd *cobra.Command, args []string) error {
		return installChaincode(cmd, args[0])
	}),
}

func init() {
	cmd.AddCommand(chaincodeCmd)

	chaincodeCmd.Flags().StringArrayP("org", "o", nil,
		"Organization owning chaincode. Can be used multiple times to pass list of organizations (required)")
	chaincodeCmd.Flags().StringArrayP("peer", "p", nil,
		"Peer hostname. Can be used multiply time to pass list of peers by (required)")
	chaincodeCmd.Flags().StringP("channel", "C", "", "Channel name (required)")
	chaincodeCmd.Flags().String("image", "", "Chaincode image")
	chaincodeCmd.Flags().String("source", "", "Chaincode source path")
	chaincodeCmd.Flags().StringP("chaincode", "c", "", "Chaincode name (required)")
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
	chaincodeCmd.Flags().Bool("rebuild", true, "Require chaincode image rebuild")
	chaincodeCmd.Flags().Bool("update", true,
		`In case chaincode which given name was already installed it will be updated, otherwise will be installed as a new one`,
	)
	chaincodeCmd.Flags().Float64P("version", "v", 1.0,
		"Version for chaincode commit. If not set and update will be required it will be automatically incremented",
	)

	_ = chaincodeCmd.MarkFlagRequired("org")
	_ = chaincodeCmd.MarkFlagRequired("peers")
	_ = chaincodeCmd.MarkFlagRequired("channel")
	_ = chaincodeCmd.MarkFlagRequired("chaincode")
}

func installChaincode(cmd *cobra.Command, srcPath string) error {
	var (
		logger = term.NewLogger()
		chaincodeName string
		channel string
		err error
	)

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return fmt.Errorf("%w: failed to parse required 'channel' parameter: %s", term.ErrInvalidArgs, err)
	}

	if chaincodeName, err = cmd.Flags().GetString("chaincode"); err != nil {
		return fmt.Errorf("%w: failed to parse required 'chaincode' parameter: %s", term.ErrInvalidArgs, err)
	}

	chaincode, err := fabric.NewChaincode(chaincodeName, channel,
		fabric.WithChaincodePeersFlag(cmd.Flags(), "org", "peer"),
		fabric.WithImageFlag(cmd.Flags(), "image"),
		fabric.WithSourceFlag(cmd.Flags(), "source"),
		fabric.WithVersionFlag(cmd.Flags(), "version"),
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

	if build, _ := cmd.Flags().GetBool("rebuild"); build {
		var (
			options = make([]fabric.BuildOption, 0)
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

		if err = chaincode.Build(cmd.Context(), srcPath, options...); err != nil {
			return err
		}
	}

	if err = chaincode.Install(cmd.Context()); err != nil {
		return err
	}

	return nil
}
