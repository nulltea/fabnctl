package cmd

import (
	"fmt"
	"io/ioutil"

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

// ordererCmd represents the orderer command.
var ordererCmd = &cobra.Command{
	Use:   "orderer",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: deployOrderer,
}

func init() {
	deployCmd.AddCommand(ordererCmd)
}

func deployOrderer(cmd *cobra.Command, _ []string) error {
	var (
		hostname = viper.GetString("fabric.orderer_hostname_name")
		tlsDir = fmt.Sprintf(".crypto-config.%s/ordererOrganizations/%s/orderers/%s.%s/tls",
			domain, domain, hostname, domain,
		)
		tlsSecretName = fmt.Sprintf("%s.%s-tls", hostname, domain)
		caSecretName = fmt.Sprintf("%s.%s-ca", hostname, domain)
	)

	// Retrieve orderer transport TLS private key:
	pkPayload, err := ioutil.ReadFile(fmt.Sprintf("%s/server.key", tlsDir))
	if err != nil {
		return errors.Wrapf(err, "failed to read private key from path: %s/server.key", tlsDir)
	}

	// Retrieve orderer transport TLS cert:
	certPayload, err := ioutil.ReadFile(fmt.Sprintf("%s/server.crt", tlsDir))
	if err != nil {
		return errors.Wrapf(err, "failed to read certificate identity from path: %s/server.crt", tlsDir)
	}

	// Retrieve orderer transport CA cert:
	caPayload, err := ioutil.ReadFile(fmt.Sprintf("%s/ca.crt", tlsDir))
	if err != nil {
		return errors.Wrapf(err, "failed to read certificate CA from path: %s/ca.crt", tlsDir)
	}

	// Create orderer transport TLS secret:
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
				"fabnetd/cid": "orderer.tls.secret",
			},
		},
	}); err != nil {
		return errors.Wrapf(err, "failed to create %s secret", tlsSecretName)
	}

	// Create orderer transport CA secret:
	if _, err = util.SecretAdapter(shared.K8s.CoreV1().Secrets(namespace)).CreateOrUpdate(cmd.Context(), corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt": caPayload,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: caSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"fabnetd/cid": "orderer.ca.secret",
			},
		},
	}); err != nil {
		return errors.Wrapf(err, "failed to create %s secret", caSecretName)
	}

	var (
		values = make(map[string]interface{})
		chartSpec = &helmclient.ChartSpec{
			ReleaseName: "orderer",
			ChartName: fmt.Sprintf("%s/orderer", chartsPath),
			Namespace: namespace,
			Wait: true,
		}
	)

	if targetArch == "arm64" {
		armValues, err := util.ValuesFromFile(fmt.Sprintf("%s/orderer/values.arm64.yaml", chartsPath))
		if err != nil {
			return err
		}
		values = armValues
	}

	values["domain"] = domain

	valuesYaml, err := yaml.Marshal(values)
	if err != nil {
		return errors.Wrap(err, "failed to encode additional values")
	}
	chartSpec.ValuesYaml = string(valuesYaml)

	shared.Helm.InstallOrUpgradeChart(cmd.Context(), chartSpec)

	cmd.Println("Orderer service deployed successfully!")

	return nil
}
