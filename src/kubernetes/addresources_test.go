package kubernetes

import (
	"testing"

	"mogenius-operator/src/crds"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Every embedded CRD manifest must yield a name, because the establishment
// wait after the apply addresses the CRD by it.
func TestCrdNameFromYamlOnEmbeddedManifests(t *testing.T) {
	t.Parallel()

	for _, crd := range crds.GetCRDs() {
		name, err := crdNameFromYaml(crd.Content)
		assert.NoError(t, err, crd.Filename)
		assert.NotEmpty(t, name, crd.Filename)
	}
}

func TestCrdIsEstablished(t *testing.T) {
	t.Parallel()

	crdWithConditions := func(conditions []any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]any{"name": "platformconfigs.mogenius.com"},
			"status":     map[string]any{"conditions": conditions},
		}}
	}

	assert.True(t, crdIsEstablished(crdWithConditions([]any{
		map[string]any{"type": "NamesAccepted", "status": "True"},
		map[string]any{"type": "Established", "status": "True"},
	})))

	// Freshly created: the condition exists but is not True yet.
	assert.False(t, crdIsEstablished(crdWithConditions([]any{
		map[string]any{"type": "Established", "status": "False"},
	})))

	// No status at all — the API server has not gotten to it.
	assert.False(t, crdIsEstablished(&unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "platformconfigs.mogenius.com"},
	}}))
}
