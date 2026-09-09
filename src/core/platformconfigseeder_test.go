package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
