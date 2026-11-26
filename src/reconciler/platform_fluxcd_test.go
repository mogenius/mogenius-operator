package reconciler

import (
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func fluxInstanceSpec(instance *v1alpha1.FluxInstanceConfig) map[string]any {
	spec := v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{FluxCD: &v1alpha1.FluxCDInstallConfig{Enabled: true, Instance: instance}},
	}
	return fluxInstanceObject("flux-system", spec)["spec"].(map[string]any)
}

func TestFluxInstanceWithoutInstanceConfigKeepsFluxDefaults(t *testing.T) {
	spec := fluxInstanceSpec(nil)

	assert.NotContains(t, spec, "cluster")
	assert.NotContains(t, spec, "kustomize")
}

func TestFluxInstanceTakesClusterAndKustomizeAsIs(t *testing.T) {
	spec := fluxInstanceSpec(&v1alpha1.FluxInstanceConfig{
		Cluster:   &apiextensionsv1.JSON{Raw: []byte(`{"size":"medium","multitenant":false}`)},
		Kustomize: &apiextensionsv1.JSON{Raw: []byte(`{"patches":[{"patch":"p","target":{"kind":"Deployment"}}]}`)},
	})

	assert.Equal(t, map[string]any{"size": "medium", "multitenant": false}, spec["cluster"])
	assert.Equal(t, map[string]any{"patches": []any{
		map[string]any{"patch": "p", "target": map[string]any{"kind": "Deployment"}},
	}}, spec["kustomize"])
}

// The operator's own patches are appended, not replaced by the configured ones.
func TestFluxInstanceKeepsTheOperatorsPatches(t *testing.T) {
	spec := v1alpha1.PlatformConfigSpec{
		GitOps: &v1alpha1.GitOpsConfig{FluxCD: &v1alpha1.FluxCDInstallConfig{
			Enabled: true,
			Instance: &v1alpha1.FluxInstanceConfig{
				Kustomize: &apiextensionsv1.JSON{Raw: []byte(`{"patches":[{"patch":"p"}]}`)},
			},
		}},
		VibeCoding: &v1alpha1.VibeCodingConfig{Enabled: true},
	}

	patches := fluxInstanceObject("flux-system", spec)["spec"].(map[string]any)["kustomize"].(map[string]any)["patches"].([]any)

	assert.Len(t, patches, 2)
	assert.Equal(t, map[string]any{"patch": "p"}, patches[0])
}

// Bitbucket takes an access token only under x-token-auth; every other host
// keeps Flux's git convention.
func TestFluxReadUsername(t *testing.T) {
	tests := []struct {
		name string
		repo v1alpha1.GitOpsRepositoryConfig
		want string
	}{
		{name: "github keeps git", repo: v1alpha1.GitOpsRepositoryConfig{URL: "https://github.com/acme/platform.git"}, want: "git"},
		{name: "bitbucket cloud", repo: v1alpha1.GitOpsRepositoryConfig{URL: "https://bitbucket.org/acme/platform.git"}, want: "x-token-auth"},
		{name: "self-hosted bitbucket host", repo: v1alpha1.GitOpsRepositoryConfig{URL: "https://bitbucket.acme.internal/scm/plat/platform.git"}, want: "x-token-auth"},
		{
			name: "bitbucket named by write.provider on a host that says nothing",
			repo: v1alpha1.GitOpsRepositoryConfig{
				URL:   "https://git.acme.internal/scm/plat/platform.git",
				Write: &v1alpha1.GitOpsWriteConfig{Provider: "BITBUCKET"},
			},
			want: "x-token-auth",
		},
		{
			name: "explicit provider wins over the host",
			repo: v1alpha1.GitOpsRepositoryConfig{
				URL:   "https://bitbucket.acme.internal/scm/plat/platform.git",
				Write: &v1alpha1.GitOpsWriteConfig{Provider: "GIT_LAB"},
			},
			want: "git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fluxReadUsername(tt.repo))
		})
	}
}
