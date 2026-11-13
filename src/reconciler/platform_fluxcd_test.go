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
