package fabric

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	"github.com/timoth-y/fabnctl/pkg/helm"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/model"
	"github.com/timoth-y/fabnctl/pkg/term"
	"github.com/timoth-y/fabnctl/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// ChaincodeInstaller defines methods for building and installing chaincodes as an external services.
type ChaincodeInstaller struct {
	org           string
	peer          string
	channel       string
	chaincodeName string
	*chaincodeArgs
}

// NewChaincodeInstaller constructs new ChaincodeInstaller instance.
func NewChaincodeInstaller(name, channel string, options ...ChaincodeOption) (*ChaincodeInstaller, error) {
	args := &chaincodeArgs{
		imageName: fmt.Sprintf("smartcontracts/%s:image", name),
		orgpeers: make(map[string][]string),
		version: 1,
		sequence: 1,
		update: true,
		sharedArgs: &sharedArgs{
			arch: "amd64",
			kubeNamespace: "network",
			logger: term.NewLogger(),
			chartsPath: "./network-config.yaml",
		},
	}

	for i := range options {
		options[i](args)
	}

	if len(args.initErrors) > 0 {
		return nil, args.Error()
	}

	return &ChaincodeInstaller{
		channel:       channel,
		chaincodeName: name,
		chaincodeArgs: args,
	}, nil
}

func (ci *ChaincodeInstaller) Install(ctx context.Context) error {
	var (
		orgPeers = make(map[string]string)
	)

	if committed, ver, seq, err := ci.checkChaincodeCommitStatus(ctx); err != nil {
		return err
	} else if committed {
		ci.logger.Infof(
			"Chaincode '%s' is already committed on '%s' channel with version '%.1f' and sequence '%d'",
			ci.chaincodeName, ci.channel, ver, seq,
		)

		if !ci.customVersion {
			ver += 0.1
		}

		seq += 1

		if ci.update {
			ci.version = ver
			ci.sequence = seq
			ci.logger.Infof("It will be updated to version '%.1f' and sequence '%d'", ci.version, ci.sequence)
		} else {
			ci.logger.Infof("Further steps will be skipped")
			return nil
		}
	}

	// Shared commands required for chaincode deployment in the letter steps:
	var (
		checkCommitReadinessCmd = kube.FormCommand(
			"peer", "lifecycle", "chaincode", "checkcommitreadiness",
			"-n", ci.chaincodeName,
			"-v", util.Vtoa(ci.version),
			"--sequence", stoa(ci.sequence),
			"-C", ci.channel,
			"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
			"--tls", "--cafile", "$ORDERER_CA",
		)

		availableCliPod string
	)

	// Iterate over given organization and peer pairs and perform chaincode installation
	for org, peers := range ci.orgpeers {
		for _, peer := range peers {
			var (
				peerPodName    = fmt.Sprintf("%s.%s.org", peer, org)
				cliPodName     = fmt.Sprintf("cli.%s.%s.org", peer, org)
				packageTarGzip = fmt.Sprintf("%s.%s.%s.tar.gz", ci.chaincodeName, peer, org)
				packageBuffer  bytes.Buffer
			)

			ci.logger.Infof("Going to install chaincode on '%s' peer of '%s' organization:", peer, org)

			// Waiting for 'org.peer' pod readiness:
			if ok, err := kube.WaitForPodReady(
				ctx,
				&peerPodName,
				fmt.Sprintf("fabnctl/app=%s.%s.org", peer, org), shared.Namespace,
			); err != nil {
				return err
			} else if !ok {
				return nil
			}

			// Waiting for 'org.peer.cli' pod readiness:
			if ok, err := kube.WaitForPodReady(
				ctx,
				&cliPodName,
				fmt.Sprintf("fabnctl/app=cli.%s.%s.org", peer, org),
				shared.Namespace,
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

			// Packaging chaincode into tar.gz archive:
			if err := ci.logger.Stream(func() error {
				if err := ci.packageExternalChaincodeInTarGzip(org, peer, &packageBuffer); err != nil {
					return fmt.Errorf("failed to package chaincode in '%s' archive: %w", packageTarGzip, err)
				}
				return nil
			}, fmt.Sprintf("Packaging chaincode into '%s' archive", packageTarGzip),
				fmt.Sprintf("Chaincode has been packaged into '%s' archive", packageTarGzip),
			); err != nil {
				return nil
			}

			// Copping chaincode package to cli pod:
			if err := ci.logger.Stream(func() error {
				if err := kube.CopyToPod(ctx, cliPodName, shared.Namespace, &packageBuffer, packageTarGzip); err != nil {
					return err
				}
				return nil
			}, fmt.Sprintf("Sending chaincode package to '%s' pod", cliPodName),
				fmt.Sprintf("Chaincode package has been sent to '%s' pod", cliPodName),
			); err != nil {
				return nil
			}

			// Installing chaincode package:
			if err := ci.logger.Stream(func() error {
				var err error
				if _, stderr, err = kube.ExecCommandInPod(ctx, cliPodName, shared.Namespace,
					"peer", "lifecycle", "chaincode", "install", packageTarGzip,
				); err != nil {
					if errors.Is(err, term.ErrRemoteCmdFailed) {
						return fmt.Errorf("Failed to install chaincode package: %w", err)
					}

					return fmt.Errorf("Failed to execute command on '%s' pod: %w", cliPodName, err)
				}

				return nil
			}, "Installing chaincode package", "Chaincode package has been installed"); err != nil {
				return ci.logger.WrapWithStderrViewPrompt(err, stderr, false)
			}

			packageID = parseInstalledPackageID(stderr)

			fmt.Printf("%s Chaincode package identifier: %s\n", viper.GetString("cli.info_emoji"), packageID)

			// Preparing additional values for chart installation:
			var (
				values    = make(map[string]interface{})
				chartSpec = &helmclient.ChartSpec{
					ReleaseName: fmt.Sprintf("%s-cc-%s-%s", ci.chaincodeName, peer, org),
					ChartName:   path.Join(shared.ChartsPath, "chaincode"),
					Namespace:   shared.Namespace,
					Wait:        true,
				}
			)

			values["image"] = map[string]interface{}{
				"repository": ci.imageName,
			}

			values["peer"] = peer
			values["org"] = org
			values["chaincode"] = ci.chaincodeName
			values["ccid"] = packageID

			valuesYaml, err := yaml.Marshal(values)
			if err != nil {
				return fmt.Errorf("failed to encode additional values: %w", err)
			}

			chartSpec.ValuesYaml = string(valuesYaml)

			// Installing orderer helm chart:
			helmCtx, cancel := context.WithTimeout(ctx, viper.GetDuration("helm.install_timeout"))

			if err = ci.logger.Stream(func() error {
				defer cancel()
				if err = helm.Client.InstallOrUpgradeChart(helmCtx, chartSpec); err != nil {
					return fmt.Errorf("failed to install chaincode helm chart: %w", err)
				}
				return nil
			}, "Installing chaincode chart",
				fmt.Sprintf("Chart 'chaincode/%s' installed successfully", chartSpec.ReleaseName),
			); err != nil {
				return nil
			}

			// Checking whether the chaincode was already approved by organization:
			if stdout, stderr, err = kube.ExecShellInPod(ctx,
				cliPodName, shared.Namespace,
				checkCommitReadinessCmd,
			); err != nil {
				if errors.Is(err, term.ErrRemoteCmdFailed) {
					return ci.logger.WrapWithStderrViewPrompt(
						fmt.Errorf("Failed to check chaincode approval by '%s' organization: %w", org, err),
						stderr, true,
					)
				}

				return fmt.Errorf("Failed to execute command on '%s' pod: %w", cliPodName, err)
			}

			var approveCmd = kube.FormCommand(
				"peer", "lifecycle", "chaincode", "approveformyorg",
				"-n", ci.chaincodeName,
				"-v", util.Vtoa(ci.version),
				"--sequence", stoa(ci.sequence),
				"--package-id", packageID,
				"--init-required=false",
				"-C", ci.channel,
				"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
				"--tls", "--cafile", "$ORDERER_CA",
			)

			// Approving chaincode if needed:
			if !checkChaincodeApprovalByOrg(stdout, org) {
				if err = ci.logger.Stream(func() (err error) {
					if _, stderr, err = kube.ExecShellInPod(ctx,
						cliPodName, shared.Namespace,
						approveCmd,
					); err != nil {
						if errors.Is(err, term.ErrRemoteCmdFailed) {
							return fmt.Errorf("Failed to approve chaincode for '%s' organization: %w", org, err)
						}

						return fmt.Errorf("failed to execute command on '%s' pod: %w", cliPodName, err)
					}

					return nil
				}, "Approving chaincode",
					fmt.Sprintf("Chaincode has been approved for '%s' organization", org),
				); err != nil {
					return ci.logger.WrapWithStderrViewPrompt(err, stderr, false)
				}
			} else {
				ci.logger.Infof("Chaincode is already approved by '%s' organization", org)
			}

			availableCliPod = cliPodName
		}
	}

	ci.logger.NewLine()

	// Verifying commit readiness,
	// by checking that all organizations on channel approved chaincode:
	if stdout, stderr, err := kube.ExecShellInPod(ctx,
		availableCliPod, shared.Namespace,
		checkCommitReadinessCmd,
	); err != nil {
		if errors.Is(err, term.ErrRemoteCmdFailed) {
			return ci.logger.WrapWithStderrViewPrompt(
				fmt.Errorf("failed to check chaincode commit readiness: %w", err),
				stderr, true,
			)
		}

		return fmt.Errorf("failed to execute command on '%s' pod: %w", availableCliPod, err)
	} else if ready, notApprovedBy := checkChaincodeCommitReadiness(stdout); !ready {
		return fmt.Errorf(
			"chaincode isn't ready to be commited, some organizations on '%s' channel haven't approved it yet: %s",
			ci.channel, strings.Join(notApprovedBy, ", "),
		)
	} else {
		ci.logger.Okf(
			"%s Chaincode has been approved by all organizations on '%s' channel, it's ready to be committed",
			ci.channel,
		)
	}

	var commitCmd = kube.FormCommand(
		"peer", "lifecycle", "chaincode", "commit",
		"-n", ci.chaincodeName,
		"-v", util.Vtoa(ci.version),
		"--sequence", stoa(ci.sequence),
		"--init-required=false",
		"-C", ci.channel,
		"-o", fmt.Sprintf("%s.%s:443", viper.GetString("fabric.orderer_hostname_name"), shared.Domain),
		"--tls", "--cafile", "$ORDERER_CA",
	)

	// Committing chaincode on peers of all given organizations:
	for org, peer := range orgPeers {
		var (
			orgHost              = fmt.Sprintf("%s.org.%s", org, shared.Domain)
			peerHost             = fmt.Sprintf("%s.%s", peer, orgHost)
			cryptoConfigPathBase = "/opt/gopath/src/github.com/hyperledger/fabric/peer/crypto-config"
			commitCmdEnding      = kube.FormCommand(
				"--peerAddresses", fmt.Sprintf("%s:443", peerHost),
				"--tlsRootCertFiles", path.Join(
					cryptoConfigPathBase,
					"peerOrganizations", orgHost,
					"peers", peerHost,
					"tls", "ca.crt",
				),
			)
		)

		commitCmd = kube.FormCommand(commitCmd, commitCmdEnding)
	}

	ci.logger.NewLine()

	var stderr io.Reader
	if err := ci.logger.Stream(func() (err error) {
		if _, stderr, err = kube.ExecShellInPod(ctx,
			availableCliPod, shared.Namespace,
			commitCmd,
		); err != nil {
			if errors.Is(err, term.ErrRemoteCmdFailed) {
				return errors.Wrapf(err,
					"Failed to commit chaincode",
				)
			}

			return fmt.Errorf("Failed to execute command on '%s' pod: %w", availableCliPod, err)
		}

		return nil
	}, "Committing chaincode on organization peers",
		"Chaincode has been committed on all organization peers",
	); err != nil {
		return ci.logger.WrapWithStderrViewPrompt(err, stderr, false)
	}

	ci.logger.Successf("Chaincode '%s' v%.1f successfully deployed!", ci.chaincodeName, ci.version)

	return nil
}

func (ci *ChaincodeInstaller) packageExternalChaincodeInTarGzip(org, peer string, writer io.Writer) error {
	var (
		codeBuffer bytes.Buffer
		mdBuffer   bytes.Buffer
		connBuffer bytes.Buffer

		codeGzip = gzip.NewWriter(&codeBuffer)
		codeTar  = tar.NewWriter(codeGzip)

		packageGzip = gzip.NewWriter(writer)
		packageTar  = tar.NewWriter(packageGzip)

		metadata = model.ChaincodeMetadata{
			Type:  "external",
			Label: ci.chaincodeName,
		}
		connection = model.ChaincodeConnection{
			Address:     fmt.Sprintf("%s-chaincode-%s-%s:7052", ci.chaincodeName, peer, org),
			DialTimeout: "10s",
		}
	)

	defer func() {
		if err := packageGzip.Close(); err != nil {
			ci.logger.Error(err, "failed to close package gzip writer")
		}
	}()

	defer func() {
		if err := codeTar.Close(); err != nil {
			ci.logger.Error(err, "failed to close code tar writer")
		}
	}()

	if err := json.NewEncoder(&connBuffer).Encode(connection); err != nil {
		return fmt.Errorf("failed to encode to 'connection.json': %w", err)
	}

	if err := util.WriteBytesToTar("connection.json", &connBuffer, codeTar); err != nil {
		return fmt.Errorf("failed to write 'connection.json' into 'code.tar.gz' archive: %w", err)
	}

	if ci.withSource {
		indexesPath := path.Join(ci.sourcePathAbs, "META-INF", "statedb", "couchdb", "indexes")
		if indexes, err := ioutil.ReadDir(indexesPath); err == nil {
			for _, index := range indexes {
				indexBytes, err := ioutil.ReadFile(path.Join(indexesPath, index.Name()))
				if err != nil {
					continue
				}

				metaIndexPath := path.Join("META-INF", "statedb", "couchdb", "indexes", index.Name())
				if err = util.WriteBytesToTar(metaIndexPath, bytes.NewBuffer(indexBytes), codeTar); err != nil {
					return fmt.Errorf("failed to write '%s' into code tar archive: %w", metaIndexPath, err)
				}
			}
		}
	}

	if err := codeTar.Close(); err != nil {
		ci.logger.Error(err, "failed to close code tar writer")
	}

	if err := codeGzip.Close(); err != nil {
		ci.logger.Error(err, "failed to close code gzip writer")
	}

	if err := util.WriteBytesToTar("code.tar.gz", &codeBuffer, packageTar); err != nil {
		return fmt.Errorf("failed to write 'code.tar.gz' into package tar archive: %w", err)
	}

	if err := json.NewEncoder(&mdBuffer).Encode(metadata); err != nil {
		return fmt.Errorf("failed to encode to 'metadata.json': %w", err)
	}

	if err := util.WriteBytesToTar("metadata.json", &mdBuffer, packageTar); err != nil {
		return fmt.Errorf("failed to write 'metadata.json' into package tar archive: %w", err)
	}

	return nil
}

func (ci *ChaincodeInstaller) checkChaincodeCommitStatus(ctx context.Context) (bool, float64, int, error) {
	var (
		availableCliPod string
		buffer          bytes.Buffer
	)

	if pods, err := kube.Client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: "fabnctl/cid=org-peer-cli",
	}); err != nil {
		return false, 0, 0, fmt.Errorf("failed to find available cli pod for chaincode commit status check: %w", err)
	} else if pods == nil || pods.Size() == 0 {
		return false, 0, 0, fmt.Errorf("failed to find available cli pod for chaincode commit status check")
	} else {
		availableCliPod = pods.Items[0].Name
	}

	// Checking whether the chaincode was already committed:
	stdout, stderr, err := kube.ExecCommandInPod(
		ctx,
		availableCliPod, shared.Namespace,
		"peer", "lifecycle", "chaincode", "querycommitted", "-C", ci.channel,
	)

	if err != nil {
		if errors.Is(err, term.ErrRemoteCmdFailed) {
			return false, 0, 0, ci.logger.WrapWithStderrViewPrompt(
				fmt.Errorf("failed to check сommit status for '%s' chaincode: %w", ci.chaincodeName, err),
				stderr, true,
			)
		}

		return false, 0, 0, fmt.Errorf("Failed to execute command on '%s' pod: %w", availableCliPod, err)
	}

	if n, err := io.Copy(&buffer, stdout); err != nil || n == 0 {
		return false, 0, 0, nil
	}

	match := regexp.MustCompile(fmt.Sprintf("Name: %s, Version: (\\d*.\\d*), Sequence: (\\d*)", ci.chaincodeName)).
		FindStringSubmatch(buffer.String())

	if len(match) < 3 {
		return false, 0, 0, nil
	}

	return true, util.Atov(match[1]), atos(match[2]), nil
}

func parseInstalledPackageID(reader io.Reader) string {
	res := regexp.MustCompile("Chaincode code package identifier:(.+?)$").
		FindStringSubmatch(term.GetLastLine(reader))
	if len(res) == 2 {
		return strings.TrimSpace(res[1])
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

func stoa(sequence int) string {
	return fmt.Sprintf("%d", sequence)
}

func atos(str string) int {
	sequence, err := strconv.Atoi(str)
	if err != nil {
		return 1
	}

	return sequence
}

