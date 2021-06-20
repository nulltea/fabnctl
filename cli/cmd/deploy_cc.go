package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/buildx/build"
	"github.com/docker/buildx/util/progress"
	types2 "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/chainmetric-network/cli/model"
	"github.com/timoth-y/chainmetric-network/cli/shared"
	"github.com/timoth-y/chainmetric-network/cli/util"
)

// ccCmd represents the cc command
var ccCmd = &cobra.Command{
	Use:   "cc [path]",
	Short: "Performs deployment sequence of the Fabric chaincode package",
	Long: `TODO: example here`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.Errorf(
				"%q requires exactly 1 argument: [path] (chaincode source code path)", cmd.CommandPath(),
			)
		}
		return nil
	},
	RunE: handleErrors(func(cmd *cobra.Command, args []string) error {
		return deployChaincode(cmd, args[0])
	}),
}

func init() {
	deployCmd.AddCommand(ccCmd)

	ccCmd.Flags().StringP("org", "o", "", "Organization owning peer (required)")
	ccCmd.Flags().StringP("peer", "p", "peer0", "Peer hostname")
	ccCmd.Flags().StringP("channel", "C", "", "Channel name (required)")
	ccCmd.Flags().StringP("chaincode", "c", "", "Chaincode name (required)")
	ccCmd.Flags().StringP("registry", "r", "",
		`Image registry that would be used to tag and push chaincode image (required)`)
	ccCmd.Flags().String("registry-auth", "", `Registry auth credentials formatted as 'username:password'.
If nothing passed docker auth config would be searched for credentials by given domain. (default: search in docker config)"`)
	ccCmd.Flags().StringP("dockerfile", "f", "docker/{chaincode}.Dockerfile",
		"Dockerfile path relative to working path",
	)
}

func deployChaincode(cmd *cobra.Command, srcPath string) error {
	var (
		err         error
		org         string
		peer        string
		channel     string
		chaincode   string
		registry    string
		regAuth     string
		dockerfile  string
	)

	// Parse flags
	if org, err = cmd.Flags().GetString("org"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse required parameter 'org' (organization): %s", err)
	} else if len(org) == 0 {
		return errors.Wrap(ErrInvalidArgs, "Required parameter 'org' (organization) is not specified")
	}

	if peer, err = cmd.Flags().GetString("peer"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse 'peer' parameter: %s", err)
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse required 'channel' parameter: %s", err)
	} else if len(channel) == 0 {
		return errors.Wrap(ErrInvalidArgs, "Required parameter 'channel' is not specified")
	}

	if chaincode, err = cmd.Flags().GetString("chaincode"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse required 'chaincode' parameter: %s", err)
	} else if len(chaincode) == 0 {
		return errors.Wrap(ErrInvalidArgs, "Required parameter 'chaincode' is not specified")
	}

	if registry, err = cmd.Flags().GetString("registry"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse 'registry' parameter: %s", err)
	} else if len(registry) == 0 {
		return errors.Wrap(ErrInvalidArgs, "Required parameter 'registry' is not specified")
	}

	if regAuth, err = cmd.Flags().GetString("registry-auth"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse 'registry-auth' parameter: %s", err)
	}

	if dockerfile, err = cmd.Flags().GetString("dockerfile"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse 'imageReg' parameter: %s", err)
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
		fmt.Sprintf("fabnetd/app=%s.%s.org", peer, org), *namespace,
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
		*namespace,
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
		fmt.Sprintf("Chaincode has been packaged into '%s' archive", packageTarGzip),
	); err != nil {
		return nil
	}

	// Copping chaincode package to cli pod:
	if err = shared.DecorateWithInteractiveLog(func() error {
		if err = util.CopyToPod(cmd.Context(), cliPodName, *namespace, &packageBuffer, packageTarGzip); err != nil {
			return err
		}
		return nil
	}, fmt.Sprintf("Sending chaincode package to '%s' pod", cliPodName),
		fmt.Sprintf("Chaincode package has been sent to '%s' pod", cliPodName),
	); err != nil {
		return nil
	}

	// Building chaincode image:
	var (
		platform = fmt.Sprintf("linux/%s", *targetArch)
		imageTag = path.Join(registry, fmt.Sprintf("chaincodes.%s", chaincode))
		srcPathAbs = srcPath
		printer = progress.NewPrinter(cmd.Context(), os.Stdout, "auto")
	)
	srcPathAbs, _ = filepath.Abs(srcPath)
	cmd.Println("🚀 Builder for chaincode image started\n")

	dis, err := shared.DockerBuildDrivers(srcPathAbs)
	if err != nil {
		return err
	}

	if _, err = build.Build(cmd.Context(), dis, map[string]build.Options{
		"default": {
			Platforms: []v1.Platform{{
				Architecture: *targetArch,
				OS:           "linux",
			}},
			Tags: []string{imageTag},
			Inputs: build.Inputs{
				ContextPath:    srcPathAbs,
				DockerfilePath: path.Join(srcPathAbs, dockerfile),
			},
		},
	}, shared.DockerAPI(), shared.DockerCLI.ConfigFile(), printer); err != nil {
		return errors.Wrap(err, "failed to build chaincode image from source path")
	}

	_ = printer.Wait()

	cmd.Printf("\n%s Successfully built chaincode image and tagged it '%s'\n",
		viper.GetString("cli.success_emoji"), imageTag,
	)

	// Pushing chaincode image to registry
	if err = determineDockerCredentials(&registry, &regAuth); err != nil {
		return err
	}

	if err = shared.DecorateWithInteractiveLog(func() error {
		if _, err := shared.Docker.ImagePush(cmd.Context(), imageTag, types.ImagePushOptions{
			Platform: platform,
			RegistryAuth: regAuth,
		}); err != nil {
			return errors.Wrapf(err, "failed to push chaincode image to '%s' registry", registry)
		}

		return nil
	}, fmt.Sprintf("Pushing chaincode image to code to '%s' registry", registry),
		fmt.Sprintf("Chaincode image '%s' has been pushed to registry", imageTag),
	); err != nil {
		return nil
	}

	// Installing chaincode package:
	var (
		stdout io.Reader
		stderr io.Reader
	)

	if stdout, stderr, err = util.ExecCommandInPod(cmd.Context(), cliPodName, *namespace,
		"peer", "lifecycle", "chaincode", "queryinstalled",
	); err != nil {
		if errors.Cause(err) == util.ErrRemoteCmdFailed {
			return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName )
		}

		return errors.Wrap(err, "Failed to query installed chaincodes")
	}

	cmd.Println(parseQueriedPackageID(stdout, chaincode))

	if err = shared.DecorateWithInteractiveLog(func() error {
		if _, stderr, err = util.ExecCommandInPod(cmd.Context(), cliPodName, *namespace,
			"peer", "lifecycle", "chaincode", "install", packageTarGzip,
		); err != nil {
			if errors.Cause(err) == util.ErrRemoteCmdFailed {
				return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName )
			}

			return errors.Wrap(err, "Failed to install chaincode package")
		}
		return nil
	}, "Installing chaincode package", "Chaincode package has been installed"); err != nil {
		return util.WrapWithStderrViewPrompt(err, stderr)
	}

	cmd.Println(parseInstalledPackageID(stderr))

	// TODO: parse Chaincode code package identifier: devices:501147208dbb535cfa3c6e9b962f6d3e03be16d6154494588709479f49c20b5d

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

	if err := util.WriteBytesToTarGzip("connection.json", &connBuffer, &codeBuffer); err != nil {
		return errors.Wrap(err, "failed to write 'connection.json' into 'code.tar.gz' archive")
	}

	if err := util.WriteBytesToTar("code.tar.gz", &codeBuffer, packageTar); err != nil {
		return errors.Wrap(err, "failed to write 'code.tar.gz' into package tar archive")
	}

	if err := json.NewEncoder(&mdBuffer).Encode(metadata); err != nil {
		return errors.Wrap(err, "failed to encode to 'metadata.json'")
	}

	if err := util.WriteBytesToTar("metadata.json", &mdBuffer, packageTar); err != nil {
		return errors.Wrap(err, "failed to write 'metadata.json' into package tar archive")
	}

	return nil
}

func determineDockerCredentials(registry *string, regAuth *string) error {
	var (
		hostname = "https://index.docker.io/v1/"
	)

	dockerCredentials, _ := shared.DockerCLI.ConfigFile().GetAllCredentials()
	if dockerCredentials == nil {
		dockerCredentials = map[string]types2.AuthConfig{}
	}

	if strings.Contains(*registry, ".") {
		hostname = fmt.Sprintf("https://%s/", *registry)
	}

	if len(*regAuth) != 0 {
		*regAuth = base64.StdEncoding.EncodeToString([]byte(*regAuth))
		return nil
	}

	identity, ok := dockerCredentials[hostname]; if !ok {
		return errors.Wrapf(
			ErrInvalidArgs,
			"credentials for '%s' not found in docker config and missing in args", registry,
		)
	}

	*regAuth = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", identity.Username, identity.Password)))
	if len(*registry) == 0 {
		*registry = identity.Username
	}

	return nil
}

func parseInstalledPackageID(reader io.Reader) string {
	res := regexp.MustCompile("Chaincode code package identifier:(.+?)$").
		FindStringSubmatch(util.GetLastLine(reader))
	if len(res) == 2 {
		return strings.TrimSpace(res[1])
	}

	return ""
}

func parseQueriedPackageID(reader io.Reader, cc string) string {
	var buffer bytes.Buffer
	if n, err := io.Copy(&buffer, reader); err != nil || n == 0 {
		return ""
	}

	res := regexp.MustCompile(fmt.Sprintf("Package ID: (.+?), Label: %s", cc)).
		FindSubmatch(buffer.Bytes())
	if len(res) == 2 {
		return strings.TrimSpace(string(res[1]))
	}

	return ""
}
