package openbaomanager

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	managedByLabelKey   = "app.kubernetes.io/managed-by"
	managedByLabelValue = "mogenius-k8s-manager"

	rootTokenDataKey  = "root_token"
	unsealKeysDataKey = "unseal_keys"
)

// readKeysSecret returns the stored root token and unseal keys. A missing
// secret yields empty values and no error, so the caller can distinguish
// "not yet initialized" from a real failure.
func (m *openBaoManager) readKeysSecret(ctx context.Context) (rootToken string, unsealKeys []string, err error) {
	ns := m.config.Get("MO_OWN_NAMESPACE")
	secret, err := m.clientProvider.K8sClientSet().CoreV1().Secrets(ns).Get(ctx, keysSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("get secret %q: %w", keysSecretName, err)
	}
	rootToken = string(secret.Data[rootTokenDataKey])
	if raw, ok := secret.Data[unsealKeysDataKey]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &unsealKeys); err != nil {
			return "", nil, fmt.Errorf("parse unseal keys: %w", err)
		}
	}
	return rootToken, unsealKeys, nil
}

// writeKeysSecret creates or updates the keys secret with the given root token
// and unseal keys. It is only called right after a fresh init, so overwriting
// stale material is intentional. The parent PlatformConfig is set as owner so
// the secret is garbage-collected when OpenBao is removed.
func (m *openBaoManager) writeKeysSecret(ctx context.Context, rootToken string, unsealKeys []string, ownerRef metav1.OwnerReference) error {
	ns := m.config.Get("MO_OWN_NAMESPACE")
	secrets := m.clientProvider.K8sClientSet().CoreV1().Secrets(ns)

	keysJSON, err := json.Marshal(unsealKeys)
	if err != nil {
		return fmt.Errorf("marshal unseal keys: %w", err)
	}
	data := map[string][]byte{
		rootTokenDataKey:  []byte(rootToken),
		unsealKeysDataKey: keysJSON,
	}

	ownerRefs := ownerReferences(ownerRef)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            keysSecretName,
			Namespace:       ns,
			Labels:          map[string]string{managedByLabelKey: managedByLabelValue},
			OwnerReferences: ownerRefs,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	_, err = secrets.Create(ctx, secret, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create secret %q: %w", keysSecretName, err)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := secrets.Get(ctx, keysSecretName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Labels == nil {
			current.Labels = map[string]string{}
		}
		current.Labels[managedByLabelKey] = managedByLabelValue
		if ownerRefs != nil {
			current.OwnerReferences = ownerRefs
		}
		current.Data = data
		_, err = secrets.Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
}

// ownerReferences returns the owner reference slice for objects the manager
// creates, or nil when the parent identity is not yet known (empty UID).
func ownerReferences(ownerRef metav1.OwnerReference) []metav1.OwnerReference {
	if ownerRef.UID == "" {
		return nil
	}
	return []metav1.OwnerReference{ownerRef}
}
