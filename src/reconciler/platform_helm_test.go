package reconciler

import (
	"encoding/json"
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/stretchr/testify/assert"
)

func mustJSON(t *testing.T, v map[string]any) *apiextensionsv1.JSON {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustJSON: %v", err)
	}
	return &apiextensionsv1.JSON{Raw: raw}
}

func patch(t *testing.T, values map[string]any) []v1alpha1.PlatformPatch {
	t.Helper()
	p := v1alpha1.PlatformPatch{}
	if values != nil {
		p.Spec.ValuesObject = mustJSON(t, values)
	}
	return []v1alpha1.PlatformPatch{p}
}

func TestMergeMaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dst  map[string]any
		src  map[string]any
		want map[string]any
	}{
		{
			name: "empty src leaves dst unchanged",
			dst:  map[string]any{"a": 1},
			src:  map[string]any{},
			want: map[string]any{"a": 1},
		},
		{
			name: "nil src leaves dst unchanged",
			dst:  map[string]any{"a": 1},
			src:  nil,
			want: map[string]any{"a": 1},
		},
		{
			name: "src key added to empty dst",
			dst:  map[string]any{},
			src:  map[string]any{"x": "y"},
			want: map[string]any{"x": "y"},
		},
		{
			name: "src scalar overwrites dst scalar",
			dst:  map[string]any{"k": "old"},
			src:  map[string]any{"k": "new"},
			want: map[string]any{"k": "new"},
		},
		{
			name: "nested maps are merged recursively",
			dst:  map[string]any{"nested": map[string]any{"a": 1, "b": 2}},
			src:  map[string]any{"nested": map[string]any{"b": 99, "c": 3}},
			want: map[string]any{"nested": map[string]any{"a": 1, "b": 99, "c": 3}},
		},
		{
			name: "scalar in src replaces nested map in dst",
			dst:  map[string]any{"k": map[string]any{"a": 1}},
			src:  map[string]any{"k": "scalar"},
			want: map[string]any{"k": "scalar"},
		},
		{
			name: "nested map in src replaces scalar in dst",
			dst:  map[string]any{"k": "scalar"},
			src:  map[string]any{"k": map[string]any{"a": 1}},
			want: map[string]any{"k": map[string]any{"a": 1}},
		},
		{
			name: "unrelated keys in dst are preserved",
			dst:  map[string]any{"keep": true, "overwrite": "old"},
			src:  map[string]any{"overwrite": "new", "add": "also"},
			want: map[string]any{"keep": true, "overwrite": "new", "add": "also"},
		},
		{
			name: "deep recursion merges multiple levels",
			dst: map[string]any{
				"l1": map[string]any{
					"l2": map[string]any{"a": 1, "b": 2},
				},
			},
			src: map[string]any{
				"l1": map[string]any{
					"l2": map[string]any{"b": 99},
				},
			},
			want: map[string]any{
				"l1": map[string]any{
					"l2": map[string]any{"a": 1, "b": 99},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mergeMaps(tc.dst, tc.src)
			assert.Equal(t, tc.want, tc.dst)
		})
	}
}

func TestMergeHelmValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		defaults     componentDefaultSpec
		configValues map[string]any
		patch        []v1alpha1.PlatformPatch
		want         map[string]any
		wantErr      bool
	}{
		{
			name: "all empty",
			want: map[string]any{},
		},
		{
			name: "only defaults",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{"a": "1", "b": "2"},
			},
			want: map[string]any{"a": "1", "b": "2"},
		},
		{
			name:         "only config values",
			configValues: map[string]any{"x": true},
			want:         map[string]any{"x": true},
		},
		{
			name:  "only patch values",
			patch: patch(t, map[string]any{"p": 42}),
			want:  map[string]any{"p": float64(42)}, // JSON unmarshal produces float64 for numbers
		},
		{
			name: "config overrides defaults",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{"key": "default", "other": "keep"},
			},
			configValues: map[string]any{"key": "overridden"},
			want:         map[string]any{"key": "overridden", "other": "keep"},
		},
		{
			name: "patch overrides config and defaults",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{"key": "default"},
			},
			configValues: map[string]any{"key": "config"},
			patch:        patch(t, map[string]any{"key": "patch"}),
			want:         map[string]any{"key": "patch"},
		},
		{
			name: "deep merge preserves sibling keys in defaults",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{
					"nested": map[string]any{"a": 1, "b": 2},
				},
			},
			configValues: map[string]any{
				"nested": map[string]any{"b": 99},
			},
			want: map[string]any{
				"nested": map[string]any{"a": 1, "b": 99},
			},
		},
		{
			// Patch values go through JSON unmarshal so numbers become float64
			name: "deep merge patch preserves sibling keys from config",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{
					"nested": map[string]any{"a": 1},
				},
			},
			configValues: map[string]any{
				"nested": map[string]any{"b": 2},
			},
			patch: patch(t, map[string]any{
				"nested": map[string]any{"c": 3},
			}),
			want: map[string]any{
				"nested": map[string]any{"a": 1, "b": 2, "c": float64(3)},
			},
		},
		{
			name: "patch nil ValuesObject is a no-op",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{"k": "v"},
			},
			patch: []v1alpha1.PlatformPatch{},
			want:  map[string]any{"k": "v"},
		},
		{
			name: "nil patch is a no-op",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{"k": "v"},
			},
			patch: nil,
			want:  map[string]any{"k": "v"},
		},
		{
			name: "patch invalid JSON returns error",
			patch: []v1alpha1.PlatformPatch{
				v1alpha1.PlatformPatch{
					Spec: v1alpha1.PlatformPatchSpec{
						ValuesObject: &apiextensionsv1.JSON{Raw: []byte(`{bad json}`)},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "scalar patch value overwrites nested map",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{
					"nested": map[string]any{"a": 1},
				},
			},
			patch: patch(t, map[string]any{"nested": "scalar"}),
			want:  map[string]any{"nested": "scalar"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := mergeHelmValues(tc.defaults, tc.configValues, tc.patch)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
