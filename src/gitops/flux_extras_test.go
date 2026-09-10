package gitops

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func moacArtifact() GitOpsArtifact {
	return GitOpsArtifact{
		Namespace: "flux-system",
		HelmChart: HelmChartReference{Name: "flux-operator"},
		ExtraObjects: []any{
			map[string]any{"kind": "GitRepository"},
			map[string]any{"kind": "Kustomization"},
		},
	}
}

func dependsOn(release *unstructured.Unstructured) []any {
	value, found, err := unstructured.NestedSlice(release.Object, "spec", "dependsOn")
	if err != nil || !found {
		return nil
	}
	return value
}

// The incident this pins down: the engine's extras release carried a dependsOn
// on a HelmRelease named after the engine — which never exists, because the
// engine's chart goes in through the Helm SDK. The release then sat in
// "Fulfilling prerequisites" forever, with the repository sync objects inside
// it, so Flux ran and synced nothing.
func TestEngineExtrasCarryNoDependency(t *testing.T) {
	release := buildFluxMoacHelmRelease("flux-operator", moacArtifact(), "flux-system", false)

	assert.Nil(t, dependsOn(release), "there is no flux-operator HelmRelease to depend on")
}

// A component's extras still wait for the component itself: those really are
// delivered as a HelmRelease, and the extras belong after it.
func TestComponentExtrasDependOnTheComponent(t *testing.T) {
	release := buildFluxMoacHelmRelease("traefik", moacArtifact(), "flux-system", true)

	deps := dependsOn(release)
	assert.Len(t, deps, 1)
	assert.Equal(t, "traefik", deps[0].(map[string]any)["name"])
}
