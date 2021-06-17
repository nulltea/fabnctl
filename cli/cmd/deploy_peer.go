package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/mittwald/go-helm-client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/chainmetric-network/cli/shared"
	"github.com/timoth-y/chainmetric-network/cli/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// peerCmd represents the peer command
var peerCmd = &cobra.Command{
	Use:   "peer",
	Short: "Performs deployment sequence of the Fabric peer",
	RunE: deployPeer,
}

func init() {
	deployCmd.AddCommand(peerCmd)

	peerCmd.Flags().StringP("org", "o", "", "Organization owning peer (required)")
	peerCmd.Flags().StringP("peer", "p", "peer0", "Peer hostname. Default is: peer0")
	peerCmd.Flags().Bool("withCA", true,
		"Deploy CA service along with peer. Default is true",
	)
}

func deployPeer(cmd *cobra.Command, args []string) error {
	var (
		err error
		org string
		peer string
		withCA bool
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

	if withCA, err = cmd.Flags().GetBool("withCA"); err != nil {
		return errors.Wrap(err, "failed to parse 'withCA' parameter")
	}

	var (
		tlsDir = path.Join(
			fmt.Sprintf(".crypto-config.%s", domain),
			"peerOrganizations", fmt.Sprintf("%s.org.%s", org, domain),
			"peers", fmt.Sprintf("%s.%s.org.%s", peer, org, domain),
			"tls",
		)
		pkPath        = path.Join(tlsDir, "server.key")
		certPath      = path.Join(tlsDir, "server.crt")
		caPath        = path.Join(tlsDir, "ca.crt")
		tlsSecretName = fmt.Sprintf("%s.%s.org.%s-tls", peer, org, domain)
		caSecretName  = fmt.Sprintf("%s.%s.org.%s-ca", peer, org, domain)
	)

	// Retrieve orderer transport TLS private key:
	pkPayload, err := ioutil.ReadFile(pkPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read private key from path: %s", pkPath)
	}

	// Retrieve orderer transport TLS cert:
	certPayload, err := ioutil.ReadFile(certPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read certificate identity from path: %s", certPath)
	}

	// Retrieve orderer transport CA cert:
	caPayload, err := ioutil.ReadFile(caPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read certificate CA from path: %s", caPath)
	}

	// Create or update peer transport TLS secret:
	if _, err = util.SecretAdapter(shared.K8s.CoreV1().Secrets(namespace)).CreateOrUpdate(cmd.Context(), corev1.Secret{
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSPrivateKeyKey: pkPayload,
			corev1.TLSCertKey:       certPayload,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"fabnetd/cid": "peer.tls.secret",
				"fabnetd/domain": domain,
				"fabnetd/host": fmt.Sprintf("%s.%s.org", peer, org),
			},
		},
	}); err != nil {
		return errors.Wrapf(err, "failed to create %s secret", tlsSecretName)
	}

	cmd.Printf("✅ Secret '%s' successfully created\n", tlsSecretName)

	// Create or update peer transport CA secret:
	if _, err = util.SecretAdapter(shared.K8s.CoreV1().Secrets(namespace)).CreateOrUpdate(cmd.Context(), corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt": caPayload,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: caSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"fabnetd/cid": "peer.ca.secret",
				"fabnetd/domain": domain,
				"fabnetd/host": fmt.Sprintf("%s.%s.org", peer, org),
			},
		},
	}); err != nil {
		return errors.Wrapf(err, "failed to create %s secret", caSecretName)
	}

	cmd.Printf("✅ Secret '%s' successfully created\n", caSecretName)


	var (
		values = make(map[string]interface{})
		chartSpec = &helmclient.ChartSpec{
			ReleaseName: fmt.Sprintf("%s-%s", peer, org),
			ChartName: path.Join(chartsPath, "peer"),
			Namespace: namespace,
			Wait: true,
		}
	)

	if targetArch == "arm64" {
		armValues, err := util.ValuesFromFile(path.Join(chartsPath, "peer", "values.arm64.yaml"))
		if err != nil {
			return err
		}
		values = armValues
	}

	values["domain"] = domain
	if caValues, ok := values["ca"].(map[string]interface{}); ok {
		caValues["enabled"] = withCA
	} else {
		values["ca"] = map[string]interface{}{
			"enabled": withCA,
		}
	}

	if configValues, ok := values["config"].(map[string]interface{}); ok {
		configValues["mspID"] = org
		configValues["domain"] = domain
		configValues["hostname"] = fmt.Sprintf("%s.org", org)
	} else {
		values["config"] = map[string]interface{}{
			"mspID": org,
			"domain": domain,
			"hostname": fmt.Sprintf("%s.org", org),
		}
	}

	if ordererValues, ok := values["orderer"].(map[string]interface{}); ok {
		ordererValues["domain"] = domain
	} else {
		values["orderer"] = map[string]interface{}{
			"domain": domain,
		}
	}

	valuesYaml, err := yaml.Marshal(values)
	if err != nil {
		return errors.Wrap(err, "failed to encode additional values")
	}

	chartSpec.ValuesYaml = string(valuesYaml)

	ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("helm.install_timeout"))
	defer cancel()

	shared.ILogger.Start()
	if err = shared.Helm.InstallOrUpgradeChart(ctx, chartSpec); err != nil {
		return errors.Wrap(err, "failed to install peer helm chart")
	}
	shared.ILogger.PersistWith(shared.ILogPrefixes[shared.ILogSuccess],
		fmt.Sprintf(" Chart 'peer/%s' installed successfully", fmt.Sprintf("%s-%s", peer, org)),
	)
	shared.ILogger.Stop()

	cmd.Printf("✅ Peer successfully deployed on %s.%s.org.%s!\n", peer, org, domain)
	return nil
}
