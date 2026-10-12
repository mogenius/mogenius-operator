package helm

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"helm.sh/helm/v4/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// ╭──────────────────────────────────────╮
// │ OCI registry credentials persistence │
// ╰──────────────────────────────────────╯
//
// registry.Client.Login stores credentials in Helm's registry config file (a
// docker config.json under MO_HELM_DATA_PATH), which lives on an emptyDir.
// After every successful login the file is mirrored into a Secret in the
// operator namespace, and restored from it on startup, so later calls without
// credentials (tag listing, upgrades) keep authenticating across restarts.
//
// Credentials are keyed by registry host, so one login grants every caller of
// the operator access to that registry.

const (
	registryCredentialsSecretName     = "mogenius-helm-registry-credentials"
	registryCredentialsManagedByKey   = "app.kubernetes.io/managed-by"
	registryCredentialsManagedByValue = "mogenius-k8s-manager"
)

// registryCredentialsMu serializes login and mirroring: concurrent logins
// through separate registry clients each rewrite the shared config file.
var registryCredentialsMu sync.Mutex

var registryCredentialsSecrets = func() (typedcorev1.SecretInterface, error) {
	restConfig, err := NewCli().RESTClientGetter().ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("build REST config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}
	return clientset.CoreV1().Secrets(config.Get("MO_OWN_NAMESPACE")), nil
}

// registryLogin logs in to an OCI registry and persists the resulting
// credentials. A failed persist is logged, not returned: the login is still
// usable until the next restart.
func registryLogin(client *registry.Client, host, username, password string) error {
	registryCredentialsMu.Lock()
	defer registryCredentialsMu.Unlock()

	if err := client.Login(host, registry.LoginOptBasicAuth(username, password)); err != nil {
		return fmt.Errorf("failed to login to OCI registry: %w", err)
	}
	if err := saveRegistryConfigToSecret(); err != nil {
		helmLogger.Warn("failed to persist OCI registry credentials", "host", host, "error", err)
	}
	return nil
}

func saveRegistryConfigToSecret() error {
	data, err := os.ReadFile(registryConfig)
	if err != nil {
		return fmt.Errorf("read registry config: %w", err)
	}
	secrets, err := registryCredentialsSecrets()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return upsertRegistryCredentialsSecret(ctx, secrets, data)
}

func upsertRegistryCredentialsSecret(ctx context.Context, secrets typedcorev1.SecretInterface, data []byte) error {
	current, err := secrets.Get(ctx, registryCredentialsSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = secrets.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:   registryCredentialsSecretName,
				Labels: map[string]string{registryCredentialsManagedByKey: registryCredentialsManagedByValue},
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{corev1.DockerConfigJsonKey: data},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create secret %q: %w", registryCredentialsSecretName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret %q: %w", registryCredentialsSecretName, err)
	}
	if current.Labels[registryCredentialsManagedByKey] != registryCredentialsManagedByValue {
		return fmt.Errorf("secret %q already exists and is not managed by mogenius, refusing to overwrite it", registryCredentialsSecretName)
	}
	current.Data = map[string][]byte{corev1.DockerConfigJsonKey: data}
	if _, err := secrets.Update(ctx, current, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update secret %q: %w", registryCredentialsSecretName, err)
	}
	return nil
}

func restoreRegistryConfigFromSecret() error {
	secrets, err := registryCredentialsSecrets()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	secret, err := secrets.Get(ctx, registryCredentialsSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret %q: %w", registryCredentialsSecretName, err)
	}
	data := secret.Data[corev1.DockerConfigJsonKey]
	if len(data) == 0 {
		return nil
	}

	registryCredentialsMu.Lock()
	defer registryCredentialsMu.Unlock()
	if err := os.WriteFile(registryConfig, data, 0600); err != nil {
		return fmt.Errorf("write registry config: %w", err)
	}
	return nil
}
