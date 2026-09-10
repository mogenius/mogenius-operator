package gitops

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The renovate-zombie incident: helm-controller parks its uninstall finalizer
// on every HelmRelease, the operator's sweep replaced metadata wholesale and
// stripped it — and a HelmRelease deleted in that window vanished without the
// uninstall, leaving the release's workload running with nothing managing it.
func TestApplyKeepsForeignFinalizersOnUpdate(t *testing.T) {
	existing := helmRelease("traefik", defaultLabels("traefik"))
	existing.SetFinalizers([]string{"finalizers.fluxcd.io"})
	existing.SetAnnotations(map[string]string{"reconcile.fluxcd.io/requestedAt": "sometime"})
	client := testClient(t, existing).Resource(testHelmReleaseGVR).Namespace("flux-system")

	update := helmRelease("traefik", defaultLabels("traefik"))
	update.SetAnnotations(map[string]string{"mogenius.com/sweep": "now"})
	require.NoError(t, applyWith(context.Background(), client, update))

	applied, err := client.Get(context.Background(), "traefik", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"finalizers.fluxcd.io"}, applied.GetFinalizers(),
		"the uninstall finalizer is what makes deleting the object actually uninstall the release")
	// Foreign keys survive, ours arrive.
	assert.Equal(t, "sometime", applied.GetAnnotations()["reconcile.fluxcd.io/requestedAt"])
	assert.Equal(t, "now", applied.GetAnnotations()["mogenius.com/sweep"])
	assert.Equal(t, defaultLabels("traefik")["app.kubernetes.io/managed-by"], applied.GetLabels()["app.kubernetes.io/managed-by"])
}

// Ours win on conflict: the operator owns what it applies, and a foreign write
// to one of its own keys must not stick.
func TestApplyPrefersOurValuesOnConflict(t *testing.T) {
	existing := helmRelease("traefik", map[string]string{"app.kubernetes.io/managed-by": "someone-else"})
	client := testClient(t, existing).Resource(testHelmReleaseGVR).Namespace("flux-system")

	update := helmRelease("traefik", defaultLabels("traefik"))
	require.NoError(t, applyWith(context.Background(), client, update))

	applied, err := client.Get(context.Background(), "traefik", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, defaultLabels("traefik")["app.kubernetes.io/managed-by"], applied.GetLabels()["app.kubernetes.io/managed-by"])
}

func TestApplyCreatesWhenAbsent(t *testing.T) {
	client := testClient(t).Resource(testHelmReleaseGVR).Namespace("flux-system")

	object := helmRelease("traefik", defaultLabels("traefik"))
	require.NoError(t, applyWith(context.Background(), client, object))

	applied, err := client.Get(context.Background(), "traefik", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "traefik", applied.GetName())
}

// Keep the compiler honest about the helper's nil behavior.
func TestMergePreferringOurs(t *testing.T) {
	assert.Nil(t, mergePreferringOurs(nil, nil))
	assert.Equal(t, map[string]string{"a": "1"}, mergePreferringOurs(nil, map[string]string{"a": "1"}))
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, mergePreferringOurs(map[string]string{"a": "x", "b": "2"}, map[string]string{"a": "1"}))
}
