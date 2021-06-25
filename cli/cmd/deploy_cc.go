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
	"github.com/timoth-y/chainmetric-network/cli/model"
	"github.com/timoth-y/chainmetric-network/cli/shared"
	"github.com/timoth-y/chainmetric-network/cli/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// ccCmd represents the cc command
var ccCmd = &cobra.Command{
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
	RunE: handleErrors(func(cmd *cobra.Command, args []string) error {
		return deployChaincode(cmd, args[0])
	}),
}

func init() {
	deployCmd.AddCommand(ccCmd)

	ccCmd.Flags().StringArrayP("org", "o", nil,
		"Organization owning chaincode. Can be used multiply time to pass list of organizations (required)")
	ccCmd.Flags().StringArrayP("peer", "p", nil,
		"Peer hostname. Can be used multiply time to pass list of peers by (required)")
	ccCmd.Flags().StringP("channel", "C", "", "Channel name (required)")
	ccCmd.Flags().StringP("chaincode", "c", "", "Chaincode name (required)")
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
	ccCmd.Flags().Float64P("version", "v", 1.0,
		"Version for chaincode commit. If not set and update will be required it will be automatically incremented",
	)

	ccCmd.MarkFlagRequired("org")
	ccCmd.MarkFlagRequired("peers")
	ccCmd.MarkFlagRequired("channel")
	ccCmd.MarkFlagRequired("chaincode")
	ccCmd.MarkFlagFilename("dockerfile")
}

func deployChaincode(cmd *cobra.Command, srcPath string) error {
	var (
		err        error
		orgs       []string
		peers      []string
		channel    string
		chaincode  string
		registry   string
		regAuth    string
		dockerfile string
		buildImage bool
		update     bool
		version    float64
		sequence   = 1
	)

	// Parse flags
	if orgs, err = cmd.Flags().GetStringArray("org"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse required parameter 'org' (organization): %s", err)
	}

	if peers, err = cmd.Flags().GetStringArray("peer"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse required 'peer' parameter: %s", err)
	}

	if channel, err = cmd.Flags().GetString("channel"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse required 'channel' parameter: %s", err)
	}

	if chaincode, err = cmd.Flags().GetString("chaincode"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse required 'chaincode' parameter: %s", err)
	}

	if registry, err = cmd.Flags().GetString("registry"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse 'registry' parameter: %s", err)
	}

	if regAuth, err = cmd.Flags().GetString("registry-auth"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse 'registry-auth' parameter: %s", err)
	}

	if buildImage, err = cmd.Flags().GetBool("rebuild"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse 'rebuild' parameter: %s", err)
	}

	if dockerfile, err = cmd.Flags().GetString("dockerfile"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse 'imageReg' parameter: %s", err)
	}
	dockerfile = strings.ReplaceAll(dockerfile, "{chaincode}", chaincode)

	if update, err = cmd.Flags().GetBool("update"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse 'update' parameter: %s", err)
	}

	if update, err = cmd.Flags().GetBool("update"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse 'update' parameter: %s", err)
	}

	if version, err = cmd.Flags().GetFloat64("version"); err != nil {
		return errors.WithMessagef(ErrInvalidArgs, "failed to parse 'version' parameter: %s", err)
	}

	var (
		orgPeers = make(map[string]string)
		imageTag = path.Join(registry, fmt.Sprintf("chaincodes.%s", chaincode))
	)

	// Bind organizations arguments along with peers:
	for i, org := range orgs {
		if len(peers) < i + 1 {
			return errors.WithMessagef(ErrInvalidArgs, "some passed organizations missing corresponding peer parameter: %s", org)
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

	if committed, ver, seq, err := checkChaincodeCommitStatus(cmd.Context(), chaincode, channel); err != nil {
		return err
	} else if committed {
		cmd.Printf(
			"%s Chaincode '%s' is already committed on '%s' channel with version '%.1f' and sequence '%d'. ",
			viper.GetString("cli.info_emoji"), chaincode, channel, ver, seq,
		)

		if !cmd.Flags().Changed("version") {
			ver += 0.1
		}

		seq += 1

		if update {
			version = ver
			sequence = seq
			cmd.Printf("It will be updated to version '%.1f' and sequence '%d'\n", version, sequence)
		} else {
			cmd.Printf("Further steps will be skipped\n")
			return nil
		}
	}

	// Shared commands required for chaincode deployment in the letter steps:
	var (
		checkCommitReadinessCmd = util.FormShellCommand(
			"peer", "lifecycle", "chaincode", "checkcommitreadiness",
			"-n", chaincode,
			"-v", vtoa(version),
			"--sequence", stoa(sequence),
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
			fmt.Sprintf("fabnctl/app=%s.%s.org", peer, org), namespace,
		); err != nil {
			return err
		} else if !ok {
			return nil
		}

		// Waiting for 'org.peer.cli' pod readiness:
		if ok, err := util.WaitForPodReady(
			cmd.Context(),
			&cliPodName,
			fmt.Sprintf("fabnctl/app=cli.%s.%s.org", peer, org),
			namespace,
		); err != nil {
			return err
		} else if !ok {
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
				return util.WrapWithStderrViewPrompt(err, stderr, false)
			}

			return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
		}

		packageID = parseQueriedPackageID(stdout, chaincode)

		// Installing chaincode package if needed:
		if len(packageID) == 0 || !update {
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
				return util.WrapWithStderrViewPrompt(err, stderr, false)
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
				return util.WrapWithStderrViewPrompt(
					errors.Wrapf(err, "Failed to check chaincode approval by '%s' organization", org),
					stderr, true,
				)
			}

			return errors.Wrapf(err, "Failed to execute command on '%s' pod", cliPodName)
		}

		var approveCmd = util.FormShellCommand(
			"peer", "lifecycle", "chaincode", "approveformyorg",
			"-n", chaincode,
			"-v", vtoa(version),
			"--sequence", stoa(sequence),
			"--package-id", packageID,
			"--init-required=false",
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
				return util.WrapWithStderrViewPrompt(err, stderr, false)
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
			return util.WrapWithStderrViewPrompt(
				errors.Wrap(err, "Failed to check chaincode commit readiness"),
				stderr, true,
			)
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

	var commitCmd = util.FormShellCommand(
		"peer", "lifecycle", "chaincode", "commit",
		"-n", chaincode,
		"-v", vtoa(version),
		"--sequence", stoa(sequence),
		"--init-required=false",
		"-C", channel,
		"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), domain),
		"--tls", "--cafile", "$ORDERER_CA",
	)

	// Committing chaincode on peers of all given organizations:
	for org, peer := range orgPeers {
		var (
			orgHost = fmt.Sprintf("%s.org.%s", org, domain)
			peerHost = fmt.Sprintf("%s.%s", peer, orgHost)
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
		)

		commitCmd = util.FormShellCommand(commitCmd, commitCmdEnding)
	}

	cmd.Println("\n")

	var stderr io.Reader
	if err = shared.DecorateWithInteractiveLog(func() error {
		if _, stderr, err = util.ExecShellInPod(
			cmd.Context(),
			availableCliPod, namespace,
			commitCmd,
		); err != nil {
			if errors.Cause(err) == util.ErrRemoteCmdFailed {
				return errors.Wrapf(err,
					"Failed to commit chaincode",
				)
			}

			return errors.Wrapf(err, "Failed to execute command on '%s' pod", availableCliPod)
		}

		return nil
	}, "Committing chaincode on organization peers",
		"Chaincode has been committed on all organization peers",
	); err != nil {
		return util.WrapWithStderrViewPrompt(err, stderr, false)
	}

	cmd.Printf("\n🎉 Chaincode '%s' v%.1f successfully deployed!\n", chaincode, version)

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

func checkChaincodeApprovalByOrg(reader io.Reader, org string) bool {
	return regexp.MustCompile(fmt.Sprintf("(?:%s): true", org)).
		MatchReader(bufio.NewReader(reader))
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

func checkChaincodeCommitStatus(ctx context.Context, chaincode, channel string) (bool, float64, int, error) {
	var (
		availableCliPod string
		buffer bytes.Buffer
	)

	if pods, err := shared.K8s.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: "fabnctl/cid=org-peer-cli",
	}); err != nil {
		return false, 0, 0, errors.Wrap(err, "failed to find available cli pod for chaincode commit status check")
	} else if pods == nil || pods.Size() == 0 {
		return false, 0, 0, errors.New("failed to find available cli pod for chaincode commit status check")
	} else {
		availableCliPod = pods.Items[0].Name
	}

	// Checking whether the chaincode was already committed:
	stdout, stderr, err := util.ExecCommandInPod(
		ctx,
		availableCliPod, namespace,
		"peer", "lifecycle", "chaincode", "querycommitted", "-C", channel,
	)

	if err != nil {
		if errors.Cause(err) == util.ErrRemoteCmdFailed {
			return false, 0, 0, util.WrapWithStderrViewPrompt(
				errors.Wrapf(err, "Failed to check сommit status for '%s' chaincode", chaincode),
				stderr, true,
			)
		}

		return false, 0, 0, errors.Wrapf(err, "Failed to execute command on '%s' pod", availableCliPod)
	}


	if n, err := io.Copy(&buffer, stdout); err != nil || n == 0 {
		return false, 0, 0, nil
	}

	match := regexp.MustCompile(fmt.Sprintf("Name: %s, Version: (\\d.\\d?), Sequence: (\\d?)", chaincode)).
		FindStringSubmatch(buffer.String())

	if len(match) < 3 {
		return false, 0, 0, nil
	}

	return true, atov(match[1]), atos(match[2]), nil
}

func vtoa(version float64) string {
	return fmt.Sprintf("%.1f", version)
}

func atov(str string) float64 {
	version, err := strconv.ParseFloat(str, 32)
	if err != nil {
		return 1.0
	}

	return version
}

func stoa(sequence int) string {
	return fmt.Sprintf("%d", sequence)
}

func atos(str string) int {
	sequence, err := strconv.Atoi(str)
	if err != nil {
		return 1.0
	}

	return sequence
}
