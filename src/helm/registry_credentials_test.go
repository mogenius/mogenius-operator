package helm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const testNamespace = "mogenius"

func managedRegistrySecret(data string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registryCredentialsSecretName,
			Namespace: testNamespace,
			Labels:    map[string]string{registryCredentialsManagedByKey: registryCredentialsManagedByValue},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(data)},
	}
}

func TestUpsertRegistryCredentialsSecret(t *testing.T) {
	ctx := context.Background()

	t.Run("creates a dockerconfigjson secret", func(t *testing.T) {
		secrets := fake.NewClientset().CoreV1().Secrets(testNamespace)

		require.NoError(t, upsertRegistryCredentialsSecret(ctx, secrets, []byte(`{"auths":{"ghcr.io":{}}}`)))

		secret, err := secrets.Get(ctx, registryCredentialsSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, corev1.SecretTypeDockerConfigJson, secret.Type)
		assert.Equal(t, registryCredentialsManagedByValue, secret.Labels[registryCredentialsManagedByKey])
		assert.JSONEq(t, `{"auths":{"ghcr.io":{}}}`, string(secret.Data[corev1.DockerConfigJsonKey]))
	})

	t.Run("replaces the data of a managed secret", func(t *testing.T) {
		secrets := fake.NewClientset(managedRegistrySecret(`{"auths":{"old.io":{}}}`)).CoreV1().Secrets(testNamespace)

		require.NoError(t, upsertRegistryCredentialsSecret(ctx, secrets, []byte(`{"auths":{"new.io":{}}}`)))

		secret, err := secrets.Get(ctx, registryCredentialsSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.JSONEq(t, `{"auths":{"new.io":{}}}`, string(secret.Data[corev1.DockerConfigJsonKey]))
	})

	t.Run("refuses to overwrite an unmanaged secret", func(t *testing.T) {
		unmanaged := managedRegistrySecret(`{"auths":{"theirs.io":{}}}`)
		unmanaged.Labels = nil
		secrets := fake.NewClientset(unmanaged).CoreV1().Secrets(testNamespace)

		assert.Error(t, upsertRegistryCredentialsSecret(ctx, secrets, []byte(`{"auths":{}}`)))

		secret, err := secrets.Get(ctx, registryCredentialsSecretName, metav1.GetOptions{})
		require.NoError(t, err)
		assert.JSONEq(t, `{"auths":{"theirs.io":{}}}`, string(secret.Data[corev1.DockerConfigJsonKey]))
	})
}

func TestRestoreRegistryConfigFromSecret(t *testing.T) {
	useSecrets := func(t *testing.T, objects ...*corev1.Secret) {
		clientset := fake.NewClientset()
		for _, obj := range objects {
			_, err := clientset.CoreV1().Secrets(testNamespace).Create(context.Background(), obj, metav1.CreateOptions{})
			require.NoError(t, err)
		}
		previous := registryCredentialsSecrets
		registryCredentialsSecrets = func() (typedcorev1.SecretInterface, error) {
			return clientset.CoreV1().Secrets(testNamespace), nil
		}
		t.Cleanup(func() { registryCredentialsSecrets = previous })
	}
	useRegistryConfig := func(t *testing.T) string {
		path := filepath.Join(t.TempDir(), "config.json")
		previous := registryConfig
		registryConfig = path
		t.Cleanup(func() { registryConfig = previous })
		return path
	}

	t.Run("writes the stored config to the registry config file", func(t *testing.T) {
		useSecrets(t, managedRegistrySecret(`{"auths":{"ghcr.io":{}}}`))
		path := useRegistryConfig(t)

		require.NoError(t, restoreRegistryConfigFromSecret())

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.JSONEq(t, `{"auths":{"ghcr.io":{}}}`, string(data))
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})

	t.Run("leaves the file alone without a secret", func(t *testing.T) {
		useSecrets(t)
		path := useRegistryConfig(t)

		require.NoError(t, restoreRegistryConfigFromSecret())

		_, err := os.Stat(path)
		assert.True(t, os.IsNotExist(err))
	})
}
