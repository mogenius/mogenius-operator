package reconciler

import (
	"context"
	"mogenius-operator/src/utils"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func platformConfigWithSpec(spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
}

func patchRefs(names ...string) []any {
	refs := make([]any, 0, len(names))
	for _, n := range names {
		refs = append(refs, map[string]any{"name": n})
	}
	return refs
}

func TestReferencesPlatformPatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spec     map[string]any
		expected bool
	}{
		{
			name:     "top-level component references the patch",
			spec:     map[string]any{"traefik": map[string]any{"enabled": true, "patches": patchRefs("other", "my-patch")}},
			expected: true,
		},
		{
			name: "nested gitOps engine references the patch",
			spec: map[string]any{"gitOps": map[string]any{
				"argocd": map[string]any{"enabled": true, "patches": patchRefs("my-patch")},
			}},
			expected: true,
		},
		{
			name:     "only other patches referenced",
			spec:     map[string]any{"loki": map[string]any{"patches": patchRefs("other")}},
			expected: false,
		},
		{
			name: "name outside a patches list does not count",
			spec: map[string]any{"renovateOperator": map[string]any{
				"repositories": []any{map[string]any{"name": "my-patch"}},
			}},
			expected: false,
		},
		{
			name:     "empty spec",
			spec:     map[string]any{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, referencesPlatformPatch(tt.spec, "my-patch"))
		})
	}
}

func TestReconcilePlatformPatchRequeue(t *testing.T) {
	t.Parallel()

	referencing := platformConfigWithSpec(map[string]any{
		"certManager": map[string]any{"patches": patchRefs("my-patch")},
	})
	unrelated := platformConfigWithSpec(map[string]any{
		"certManager": map[string]any{"patches": patchRefs("other")},
	})

	tests := []struct {
		op            operation
		expectRequeue bool
	}{
		{createOperation, true},
		{updateOperation, true},
		{deleteOperation, true},
		{backgroundOperation, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			t.Parallel()

			var requeued *utils.ResourceDescriptor
			var match func(*unstructured.Unstructured) bool
			d := &reconcilerModule{
				requeue: func(resource utils.ResourceDescriptor, m func(*unstructured.Unstructured) bool) {
					requeued = &resource
					match = m
				},
			}

			patch := &unstructured.Unstructured{Object: map[string]any{}}
			patch.SetName("my-patch")
			assert.Nil(t, d.reconcilePlatformPatch(context.Background(), patch, tt.op))

			if !tt.expectRequeue {
				assert.Nil(t, requeued)
				return
			}
			assert.Equal(t, utils.PlatformConfigResource, *requeued)
			assert.True(t, match(referencing))
			assert.False(t, match(unrelated))
		})
	}
}
