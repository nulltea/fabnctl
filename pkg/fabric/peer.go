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

type Peer struct {
	org string
	peer string
	*peerArgs
}

func NewPeer(org, name string, options ...PeerOption) (*Peer, error) {
	var args = &peerArgs{
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

	return &Peer{
		org:      org,
		peer:     name,
		peerArgs: args,
	}, nil
}

func (p *Peer) Install(ctx context.Context) error {
	var (
		tlsDir = path.Join(
			fmt.Sprintf(".crypto-config.%s", p.domain),
			"peerOrganizations", fmt.Sprintf("%s.org.%s", p.org, p.domain),
			"peers", fmt.Sprintf("%s.%s.org.%s", p.peer, p.org, p.domain),
			"tls",
		)
		pkPath        = path.Join(tlsDir, "server.key")
		certPath      = path.Join(tlsDir, "server.crt")
		caPath        = path.Join(tlsDir, "ca.crt")
		tlsSecretName = fmt.Sprintf("%s.%s.org.%s-tls", p.peer, p.org, p.domain)
		caSecretName  = fmt.Sprintf("%s.%s.org.%s-ca", p.peer, p.org, p.domain)
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
	if _, err = kube.SecretAdapter(kube.Client.CoreV1().Secrets(p.kubeNamespace)).CreateOrUpdate(ctx, corev1.Secret{
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSPrivateKeyKey: pkPayload,
			corev1.TLSCertKey:       certPayload,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      tlsSecretName,
			Namespace: p.kubeNamespace,
			Labels: map[string]string{
				"fabnctl/cid":    "peer.tls.secret",
				"fabnctl/domain": p.domain,
				"fabnctl/host":   fmt.Sprintf("%s.%s.org", p.peer, p.org),
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to create %s secret: %w", tlsSecretName, err)
	}

	p.logger.Successf("Secret '%s' successfully created\n", tlsSecretName)

	// Create or update peer transport CA secret:
	if _, err = kube.SecretAdapter(kube.Client.CoreV1().Secrets(p.kubeNamespace)).CreateOrUpdate(ctx, corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ca.crt": caPayload,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      caSecretName,
			Namespace: p.kubeNamespace,
			Labels: map[string]string{
				"fabnctl/cid":    "peer.ca.secret",
				"fabnctl/domain": p.domain,
				"fabnctl/host":   fmt.Sprintf("%s.%s.org", p.peer, p.org),
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to create %s secret: %w", caSecretName, err)
	}

	p.logger.Successf("Secret '%s' successfully created", caSecretName)

	// Preparing additional values for chart installation:
	var (
		values = make(map[string]interface{})
		chartSpec = &helmclient.ChartSpec{
			ReleaseName: fmt.Sprintf("%s-%s", p.peer, p.org),
			ChartName:   path.Join(p.chartsPath, "peer"),
			Namespace:   p.kubeNamespace,
			Wait:        true,
		}
	)

	if p.arch == "arm64" {
		armValues, err := helm.ValuesFromFile(path.Join(p.chartsPath, "peer", "values.arm64.yaml"))
		if err != nil {
			return err
		}
		values = armValues
	}

	values["domain"] = p.domain
	if caValues, ok := values["ca"].(map[string]interface{}); ok {
		caValues["enabled"] = p.installCA
	} else {
		values["ca"] = map[string]interface{}{
			"enabled": p.installCA,
		}
	}

	if configValues, ok := values["config"].(map[string]interface{}); ok {
		configValues["mspID"] = p.org
		configValues["domain"] = p.domain
		configValues["hostname"] = fmt.Sprintf("%s.org", p.org)
	} else {
		values["config"] = map[string]interface{}{
			"mspID":    p.org,
			"domain":   p.domain,
			"hostname": fmt.Sprintf("%s.org", p.org),
		}
	}

	if ordererValues, ok := values["orderer"].(map[string]interface{}); ok {
		ordererValues["domain"] = p.domain
	} else {
		values["orderer"] = map[string]interface{}{
			"domain": p.domain,
		}
	}

	valuesYaml, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("failed to encode additional values: %w", err)
	}

	chartSpec.ValuesYaml = string(valuesYaml)

	// Installing orderer helm chart:
	ctx, cancel := context.WithTimeout(ctx, viper.GetDuration("helm.install_timeout"))
	defer cancel()

	if err = p.logger.Stream(func() error {
		if err = helm.Client.InstallOrUpgradeChart(ctx, chartSpec); err != nil {
			return fmt.Errorf("failed to install peer helm chart: %w", err)
		}
		return nil
	}, fmt.Sprintf("Installing 'peer/%s-%s' chart", p.peer, p.org),
		fmt.Sprintf("Chart 'peer/%s' installed successfully", chartSpec.ReleaseName),
	); err != nil {
		return nil
	}

	p.logger.Successf("Peer successfully deployed on %s.%s.org.%s!", p.peer, p.org, p.domain)

	return nil
}
