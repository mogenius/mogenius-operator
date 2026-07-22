package openbaomanager

import (
	"errors"
	"strings"
	"testing"

	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/utils"

	"github.com/stretchr/testify/assert"
)

func TestResolveTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *v1alpha1.OpenBaoConfig
		want target
	}{
		{
			name: "defaults",
			cfg:  &v1alpha1.OpenBaoConfig{},
			want: target{
				namespace: "openbao",
				release:   "openbao",
				kvMount:   "secret",
				storeName: "openbao",
				serverURL: "http://openbao.openbao.svc:8200",
			},
		},
		{
			name: "overrides",
			cfg: &v1alpha1.OpenBaoConfig{
				KVMount:         "kv",
				SecretStoreName: "vault-store",
				Chart: &v1alpha1.HelmChartReference{
					Name:      "bao",
					Namespace: "security",
				},
			},
			want: target{
				namespace: "security",
				release:   "bao",
				kvMount:   "kv",
				storeName: "vault-store",
				serverURL: "http://bao.security.svc:8200",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, resolveTarget(tc.cfg))
		})
	}
}

func TestGvrFor(t *testing.T) {
	t.Parallel()

	gvr := gvrFor(utils.ClusterSecretStoreResource)
	assert.Equal(t, "external-secrets.io", gvr.Group)
	assert.Equal(t, "v1", gvr.Version)
	assert.Equal(t, "clustersecretstores", gvr.Resource)

	// Core resource with no group.
	core := gvrFor(utils.ConfigMapResource)
	assert.Equal(t, "", core.Group)
	assert.Equal(t, "v1", core.Version)
	assert.Equal(t, "configmaps", core.Resource)
}

func TestIsAlreadyExists(t *testing.T) {
	t.Parallel()

	assert.False(t, isAlreadyExists(nil))
	assert.True(t, isAlreadyExists(errors.New("path is already in use at secret/")))
	assert.True(t, isAlreadyExists(errors.New("mount already exists")))
	assert.False(t, isAlreadyExists(errors.New("permission denied")))
}

func TestEsoReadPolicy(t *testing.T) {
	t.Parallel()

	policy := esoReadPolicy("secret")
	assert.Contains(t, policy, `path "secret/data/*"`)
	assert.Contains(t, policy, `path "secret/metadata/*"`)
	assert.Contains(t, policy, `capabilities = ["read"]`)

	custom := esoReadPolicy("kv")
	assert.Contains(t, custom, `path "kv/data/*"`)
	assert.NotContains(t, custom, "secret/data")
}

func TestClusterSecretStoreObject(t *testing.T) {
	t.Parallel()

	tgt := resolveTarget(&v1alpha1.OpenBaoConfig{})
	obj := clusterSecretStoreObject(tgt)

	assert.Equal(t, "external-secrets.io/v1", obj["apiVersion"])
	assert.Equal(t, "ClusterSecretStore", obj["kind"])

	meta := obj["metadata"].(map[string]any)
	assert.Equal(t, "openbao", meta["name"])

	vault := obj["spec"].(map[string]any)["provider"].(map[string]any)["vault"].(map[string]any)
	assert.Equal(t, "http://openbao.openbao.svc:8200", vault["server"])
	assert.Equal(t, "secret", vault["path"])
	assert.Equal(t, "v2", vault["version"])

	k8sAuth := vault["auth"].(map[string]any)["kubernetes"].(map[string]any)
	assert.Equal(t, esoRoleName, k8sAuth["role"])
	assert.Equal(t, kubernetesAuthPath, k8sAuth["mountPath"])
	saRef := k8sAuth["serviceAccountRef"].(map[string]any)
	assert.Equal(t, esoServiceAccountName, saRef["name"])
	assert.Equal(t, esoNamespace, saRef["namespace"])

	// The store name must be a valid-looking DNS name (no spaces/uppercase).
	assert.Equal(t, strings.ToLower(meta["name"].(string)), meta["name"])
}
