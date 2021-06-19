package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/timoth-y/chainmetric-network/cli/model"
	"github.com/timoth-y/chainmetric-network/cli/shared"
	"github.com/timoth-y/chainmetric-network/cli/util"
)

// ccCmd represents the cc command
var ccCmd = &cobra.Command{
	Use:   "cc [PATH] | -",
	Short: "Performs deployment sequence of the Fabric chaincode package",
	Long: `TODO: example here`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.Errorf(
				"%q requires exactly 1 argument: [PATH] (chaincode source code path)", cmd.CommandPath(),
			)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return deployChaincode(cmd, args[0])
	},
}

func init() {
	deployCmd.AddCommand(ccCmd)

	ccCmd.Flags().StringP("org", "o", "", "Organization owning peer (required)")
	ccCmd.Flags().StringP("peer", "p", "peer0", "Peer hostname")
	ccCmd.Flags().StringP("channel", "C", "", "Channel name (required)")
	ccCmd.Flags().StringP("chaincode", "c", "", "Chaincode name (required)")
	ccCmd.Flags().StringP("imageReg", "r", "docker.io", "Image registry")
	ccCmd.Flags().StringP("dockerfile", "d", "docker/{chaincode}.Dockerfile",
		"Dockerfile path relative to working path",
	)
}

func deployChaincode(cmd *cobra.Command, srcPath string) error {
	var (
		err       error
		org       string
		peer      string
		channel   string
		chaincode string
		imageReg  string
		dockerfile string
	)

	// Parse flags
	if org, err = cmd.Flags().GetString("org"); err != nil {
		return errors.Wrap(err, "failed to parse required parameter 'org' (organization)")
	} else if len(org) == 0 {
		return errors.New("Required parameter 'org' (organization) is not specified")
	}

	if peer, err = cmd.Flags().GetString("peer"); err != nil {
		return errors.Wrap(err, "failed to parse 'peer' parameter")
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return errors.Wrap(err, "failed to parse required 'channel' parameter")
	} else if len(channel) == 0 {
		return errors.New("Required parameter 'channel' is not specified")
	}

	if chaincode, err = cmd.Flags().GetString("chaincode"); err != nil {
		return errors.Wrap(err, "failed to parse required 'chaincode' parameter")
	} else if len(chaincode) == 0 {
		return errors.New("Required parameter 'chaincode' is not specified")
	}

	if imageReg, err = cmd.Flags().GetString("imageReg"); err != nil {
		return errors.Wrap(err, "failed to parse 'imageReg' parameter")
	}

	if dockerfile, err = cmd.Flags().GetString("dockerfile"); err != nil {
		return errors.Wrap(err, "failed to parse 'imageReg' parameter")
	}
	dockerfile = strings.ReplaceAll(dockerfile, "{chaincode}", chaincode)

	var (
		peerPodName     = fmt.Sprintf("%s.%s.org", peer, org)
		cliPodName      = fmt.Sprintf("cli.%s.%s.org", peer, org)
		packageTarGzip  = fmt.Sprintf("%s.%s.%s.tar.gz", chaincode, peer, org)
		packageBuffer   bytes.Buffer
	)

	// Waiting for 'org.peer' pod readiness:
	if ok, err := util.WaitForPodReady(
		cmd.Context(),
		&peerPodName,
		fmt.Sprintf("fabnetd/app=%s.%s.org", peer, org), namespace,
	); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Waiting for 'org.peer.cli' pod readiness:
	if ok, err := util.WaitForPodReady(
		cmd.Context(),
		&cliPodName,
		fmt.Sprintf("fabnetd/app=cli.%s.%s.org", peer, org),
		namespace,
	); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Packaging chaincode into tar.gz archive:
	if err = shared.DecorateWithInteractiveLog(func() error {
		if err = packageExternalChaincodeInTarGzip(chaincode, peer, org, &packageBuffer); err != nil {
			return errors.Wrapf(err, "failed to package chaincode in '%s' archive", packageTarGzip)
		}
		return nil
	}, fmt.Sprintf("Packaging chaincode into '%s' archive", packageTarGzip),
		fmt.Sprintf("Chaincode has been pachaged into '%s' archive", packageTarGzip),
	); err != nil {
		return nil
	}

	// Copping chaincode package to cli pod:
	if err = shared.DecorateWithInteractiveLog(func() error {
		if err = util.CopyToPod(cmd.Context(), cliPodName, namespace, &packageBuffer, packageTarGzip); err != nil {
			return err
		}
		return nil
	}, fmt.Sprintf("Sending chaincode package to '%s' pod", cliPodName),
		fmt.Sprintf("Chaincode package has been sent to '%s' pod", cliPodName),
	); err != nil {
		return nil
	}

	// Build images and push to registry:
	var (
		platform = fmt.Sprintf("linux/%s", targetArch)
		imageTag = path.Join(imageReg, fmt.Sprintf("cc.%s", chaincode))
		dockerResp types.ImageBuildResponse
	)

	if err = shared.DecorateWithInteractiveLog(func() error {
		srcTar, err := archive.TarWithOptions(srcPath, &archive.TarOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to build tar archive of source code from path '%s'", srcPath)
		}

		shared.ILogger.Text("Source code packaged to tar archive and being now send to docker Daemon")

		if dockerResp, err = shared.Docker.ImageBuild(cmd.Context(), srcTar, types.ImageBuildOptions{
			Platform: fmt.Sprintf("linux/%s", targetArch),
			Tags: []string{imageReg, fmt.Sprintf("cc.%s", chaincode)},
			Dockerfile: dockerfile,
		}); err != nil {
			return errors.Wrap(err, "failed to build chaincode image from source path")
		}

		shared.ILogger.Text(fmt.Sprintf("Successfully built chaincode image and tagged it '%s'", imageTag))

		if _, err := shared.Docker.ImagePush(cmd.Context(), imageTag, types.ImagePushOptions{
			Platform: platform,
			// TODO reg auth
		}); err != nil {
			return errors.Wrapf(err, "failed to push chaincode image to '%s' registry", imageReg)
		}

		return nil
	}, "Packaging source code to tar archive",
		fmt.Sprintf("Chaincode image '%s' has been pushed to registry", imageTag),
	); err != nil {
		return nil
	}

	_ = channel

	return nil
}

func packageExternalChaincodeInTarGzip(chaincode, peer, org string, writer io.Writer) error {
	var (
		codeBuffer    bytes.Buffer
		mdBuffer      bytes.Buffer
		connBuffer    bytes.Buffer

		packageGzip = gzip.NewWriter(writer)
		packageTar = tar.NewWriter(packageGzip)

		metadata   = model.ChaincodeMetadata{
			Type:  "external",
			Label: chaincode,
		}
		connection = model.ChaincodeConnection{
			Address:     fmt.Sprintf("%s-chaincode-%s-%s:7052", chaincode, peer, org),
			DialTimeout: "10s",
		}
	)

	defer func() {
		if err := packageGzip.Close(); err != nil {
			shared.Logger.Error(errors.Wrapf(err, "failed to close package gzip writer"))
		}
	}()

	defer func() {
		if err := packageTar.Close(); err != nil {
			shared.Logger.Error(errors.Wrapf(err, "failed to close package tar writer"))
		}
	}()

	if err := json.NewEncoder(&connBuffer).Encode(connection); err != nil {
		return errors.Wrap(err, "failed to encode to 'connection.json'")
	}

	if err := util.WriteBytesToTarGzip("connection.json", &connBuffer, &codeBuffer, connBuffer.Len()); err != nil {
		return errors.Wrap(err, "failed to write 'connection.json' into 'code.tar.gz' archive")
	}

	if err := util.WriteBytesToTar("code.tar.gz", &codeBuffer, packageTar, codeBuffer.Len()); err != nil {
		return errors.Wrap(err, "failed to write 'code.tar.gz' into package tar archive")
	}

	if err := json.NewEncoder(&mdBuffer).Encode(metadata); err != nil {
		return errors.Wrap(err, "failed to encode to 'metadata.json'")
	}

	if err := util.WriteBytesToTar("metadata.json", &mdBuffer, packageTar, mdBuffer.Len()); err != nil {
		return errors.Wrap(err, "failed to write 'metadata.json' into package tar archive")
	}

	return nil
}
