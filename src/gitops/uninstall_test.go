package gitops

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

var testHelmReleaseGVR = schema.GroupVersionResource{
	Group:    "helm.toolkit.fluxcd.io",
	Version:  "v2",
	Resource: "helmreleases",
}

func helmRelease(name string, labels map[string]string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "flux-system",
		},
	}}
	if labels != nil {
		object.SetLabels(labels)
	}
	return object
}

func testClient(t *testing.T, objects ...*unstructured.Unstructured) *dynamicfake.FakeDynamicClient {
	t.Helper()

	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{testHelmReleaseGVR: "HelmReleaseList"}

	runtimeObjects := make([]runtime.Object, 0, len(objects))
	for _, object := range objects {
		runtimeObjects = append(runtimeObjects, object)
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, runtimeObjects...)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// The incident this guards: a Traefik that predated mogenius was deleted
// because a stray enabled:false reached the operator. A release this operator
// did not install is never removed, whoever wrote the false.
func TestDeleteIfManagedLeavesForeignObjectsAlone(t *testing.T) {
	foreign := helmRelease("traefik", map[string]string{"app.kubernetes.io/managed-by": "Helm"})
	client := testClient(t, foreign)
	resource := client.Resource(testHelmReleaseGVR).Namespace("flux-system")

	err := deleteIfManaged(context.Background(), discardLogger(), resource, "flux helmrelease", "traefik")

	require.NoError(t, err, "a skipped delete is not a failure")

	survivor, err := resource.Get(context.Background(), "traefik", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "traefik", survivor.GetName(), "a foreign release must survive an uninstall")
}

func TestDeleteIfManagedLeavesUnlabelledObjectsAlone(t *testing.T) {
	client := testClient(t, helmRelease("traefik", nil))
	resource := client.Resource(testHelmReleaseGVR).Namespace("flux-system")

	err := deleteIfManaged(context.Background(), discardLogger(), resource, "flux helmrelease", "traefik")

	require.NoError(t, err)
	_, err = resource.Get(context.Background(), "traefik", metav1.GetOptions{})
	assert.NoError(t, err, "an object without the label must not be deleted either")
}

// enabled:false still has to work for what mogenius installed, or a component
// could never be removed.
func TestDeleteIfManagedDeletesOwnObjects(t *testing.T) {
	client := testClient(t, helmRelease("traefik", defaultLabels("traefik")))
	resource := client.Resource(testHelmReleaseGVR).Namespace("flux-system")

	err := deleteIfManaged(context.Background(), discardLogger(), resource, "flux helmrelease", "traefik")

	require.NoError(t, err)
	_, err = resource.Get(context.Background(), "traefik", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "a release this operator installed must be removable")
}

func TestDeleteIfManagedIgnoresMissingObjects(t *testing.T) {
	client := testClient(t)
	resource := client.Resource(testHelmReleaseGVR).Namespace("flux-system")

	err := deleteIfManaged(context.Background(), discardLogger(), resource, "flux helmrelease", "traefik")

	assert.NoError(t, err, "nothing to delete is the desired state, not an error")
}

func TestDeleteIfManagedToleratesANilLogger(t *testing.T) {
	client := testClient(t, helmRelease("traefik", map[string]string{"app.kubernetes.io/managed-by": "Helm"}))
	resource := client.Resource(testHelmReleaseGVR).Namespace("flux-system")

	assert.NotPanics(t, func() {
		_ = deleteIfManaged(context.Background(), nil, resource, "flux helmrelease", "traefik")
	})
}

func TestDefaultLabelsCarryTheManagedByMarker(t *testing.T) {
	labels := defaultLabels("traefik")

	assert.Equal(t, ManagedByValue, labels[ManagedByLabel],
		"UnInstall matches on this label, so Install has to write it")
	assert.Equal(t, "traefik", labels["app.kubernetes.io/component"])
}
