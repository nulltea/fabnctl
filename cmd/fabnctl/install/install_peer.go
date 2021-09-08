package install

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/mittwald/go-helm-client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/cmd/fabnctl/shared"
	util2 "github.com/timoth-y/fabnctl/pkg/helm"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/terminal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// peerCmd represents the peer command
var peerCmd = &cobra.Command{
	Use:   "peer",
	Short: "Performs deployment sequence of the Fabric peer",
	Long: `Performs deployment sequence of the Fabric peer

Examples:
  # Deploy peer:
  fabnctl deploy peer -d example.com -o org1 -p peer0

  # Deploy peer but skip CA service installation:
  fabnctl deploy peer -d example.com -o org1 -p peer0 --withCA=false`,

	RunE: shared.WithHandleErrors(deployPeer),
}

func init() {
	cmd.AddCommand(peerCmd)

	peerCmd.Flags().StringP("org", "o", "", "Organization owning peer (required)")
	peerCmd.Flags().StringP("peer", "p", "peer0", "Peer hostname")
	peerCmd.Flags().Bool("withCA", true,
		"Deploy CA service along with peer",
	)

	peerCmd.MarkFlagRequired("org")
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
		return fmt.Errorf("%w: failed to parse required parameter 'org' (organization): %s", err, shared.ErrInvalidArgs)
	}

	if peer, err = cmd.Flags().GetString("peer"); err != nil {
		return fmt.Errorf("%w: failed to parse 'peer' parameter: %s", err, shared.ErrInvalidArgs)
	}

	if withCA, err = cmd.Flags().GetBool("withCA"); err != nil {
		return fmt.Errorf("%w: failed to parse 'withCA' parameter: %s", err, shared.ErrInvalidArgs)
	}

	var (
		tlsDir = path.Join(
			fmt.Sprintf(".crypto-config.%s", shared.Domain),
			"peerOrganizations", fmt.Sprintf("%s.org.%s", org, shared.Domain),
			"peers", fmt.Sprintf("%s.%s.org.%s", peer, org, shared.Domain),
			"tls",
		)
		pkPath        = path.Join(tlsDir, "server.key")
		certPath      = path.Join(tlsDir, "server.crt")
		caPath        = path.Join(tlsDir, "ca.crt")
		tlsSecretName = fmt.Sprintf("%s.%s.org.%s-tls", peer, org, shared.Domain)
		caSecretName  = fmt.Sprintf("%s.%s.org.%s-ca", peer, org, shared.Domain)
	)

	// Retrieve orderer transport TLS private key:
	pkPayload, err := ioutil.ReadFile(pkPath)
	if err != nil {
		return fmt.Errorf("failed to read private key from path: %s: %w", pkPath, err)
	}

	// Retrieve orderer transport TLS cert:
	certPayload, err := ioutil.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate identity from path: %s: %w", certPath, err)
	}

	// Retrieve orderer transport CA cert:
	caPayload, err := ioutil.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate CA from path: %s: %w", caPath, err)
	}

	// Create or update peer transport TLS secret:
	if _, err = kube.SecretAdapter(kube.Client.CoreV1().Secrets(shared.Namespace)).CreateOrUpdate(cmd.Context(), corev1.Secret{
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSPrivateKeyKey: pkPayload,
			corev1.TLSCertKey:       certPayload,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: shared.Namespace,
			Labels: map[string]string{
				"fabnctl/cid":    "peer.tls.secret",
				"fabnctl/domain": shared.Domain,
				"fabnctl/host":   fmt.Sprintf("%s.%s.org", peer, org),
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to create %s secret: %w", tlsSecretName, err)
	}

	cmd.Printf("%s Secret '%s' successfully created\n",
		viper.GetString("cli.success_emoji"), tlsSecretName,
	)

	// Create or update peer transport CA secret:
	if _, err = kube.SecretAdapter(kube.Client.CoreV1().Secrets(shared.Namespace)).CreateOrUpdate(cmd.Context(), corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt": caPayload,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      caSecretName,
			Namespace: shared.Namespace,
			Labels: map[string]string{
				"fabnctl/cid":    "peer.ca.secret",
				"fabnctl/domain": shared.Domain,
				"fabnctl/host":   fmt.Sprintf("%s.%s.org", peer, org),
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to create %s secret: %w", caSecretName, err)
	}

	cmd.Printf("%s Secret '%s' successfully created\n",
		viper.GetString("cli.success_emoji"), caSecretName,
	)

	// Preparing additional values for chart installation:
	var (
		values = make(map[string]interface{})
		chartSpec = &helmclient.ChartSpec{
			ReleaseName: fmt.Sprintf("%s-%s", peer, org),
			ChartName:   path.Join(shared.ChartsPath, "peer"),
			Namespace:   shared.Namespace,
			Wait:        true,
		}
	)

	if shared.TargetArch == "arm64" {
		armValues, err := util2.ValuesFromFile(path.Join(shared.ChartsPath, "peer", "values.arm64.yaml"))
		if err != nil {
			return err
		}
		values = armValues
	}

	values["domain"] = shared.Domain
	if caValues, ok := values["ca"].(map[string]interface{}); ok {
		caValues["enabled"] = withCA
	} else {
		values["ca"] = map[string]interface{}{
			"enabled": withCA,
		}
	}

	if configValues, ok := values["config"].(map[string]interface{}); ok {
		configValues["mspID"] = org
		configValues["domain"] = shared.Domain
		configValues["hostname"] = fmt.Sprintf("%s.org", org)
	} else {
		values["config"] = map[string]interface{}{
			"mspID":    org,
			"domain":   shared.Domain,
			"hostname": fmt.Sprintf("%s.org", org),
		}
	}

	if ordererValues, ok := values["orderer"].(map[string]interface{}); ok {
		ordererValues["domain"] = shared.Domain
	} else {
		values["orderer"] = map[string]interface{}{
			"domain": shared.Domain,
		}
	}

	valuesYaml, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("failed to encode additional values: %w", err)
	}

	chartSpec.ValuesYaml = string(valuesYaml)

	// Installing orderer helm chart:
	ctx, cancel := context.WithTimeout(cmd.Context(), viper.GetDuration("helm.install_timeout"))
	defer cancel()

	if err = terminal.DecorateWithInteractiveLog(func() error {
		if err = util2.Client.InstallOrUpgradeChart(ctx, chartSpec); err != nil {
			return fmt.Errorf("failed to install peer helm chart: %w", err)
		}
		return nil
	}, fmt.Sprintf("Installing 'peer/%s-%s' chart", peer, org),
		fmt.Sprintf("Chart 'peer/%s' installed successfully", chartSpec.ReleaseName),
	); err != nil {
		return nil
	}

	cmd.Printf("🎉 Peer successfully deployed on %s.%s.org.%s!\n", peer, org, shared.Domain)
	return nil
}
