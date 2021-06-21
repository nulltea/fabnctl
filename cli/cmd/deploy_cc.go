package cmd

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
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
	"github.com/docker/cli/cli/command"
	clitypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	helmclient "github.com/mittwald/go-helm-client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/chainmetric-network/cli/model"
	"github.com/timoth-y/chainmetric-network/cli/shared"
	"github.com/timoth-y/chainmetric-network/cli/util"
	"sigs.k8s.io/yaml"
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

	ccCmd.Flags().StringArrayP("org", "o", nil,
		"Organization owning chaincode. Can be used multiply time to pass list of organizations")
	ccCmd.Flags().StringArrayP("peer", "p", nil,
		"Peer hostname. Can be used multiply time to pass list of peers by")
	ccCmd.Flags().StringP("channel", "C", "", "Channel name")
	ccCmd.Flags().StringP("chaincode", "c", "", "Chaincode name")
	ccCmd.Flags().StringP("registry", "r", "",
		"Image registry that would be used to tag and push chaincode image (default: search in docker config)")
	ccCmd.Flags().String("registry-auth", "", `Registry auth credentials formatted as 'username:password'.
If nothing passed docker auth config would be searched for credentials by given domain. (default: search in docker config)"`)
	ccCmd.Flags().StringP("dockerfile", "f", "docker/{chaincode}.Dockerfile",
		"Dockerfile path relative to working path",
	)
	ccCmd.Flags().Bool("rebuild", true, "Require chaincode image rebuild")
	ccCmd.Flags().Bool("update", true,
		`In case chaincode which given name was already installed it will be updated, otherwise will be installed as a new one`,
	)

	ccCmd.MarkFlagRequired("org")
	ccCmd.MarkFlagRequired("peers")
	ccCmd.MarkFlagRequired("channel")
	ccCmd.MarkFlagRequired("chaincode")
	ccCmd.MarkFlagFilename("dockerfile")
}

func deployChaincode(cmd *cobra.Command, srcPath string) error {
	var (
		err             error
		orgs            []string
		peers           []string
		channel         string
		chaincode       string
		registry        string
		regAuth         string
		dockerfile      string
		buildImage      bool
		updateInstalled bool
	)

	// Parse flags
	if orgs, err = cmd.Flags().GetStringArray("org"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse required parameter 'org' (organization): %s", err)
	}

	if peers, err = cmd.Flags().GetStringArray("peer"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse required 'peer' parameter: %s", err)
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse required 'channel' parameter: %s", err)
	}

	if chaincode, err = cmd.Flags().GetString("chaincode"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse required 'chaincode' parameter: %s", err)
	}

	if registry, err = cmd.Flags().GetString("registry"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse 'registry' parameter: %s", err)
	}

	if regAuth, err = cmd.Flags().GetString("registry-auth"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse 'registry-auth' parameter: %s", err)
	}

	if buildImage, err = cmd.Flags().GetBool("rebuild"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse 'rebuild' parameter: %s", err)
	}

	if dockerfile, err = cmd.Flags().GetString("dockerfile"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse 'imageReg' parameter: %s", err)
	}
	dockerfile = strings.ReplaceAll(dockerfile, "{chaincode}", chaincode)

	if updateInstalled, err = cmd.Flags().GetBool("update"); err != nil {
		return errors.Wrapf(ErrInvalidArgs, "failed to parse 'update' parameter: %s", err)
	}

	var (
		orgPeers = make(map[string]string)
		imageTag = path.Join(registry, fmt.Sprintf("chaincodes.%s", chaincode))
	)

	// Bind organizations arguments along with peers:
	for i, org := range orgs {
		if len(peers) < i + 1 {
			return errors.Wrapf(ErrInvalidArgs, "some passed organizations missing corresponding peer parameter: %s", org)
		}
		orgPeers[org] = peers[i]
	}

	// Building chaincode image:
	if buildImage {
		var (
			platform   = fmt.Sprintf("linux/%s", targetArch)
			srcPathAbs = srcPath
			printer    = progress.NewPrinter(cmd.Context(), os.Stdout, "auto")
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
					Architecture: targetArch,
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

		cmd.Printf("\n🚀 Pushing chaincode image to '%s' registry\n\n", registry)

		resp, err := shared.Docker.ImagePush(cmd.Context(), imageTag, types.ImagePushOptions{
			Platform:     platform,
			RegistryAuth: regAuth,
			All: true,
		})
		if err != nil {
			return errors.Wrapf(err, "failed to push chaincode image to '%s' registry", registry)
		}

		_ = jsonmessage.DisplayJSONMessagesToStream(resp, shared.DockerCLI.Out(), nil)

		cmd.Printf("\n%s Chaincode image '%s' has been pushed to registry\n",
			viper.GetString("cli.success_emoji"), imageTag,
		)
	}

	// Shared commands required for chaincode deployment in the latter steps:
	var (
		checkCommitReadinessCmd = util.FormShellCommand(
			"peer", "lifecycle", "chaincode", "checkcommitreadiness",
			"-n", chaincode,
			"-v", "1.0",
			"--sequence", "1",
			"-C", channel,
			"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), domain),
			"--tls", "--cafile", "$ORDERER_CA",
		)

		availableCliPod string
	)

	// Iterate over given organization and peer pairs and perform chaincode installation
	for org, peer := range orgPeers {
		var (
			peerPodName    = fmt.Sprintf("%s.%s.org", peer, org)
			cliPodName     = fmt.Sprintf("cli.%s.%s.org", peer, org)
			packageTarGzip = fmt.Sprintf("%s.%s.%s.tar.gz", chaincode, peer, org)
			packageBuffer  bytes.Buffer
		)

		cmd.Printf(
			"\n%s Going to install chaincode on '%s' peer of '%s' organization:\n",
			viper.GetString("cli.info_emoji"),
			peer, org,
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
			fmt.Sprintf("Chaincode has been packaged into '%s' archive", packageTarGzip),
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

		var (
			stdout    io.Reader
			stderr    io.Reader
			packageID string
		)

		// Checking whether the chaincode was already installed:
		if stdout, stderr, err = util.ExecCommandInPod(cmd.Context(), cliPodName, namespace,
			"peer", "lifecycle", "chaincode", "queryinstalled",
		); err != nil {
			if errors.Cause(err) == util.ErrRemoteCmdFailed {
				err = errors.Wrap(err, "Failed to query installed chaincodes")
				cmd.Println(viper.GetString("cli.error_emoji"), "Error:", err)
				return util.WrapWithStderrViewPrompt(err, stderr)
			}

			return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
		}

		packageID = parseQueriedPackageID(stdout, chaincode)

		// Installing chaincode package if needed:
		if len(packageID) == 0 || !updateInstalled {
			if err = shared.DecorateWithInteractiveLog(func() error {
				if _, stderr, err = util.ExecCommandInPod(cmd.Context(), cliPodName, namespace,
					"peer", "lifecycle", "chaincode", "install", packageTarGzip,
				); err != nil {
					if errors.Cause(err) == util.ErrRemoteCmdFailed {
						return errors.Wrap(err, "Failed to install chaincode package")
					}

					return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
				}

				return nil
			}, "Installing chaincode package", "Chaincode package has been installed"); err != nil {
				return util.WrapWithStderrViewPrompt(err, stderr)
			}

			packageID = parseInstalledPackageID(stderr)
		}

		fmt.Printf("%s Chaincode package identifier: %s\n", viper.GetString("cli.info_emoji"), packageID)

		// Preparing additional values for chart installation:
		var (
			values    = make(map[string]interface{})
			chartSpec = &helmclient.ChartSpec{
				ReleaseName: fmt.Sprintf("%s-cc-%s-%s", chaincode, peer, org),
				ChartName:   path.Join(chartsPath, "chaincode"),
				Namespace:   namespace,
				Wait:        true,
			}
		)

		values["image"] = map[string]interface{} {
			"repository": imageTag,
		}

		values["peer"] = peer
		values["org"] = org
		values["chaincode"] = chaincode
		values["ccid"] = packageID

		valuesYaml, err := yaml.Marshal(values)
		if err != nil {
			return errors.Wrap(err, "failed to encode additional values")
		}

		chartSpec.ValuesYaml = string(valuesYaml)

		// Installing orderer helm chart:
		ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("helm.install_timeout"))

		if err = shared.DecorateWithInteractiveLog(func() error {
			defer cancel()
			if err = shared.Helm.InstallOrUpgradeChart(ctx, chartSpec); err != nil {
				return errors.Wrap(err, "failed to install chaincode helm chart")
			}
			return nil
		}, "Installing chaincode chart",
			fmt.Sprintf("Chart 'chaincode/%s' installed successfully", chartSpec.ReleaseName),
		); err != nil {
			return nil
		}

		// Checking whether the chaincode was already approved by organization:
		if stdout, stderr, err = util.ExecShellInPod(
			cmd.Context(),
			cliPodName, namespace,
			checkCommitReadinessCmd,
		); err != nil {
			if errors.Cause(err) == util.ErrRemoteCmdFailed {
				cmd.Println(viper.GetString("cli.error_emoji"), "Error:", err)
				return util.WrapWithStderrViewPrompt(
					errors.Wrapf(err, "Failed to check chaincode approval by '%s' organization", org),
					stderr,
				)
			}

			return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
		}

		var approveCmd = util.FormShellCommand(
			"peer", "lifecycle", "chaincode", "approveformyorg",
			"-n", chaincode,
			"-v", "1.0",
			"--sequence", "1",
			"--package-id", packageID,
			"--init-required", "false",
			"-C", channel,
			"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), domain),
			"--tls", "--cafile", "$ORDERER_CA",
		)

		// Approving chaincode if needed:
		if !checkChaincodeApprovalByOrg(stdout, org) {
			if err = shared.DecorateWithInteractiveLog(func() error {
				if _, stderr, err = util.ExecShellInPod(
					cmd.Context(),
					cliPodName, namespace,
					approveCmd,
				); err != nil {
					if errors.Cause(err) == util.ErrRemoteCmdFailed {
						return errors.Wrapf(err, "Failed to approve chaincode for '%s' organization", org)
					}

					return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
				}

				return nil
			}, "Approving chaincode",
				fmt.Sprintf("Chaincode has been approved for '%s' organization", org),
			); err != nil {
				return util.WrapWithStderrViewPrompt(err, stderr)
			}
		} else {
			cmd.Printf(
				"%s Chaincode is already approved by '%s' organization\n",
				viper.GetString("cli.info_emoji"), org,
			)
		}

		availableCliPod = cliPodName
	}

	// Verifying commit readiness,
	// by checking that all organizations on channel approved chaincode:
	if stdout, stderr, err := util.ExecShellInPod(
		cmd.Context(),
		availableCliPod, namespace,
		checkCommitReadinessCmd,
	); err != nil {
		if errors.Cause(err) == util.ErrRemoteCmdFailed {
			err = errors.Wrap(err, "Failed to check chaincode commit readiness")
			cmd.Println(viper.GetString("cli.error_emoji"), "Error:", err)
			return util.WrapWithStderrViewPrompt(err, stderr)
		}

		return errors.Wrapf(err, "Failed to execute command on '%s' pod", availableCliPod)
	} else if ready, notApprovedBy := checkChaincodeCommitReadiness(stdout); !ready {
		return errors.Errorf(
			"Chaincode isn't ready to be commited, some organizations on '%s' channel haven't approved it yet: %s",
			channel, strings.Join(notApprovedBy, ", "),
		)
	} else {
		cmd.Printf(
			"%s Chaincode has been approved by all organizations on '%s' channel, it's ready to be committed",
			viper.GetString("cli.ok_emoji"),
			channel,
		)
	}

	var commitPartialCmd = util.FormShellCommand(
		"peer", "lifecycle", "chaincode", "commit",
		"-n", chaincode,
		"-v", "1.0",
		"--sequence", "1",
		"--init-required", "false",
		"-C", channel,
		"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), domain),
		"--tls", "--cafile", "$ORDERER_CA",
	)

	// Committing chaincode on peers of all given organizations:
	for org, peer := range orgPeers {
		var (
			stderr io.Reader
			orgHost = fmt.Sprintf("%s.%s", org, domain)
			peerHost = fmt.Sprintf("%s.%s", org, orgHost)
			cryptoConfigPathBase = "/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto-config"
			commitCmdEnding = util.FormShellCommand(
				"--peerAddresses", fmt.Sprintf("%s:443", peerHost),
				"--tlsRootCertFiles", path.Join(
					cryptoConfigPathBase,
					"peerOrganizations", orgHost,
					"peers", peerHost,
					"tls", "ca.crt",
				),
			)
			commitForOrgCmd = util.FormShellCommand(commitPartialCmd, commitCmdEnding)
		)

		if err = shared.DecorateWithInteractiveLog(func() error {
			if _, stderr, err = util.ExecCommandInPod(
				cmd.Context(),
				availableCliPod, namespace,
				commitForOrgCmd,
			); err != nil {
				if errors.Cause(err) == util.ErrRemoteCmdFailed {
					return errors.Wrapf(err,
						"Failed to commit chaincode on '%s' peer of '%s' organization",
						peer, org,
					)
				}

				return errors.Wrapf(err, "Failed to execute command on '%s' pod", availableCliPod)
			}

			return nil
		}, fmt.Sprintf("Committing chaincode on '%s' peer of '%s' organization", peer, org),
			fmt.Sprintf("Chaincode has been commited on '%s' peer of '%s' organization", peer, org),
		); err != nil {
			cmd.Println(viper.GetString("cli.error_emoji"), "Error:", err)
			return util.WrapWithStderrViewPrompt(err, stderr)
		}
	}

	cmd.Printf("\n🎉 Chaincode '%s' successfully deployed!\n", chaincode)

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
		err error
		hostname = "https://index.docker.io/v1/"
	)

	dockerCredentials, _ := shared.DockerCLI.ConfigFile().GetAllCredentials()
	if dockerCredentials == nil {
		dockerCredentials = map[string]clitypes.AuthConfig{}
	}

	if strings.Contains(*registry, ".") {
		hostname = fmt.Sprintf("https://%s/", *registry)
	}

	if len(*regAuth) != 0 {
		auth := types.AuthConfig{ServerAddress: hostname}
		if identity := strings.Split(*regAuth, ":"); len(identity) == 2 {
			auth.Username = identity[0]
			auth.Password = identity[1]
		} else {
			auth.IdentityToken = *regAuth
		}

		if *regAuth, err = command.EncodeAuthToBase64(auth); err != nil {
			return errors.Wrap(err, "failed to encode registry auth")
		}

		return nil
	}

	identity, ok := dockerCredentials[hostname]; if !ok {
		return errors.Wrapf(
			ErrInvalidArgs,
			"credentials for '%s' not found in docker config and missing in args", *registry,
		)
	}

	if payload, err := json.Marshal(identity); err != nil {
		return errors.Wrap(err, "failed to encode registry auth")
	} else {
		*regAuth = base64.StdEncoding.EncodeToString(payload)
	}

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

func checkChaincodeApprovalByOrg(reader io.Reader, org ...string) bool {
	return len(regexp.MustCompile(fmt.Sprintf("(?:%s): true", strings.Join(org, "|"))).
		FindReaderIndex(bufio.NewReader(reader))) == len(org)
}

func checkChaincodeCommitReadiness(reader io.Reader) (ready bool, notApprovedBy []string) {
	var buffer bytes.Buffer
	if n, err := io.Copy(&buffer, reader); err != nil || n == 0 {
		return false, nil
	}

	matches := regexp.MustCompile("(.*)(?:: false)").
		FindAllStringSubmatch(buffer.String(), -1)

	if len(matches) == 0 {
		return true, nil
	}

	for _, groups := range matches {
		notApprovedBy = append(notApprovedBy, groups[1])
	}

	return false, notApprovedBy
}
