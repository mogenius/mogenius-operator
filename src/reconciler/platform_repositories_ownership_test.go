package reconciler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The incident this pins down: on a cluster whose Flux came through the
// flux-operator, the repositories path applied the operator's bare
// FluxInstance template over the user's existing instance. The apply replaces
// the spec wholesale, so the instance lost its spec.sync, the flux-operator
// garbage-collected the root Kustomization, and its prune finalizer deleted
// every application the cluster ran. An object without the operator's
// managed-by label must never be written.
func TestPlatformObjectForeignRejectsUnlabelledObjects(t *testing.T) {
	existing := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "flux"},
	}}

	assert.True(t, platformObjectForeign(existing))
}

func TestPlatformObjectForeignRejectsObjectsManagedByOthers(t *testing.T) {
	existing := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":   "flux",
			"labels": map[string]any{"app.kubernetes.io/managed-by": "kustomize-controller"},
		},
	}}

	assert.True(t, platformObjectForeign(existing))
}

// Absence is not foreign: a missing object is free to create, and an object
// the operator labelled on a previous sweep is its own to keep reconciling.
func TestPlatformObjectForeignAllowsAbsentAndOwnObjects(t *testing.T) {
	assert.False(t, platformObjectForeign(nil))

	own := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":   "flux",
			"labels": map[string]any{"app.kubernetes.io/managed-by": "mogenius-operator"},
		},
	}}
	assert.False(t, platformObjectForeign(own))
}

// A sync someone added to an operator-owned FluxInstance survives the sweep:
// the template never declares one, and dropping it detaches the cluster from
// its Git repository — the same failure the ownership guard prevents for
// foreign instances.
func TestPreserveFluxInstanceSyncCarriesTheExistingSyncOver(t *testing.T) {
	existing := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"sync": map[string]any{"kind": "GitRepository", "url": "https://github.com/acme/platform.git"},
		},
	}}
	desired := &unstructured.Unstructured{Object: fluxInstanceObject("flux-system")}

	preserveFluxInstanceSync(existing, desired)

	sync, found, err := unstructured.NestedMap(desired.Object, "spec", "sync")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "https://github.com/acme/platform.git", sync["url"])
}

// A sync the desired object declares itself wins over the existing one, and an
// existing instance without a sync adds nothing.
func TestPreserveFluxInstanceSyncLeavesADeclaredSyncAlone(t *testing.T) {
	existing := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"sync": map[string]any{"url": "https://github.com/acme/old.git"},
		},
	}}
	desired := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"sync": map[string]any{"url": "https://github.com/acme/declared.git"},
		},
	}}

	preserveFluxInstanceSync(existing, desired)

	sync, _, _ := unstructured.NestedMap(desired.Object, "spec", "sync")
	assert.Equal(t, "https://github.com/acme/declared.git", sync["url"])

	bare := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{}}}
	desired = &unstructured.Unstructured{Object: fluxInstanceObject("flux-system")}
	preserveFluxInstanceSync(bare, desired)
	_, found, _ := unstructured.NestedMap(desired.Object, "spec", "sync")
	assert.False(t, found)
}
