package core

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

// The seeded object carries no configuration at all. An empty spec declares
// neither a repository nor an engine, which is what makes creating it safe: the
// operator only publishes the detected GitOps status on it, and the repository
// is connected through the mogenius UI.
func TestDefaultPlatformConfigHasEmptySpec(t *testing.T) {
	t.Parallel()

	platformConfig := defaultPlatformConfig(time.Now())

	spec, found, err := unstructured.NestedMap(platformConfig.Object, "spec")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Empty(t, spec)

	assert.Equal(t, DEFAULT_PLATFORM_CONFIG_NAME, platformConfig.GetName())
}

// The annotation is what tells an admin the object came from the operator rather
// than from a user or a GitOps engine.
func TestDefaultPlatformConfigStampsBootstrappedAt(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.September, 9, 12, 34, 56, 0, time.UTC)

	annotation := defaultPlatformConfig(createdAt).GetAnnotations()[PLATFORM_CONFIG_BOOTSTRAPPED_AT_ANNOTATION]

	assert.Equal(t, "2026-09-09T12:34:56Z", annotation)

	parsed, err := time.Parse(time.RFC3339, annotation)
	assert.NoError(t, err)
	assert.True(t, createdAt.Equal(parsed))
}

var testPlatformConfigGVR = schema.GroupVersionResource{
	Group:    "mogenius.com",
	Version:  "v1alpha1",
	Resource: "platformconfigs",
}

func seederTestClient(t *testing.T, objects ...*unstructured.Unstructured) *dynamicfake.FakeDynamicClient {
	t.Helper()

	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{testPlatformConfigGVR: "PlatformConfigList"}

	runtimeObjects := make([]runtime.Object, 0, len(objects))
	for _, object := range objects {
		runtimeObjects = append(runtimeObjects, object)
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, runtimeObjects...)
}

func seederTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSeedDefaultPlatformConfigCreatesWhenMissing(t *testing.T) {
	t.Parallel()

	client := seederTestClient(t)

	err := seedDefaultPlatformConfig(context.Background(), seederTestLogger(), client.Resource(testPlatformConfigGVR))
	assert.NoError(t, err)

	created, err := client.Resource(testPlatformConfigGVR).Get(context.Background(), DEFAULT_PLATFORM_CONFIG_NAME, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotEmpty(t, created.GetAnnotations()[PLATFORM_CONFIG_BOOTSTRAPPED_AT_ANNOTATION])
}

// Only on create: an existing object may already be synced from git, and
// re-seeding it would fight the synced config on every leadership change.
func TestSeedDefaultPlatformConfigLeavesExistingUntouched(t *testing.T) {
	t.Parallel()

	existing := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "mogenius.com/v1alpha1",
		"kind":       "PlatformConfig",
		"metadata":   map[string]any{"name": DEFAULT_PLATFORM_CONFIG_NAME},
		"spec":       map[string]any{"gitOps": map[string]any{"fluxcd": map[string]any{"enabled": true}}},
	}}
	client := seederTestClient(t, existing)

	err := seedDefaultPlatformConfig(context.Background(), seederTestLogger(), client.Resource(testPlatformConfigGVR))
	assert.NoError(t, err)

	current, err := client.Resource(testPlatformConfigGVR).Get(context.Background(), DEFAULT_PLATFORM_CONFIG_NAME, metav1.GetOptions{})
	assert.NoError(t, err)
	enabled, _, _ := unstructured.NestedBool(current.Object, "spec", "gitOps", "fluxcd", "enabled")
	assert.True(t, enabled)
	assert.Empty(t, current.GetAnnotations()[PLATFORM_CONFIG_BOOTSTRAPPED_AT_ANNOTATION])
}

// A concurrent replica winning the create is not a failure.
func TestSeedDefaultPlatformConfigToleratesAlreadyExists(t *testing.T) {
	t.Parallel()

	client := seederTestClient(t)
	client.PrependReactor("get", "platformconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(testPlatformConfigGVR.GroupResource(), DEFAULT_PLATFORM_CONFIG_NAME)
	})
	client.PrependReactor("create", "platformconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewAlreadyExists(testPlatformConfigGVR.GroupResource(), DEFAULT_PLATFORM_CONFIG_NAME)
	})

	err := seedDefaultPlatformConfig(context.Background(), seederTestLogger(), client.Resource(testPlatformConfigGVR))
	assert.NoError(t, err)
}

// The race the retry exists for: the CRD is applied moments before the seeder
// runs, and until it is Established the API server answers NotFound even to
// Create. That attempt must surface as an error so the caller retries, instead
// of the cluster silently staying without a PlatformConfig until the next
// leadership change.
func TestSeedDefaultPlatformConfigReportsCrdNotServedYet(t *testing.T) {
	t.Parallel()

	client := seederTestClient(t)
	client.PrependReactor("get", "platformconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "mogenius.com", Resource: "platformconfigs"}, "")
	})
	client.PrependReactor("create", "platformconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "mogenius.com", Resource: "platformconfigs"}, "")
	})

	err := seedDefaultPlatformConfig(context.Background(), seederTestLogger(), client.Resource(testPlatformConfigGVR))
	assert.Error(t, err)
	assert.ErrorContains(t, err, "create platform config")
}
