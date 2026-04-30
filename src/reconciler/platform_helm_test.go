package reconciler

import (
	"encoding/json"
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/stretchr/testify/assert"
)

func mustJSON(t *testing.T, v map[string]interface{}) *apiextensionsv1.JSON {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustJSON: %v", err)
	}
	return &apiextensionsv1.JSON{Raw: raw}
}

func patch(t *testing.T, values map[string]interface{}) *v1alpha1.PlatformPatch {
	t.Helper()
	p := &v1alpha1.PlatformPatch{}
	if values != nil {
		p.Spec.ValuesObject = mustJSON(t, values)
	}
	return p
}

func TestMergeMaps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dst  map[string]interface{}
		src  map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "empty src leaves dst unchanged",
			dst:  map[string]interface{}{"a": 1},
			src:  map[string]interface{}{},
			want: map[string]interface{}{"a": 1},
		},
		{
			name: "nil src leaves dst unchanged",
			dst:  map[string]interface{}{"a": 1},
			src:  nil,
			want: map[string]interface{}{"a": 1},
		},
		{
			name: "src key added to empty dst",
			dst:  map[string]interface{}{},
			src:  map[string]interface{}{"x": "y"},
			want: map[string]interface{}{"x": "y"},
		},
		{
			name: "src scalar overwrites dst scalar",
			dst:  map[string]interface{}{"k": "old"},
			src:  map[string]interface{}{"k": "new"},
			want: map[string]interface{}{"k": "new"},
		},
		{
			name: "nested maps are merged recursively",
			dst:  map[string]interface{}{"nested": map[string]interface{}{"a": 1, "b": 2}},
			src:  map[string]interface{}{"nested": map[string]interface{}{"b": 99, "c": 3}},
			want: map[string]interface{}{"nested": map[string]interface{}{"a": 1, "b": 99, "c": 3}},
		},
		{
			name: "scalar in src replaces nested map in dst",
			dst:  map[string]interface{}{"k": map[string]interface{}{"a": 1}},
			src:  map[string]interface{}{"k": "scalar"},
			want: map[string]interface{}{"k": "scalar"},
		},
		{
			name: "nested map in src replaces scalar in dst",
			dst:  map[string]interface{}{"k": "scalar"},
			src:  map[string]interface{}{"k": map[string]interface{}{"a": 1}},
			want: map[string]interface{}{"k": map[string]interface{}{"a": 1}},
		},
		{
			name: "unrelated keys in dst are preserved",
			dst:  map[string]interface{}{"keep": true, "overwrite": "old"},
			src:  map[string]interface{}{"overwrite": "new", "add": "also"},
			want: map[string]interface{}{"keep": true, "overwrite": "new", "add": "also"},
		},
		{
			name: "deep recursion merges multiple levels",
			dst: map[string]interface{}{
				"l1": map[string]interface{}{
					"l2": map[string]interface{}{"a": 1, "b": 2},
				},
			},
			src: map[string]interface{}{
				"l1": map[string]interface{}{
					"l2": map[string]interface{}{"b": 99},
				},
			},
			want: map[string]interface{}{
				"l1": map[string]interface{}{
					"l2": map[string]interface{}{"a": 1, "b": 99},
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
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
		configValues map[string]interface{}
		patch        *v1alpha1.PlatformPatch
		want         map[string]interface{}
		wantErr      bool
	}{
		{
			name:    "all empty",
			want:    map[string]interface{}{},
		},
		{
			name: "only defaults",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]interface{}{"a": "1", "b": "2"},
			},
			want: map[string]interface{}{"a": "1", "b": "2"},
		},
		{
			name:         "only config values",
			configValues: map[string]interface{}{"x": true},
			want:         map[string]interface{}{"x": true},
		},
		{
			name:  "only patch values",
			patch: patch(t, map[string]interface{}{"p": 42}),
			want:  map[string]interface{}{"p": float64(42)}, // JSON unmarshal produces float64 for numbers
		},
		{
			name: "config overrides defaults",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]interface{}{"key": "default", "other": "keep"},
			},
			configValues: map[string]interface{}{"key": "overridden"},
			want:         map[string]interface{}{"key": "overridden", "other": "keep"},
		},
		{
			name: "patch overrides config and defaults",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]interface{}{"key": "default"},
			},
			configValues: map[string]interface{}{"key": "config"},
			patch:        patch(t, map[string]interface{}{"key": "patch"}),
			want:         map[string]interface{}{"key": "patch"},
		},
		{
			name: "deep merge preserves sibling keys in defaults",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]interface{}{
					"nested": map[string]interface{}{"a": 1, "b": 2},
				},
			},
			configValues: map[string]interface{}{
				"nested": map[string]interface{}{"b": 99},
			},
			want: map[string]interface{}{
				"nested": map[string]interface{}{"a": 1, "b": 99},
			},
		},
		{
			// Patch values go through JSON unmarshal so numbers become float64
			name: "deep merge patch preserves sibling keys from config",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]interface{}{
					"nested": map[string]interface{}{"a": 1},
				},
			},
			configValues: map[string]interface{}{
				"nested": map[string]interface{}{"b": 2},
			},
			patch: patch(t, map[string]interface{}{
				"nested": map[string]interface{}{"c": 3},
			}),
			want: map[string]interface{}{
				"nested": map[string]interface{}{"a": 1, "b": 2, "c": float64(3)},
			},
		},
		{
			name: "patch nil ValuesObject is a no-op",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]interface{}{"k": "v"},
			},
			patch: &v1alpha1.PlatformPatch{},
			want:  map[string]interface{}{"k": "v"},
		},
		{
			name: "nil patch is a no-op",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]interface{}{"k": "v"},
			},
			patch: nil,
			want:  map[string]interface{}{"k": "v"},
		},
		{
			name: "patch invalid JSON returns error",
			patch: &v1alpha1.PlatformPatch{
				Spec: v1alpha1.PlatformPatchSpec{
					ValuesObject: &apiextensionsv1.JSON{Raw: []byte(`{bad json}`)},
				},
			},
			wantErr: true,
		},
		{
			name: "scalar patch value overwrites nested map",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]interface{}{
					"nested": map[string]interface{}{"a": 1},
				},
			},
			patch: patch(t, map[string]interface{}{"nested": "scalar"}),
			want:  map[string]interface{}{"nested": "scalar"},
		},
	}

	for _, tc := range tests {
		tc := tc
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
