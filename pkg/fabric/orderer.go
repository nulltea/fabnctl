package fabric

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"

	helmclient "github.com/mittwald/go-helm-client"
	"github.com/spf13/viper"
	"github.com/timoth-y/fabnctl/pkg/helm"
	"github.com/timoth-y/fabnctl/pkg/kube"
	"github.com/timoth-y/fabnctl/pkg/term"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type Orderer struct {
	hostname string
	*sharedArgs
}

func NewOrderer(name string, options ...SharedOption) (*Orderer, error) {
	var args = &sharedArgs{
		arch: "amd64",
		kubeNamespace: "network",
		logger: term.NewLogger(),
		chartsPath: "./network-config.yaml",
	}

	for i := range options {
		options[i](args)
	}

	if len(args.initErrors) > 0 {
		return nil, args.Error()
	}

	return &Orderer{
		hostname: name,
		sharedArgs: args,
	}, nil
}

func (o *Orderer) Install(ctx context.Context) error {
	var (
		tlsDir   = path.Join(
			fmt.Sprintf(".crypto-config.%s", o.domain),
			"ordererOrganizations", o.domain,
			"orderers", fmt.Sprintf("%s.%s", o.hostname, o.domain),
			"tls",
		)
		pkPath        = path.Join(tlsDir, "server.key")
		certPath      = path.Join(tlsDir, "server.crt")
		caPath        = path.Join(tlsDir, "ca.crt")
		tlsSecretName = fmt.Sprintf("%s.%s-tls", o.hostname, o.domain)
		caSecretName  = fmt.Sprintf("%s.%s-ca", o.hostname, o.domain)
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

	// Create or update orderer transport TLS secret:
	if _, err = kube.SecretAdapter(kube.Client.CoreV1().Secrets(o.kubeNamespace)).CreateOrUpdate(ctx, corev1.Secret{
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSPrivateKeyKey: pkPayload,
			corev1.TLSCertKey:       certPayload,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: o.kubeNamespace,
			Labels: map[string]string{
				"fabnctl/cid":    "orderer.tls.secret",
				"fabnctl/domain": o.domain,
				"fabnctl/host":   o.hostname,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to create %s secret: %w", tlsSecretName, err)
	}

	o.logger.Successf("Secret '%s' successfully created", tlsSecretName)

	// Create or update orderer transport CA secret:
	if _, err = kube.SecretAdapter(kube.Client.CoreV1().Secrets(o.kubeNamespace)).CreateOrUpdate(ctx, corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt": caPayload,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      caSecretName,
			Namespace: o.kubeNamespace,
			Labels: map[string]string{
				"fabnctl/cid":    "orderer.ca.secret",
				"fabnctl/domain": o.domain,
				"fabnctl/host":   o.hostname,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to create %s secret: %w", caSecretName, err)
	}

	o.logger.Successf("Secret '%s' successfully created", caSecretName)

	// Preparing additional values for chart installation:
	var (
		values = make(map[string]interface{})
		chartSpec = &helmclient.ChartSpec{
			ReleaseName: "orderer",
			ChartName:   path.Join(o.chartsPath, "orderer"),
			Namespace:   o.kubeNamespace,
			Wait:        true,
		}
	)

	if o.arch == "arm64" {
		armValues, err := helm.ValuesFromFile(path.Join(o.chartsPath, "orderer", "values.arm64.yaml"))
		if err != nil {
			return err
		}
		values = armValues
	}

	values["domain"] = o.domain

	valuesYaml, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("failed to encode additional values: %w", err)
	}

	chartSpec.ValuesYaml = string(valuesYaml)

	// Installing orderer helm chart:
	ctx, cancel := context.WithTimeout(ctx, viper.GetDuration("helm.install_timeout"))
	defer cancel()

	if err = o.logger.Stream(func() error {
		if err = helm.Client.InstallOrUpgradeChart(ctx, chartSpec); err != nil {
			return fmt.Errorf("failed to install orderer helm chart: %w", err)
		}
		return nil
	}, "Installing orderer chart", "Chart 'orderer/orderer' installed successfully"); err != nil {
		return nil
	}

	o.logger.Successf("Orderer service successfully deployed on %s.%s!", o.hostname, o.domain)

	return nil
}
