package reconciler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeVersionedObj(resourceVersion string, generation int64, annotations map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{}}
	obj.SetResourceVersion(resourceVersion)
	obj.SetGeneration(generation)
	if annotations != nil {
		obj.SetAnnotations(annotations)
	}
	return obj
}

func TestShouldReconcileUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		oldObj   *unstructured.Unstructured
		newObj   *unstructured.Unstructured
		expected bool
	}{
		{
			name:     "informer resync with unchanged object is skipped",
			oldObj:   makeVersionedObj("100", 3, nil),
			newObj:   makeVersionedObj("100", 3, nil),
			expected: false,
		},
		{
			name:     "status-only patch (new resourceVersion, same generation and annotations) is skipped",
			oldObj:   makeVersionedObj("100", 3, map[string]string{"a": "1"}),
			newObj:   makeVersionedObj("101", 3, map[string]string{"a": "1"}),
			expected: false,
		},
		{
			name:     "spec change (generation bump) reconciles",
			oldObj:   makeVersionedObj("100", 3, nil),
			newObj:   makeVersionedObj("101", 4, nil),
			expected: true,
		},
		{
			// The manual agent-run trigger: annotations change without a
			// generation bump. Must reconcile within informer latency, not
			// wait for the background sweep.
			name:     "annotation-only change reconciles",
			oldObj:   makeVersionedObj("100", 3, map[string]string{}),
			newObj:   makeVersionedObj("101", 3, map[string]string{"mogenius.com/run-requested-at": "2026-07-21T15:59:43Z"}),
			expected: true,
		},
		{
			name:     "annotation value change reconciles",
			oldObj:   makeVersionedObj("100", 3, map[string]string{"mogenius.com/run-requested-at": "old"}),
			newObj:   makeVersionedObj("101", 3, map[string]string{"mogenius.com/run-requested-at": "new"}),
			expected: true,
		},
		{
			// Resources without a generation (generation 0, e.g. plain
			// core objects) can't use the generation heuristic — every
			// version change reconciles.
			name:     "generation-less resource reconciles on any version change",
			oldObj:   makeVersionedObj("100", 0, nil),
			newObj:   makeVersionedObj("101", 0, nil),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, shouldReconcileUpdate(tt.oldObj, tt.newObj))
		})
	}
}
