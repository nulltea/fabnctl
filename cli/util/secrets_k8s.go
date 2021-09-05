package util

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// SecretInterface provides additional methods for dealing with Kubernetes secrets.
type SecretInterface struct {
	v1.SecretInterface
}

// SecretAdapter constructs new SecretInterface adapter instance.
func SecretAdapter(i v1.SecretInterface) *SecretInterface {
	return &SecretInterface{
		SecretInterface: i,
	}
}

// CreateOrUpdate takes the representation of a secret and either creates it or update existing one.
func (i *SecretInterface) CreateOrUpdate(ctx context.Context, secret corev1.Secret) (*corev1.Secret, error) {
	if s, err := i.Get(ctx, secret.Name, metav1.GetOptions{}); s == nil || err != nil {
		return i.Create(ctx, &secret, metav1.CreateOptions{})
	}

	return i.Update(ctx, &secret, metav1.UpdateOptions{})
}
