package install

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/buildx/build"
	"github.com/docker/buildx/util/progress"
	"github.com/docker/cli/cli/command"
	clitypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	helmclient "github.com/mittwald/go-helm-client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/docker"
	"github.com/timoth-y/fabnctl/pkg/helm"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/model"
	"github.com/timoth-y/fabnctl/pkg/ssh"
	"github.com/timoth-y/fabnctl/pkg/term"
	"github.com/timoth-y/fabnctl/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
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
			return errors.Errorf(
				"%q requires exactly 1 argument: [path] (chaincode source code path)", cmd.CommandPath(),
			)
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
		"Organization owning chaincode. Can be used multiply time to pass list of organizations (required)")
	chaincodeCmd.Flags().StringArrayP("peer", "p", nil,
		"Peer hostname. Can be used multiply time to pass list of peers by (required)")
	chaincodeCmd.Flags().StringP("channel", "C", "", "Channel name (required)")
	chaincodeCmd.Flags().StringP("chaincode", "c", "", "Chaincode name (required)")
	chaincodeCmd.Flags().StringP("registry", "r", "",
		"Image registry that would be used to tag and push chaincode image (default: search in docker config)")
	chaincodeCmd.Flags().String("registry-auth", "", `Registry auth credentials formatted as 'username:password'.
If nothing passed docker auth config would be searched for credentials by given domain. (default: search in docker config)"`)
	chaincodeCmd.Flags().StringP("dockerfile", "f", "docker/{chaincode}.Dockerfile",
		"Dockerfile path relative to working path",
	)
	chaincodeCmd.Flags().Bool("ssh", true, "Build over SSH")
	chaincodeCmd.Flags().String("host", kube.Config.Host, "Remote host for SSH connection (default: get from .kube config)")
	chaincodeCmd.Flags().Int("port", 22, "Remote port for SSH connection")
	chaincodeCmd.Flags().StringP("user", "u", os.Getenv("USER"), "User from remote host for SSH connection")
	chaincodeCmd.Flags().StringSliceP("skip", "s", nil, "File patterns to skip during transfer")
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
		err          error
		orgs         []string
		peers        []string
		channel      string
		chaincode    string
		registry     string
		regAuth      string
		dockerfile   string
		buildImage   bool
		buildOverSSH bool
		sshHost      string
		sshPort      int
		sshUser      string
		sshSkip      []string
		update       bool
		version      float64
		sequence     = 1
	)

	// Parse flags
	if orgs, err = cmd.Flags().GetStringArray("org"); err != nil {
		return fmt.Errorf("%w: failed to parse required parameter 'org' (organization): %s", shared.ErrInvalidArgs, err)
	}

	if peers, err = cmd.Flags().GetStringArray("peer"); err != nil {
		return fmt.Errorf("%w: failed to parse required 'peer' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return fmt.Errorf("%w: failed to parse required 'channel' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if chaincode, err = cmd.Flags().GetString("chaincode"); err != nil {
		return fmt.Errorf("%w: failed to parse required 'chaincode' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if registry, err = cmd.Flags().GetString("registry"); err != nil {
		return fmt.Errorf("%w: failed to parse 'registry' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if regAuth, err = cmd.Flags().GetString("registry-auth"); err != nil {
		return fmt.Errorf("%w: failed to parse 'registry-auth' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if buildImage, err = cmd.Flags().GetBool("rebuild"); err != nil {
		return fmt.Errorf("%w: failed to parse 'rebuild' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if dockerfile, err = cmd.Flags().GetString("dockerfile"); err != nil {
		return fmt.Errorf("%w: failed to parse 'imageReg' parameter: %s", shared.ErrInvalidArgs, err)
	}
	dockerfile = strings.ReplaceAll(dockerfile, "{chaincode}", chaincode)

	if buildOverSSH, err = cmd.Flags().GetBool("ssh"); err != nil {
		return fmt.Errorf("%w: failed to parse 'ssh' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if sshHost, err = cmd.Flags().GetString("host"); err != nil {
		return fmt.Errorf("%w: failed to parse 'host' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if len(sshHost) == 0 {
		sshHost = kube.Config.Host
	}

	if sshPort, err = cmd.Flags().GetInt("port"); err != nil {
		return fmt.Errorf("%w: failed to parse 'port' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if sshUser, err = cmd.Flags().GetString("user"); err != nil {
		return fmt.Errorf("%w: failed to parse 'user' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if sshSkip, err = cmd.Flags().GetStringSlice("skip"); err != nil {
		return fmt.Errorf("%w: failed to parse 'skip' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if update, err = cmd.Flags().GetBool("update"); err != nil {
		return fmt.Errorf("%w: failed to parse 'update' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if update, err = cmd.Flags().GetBool("update"); err != nil {
		return fmt.Errorf("%w: failed to parse 'update' parameter: %s", shared.ErrInvalidArgs, err)
	}

	if version, err = cmd.Flags().GetFloat64("version"); err != nil {
		return fmt.Errorf("%w: failed to parse 'version' parameter: %s", shared.ErrInvalidArgs, err)
	}

	return nil
}
