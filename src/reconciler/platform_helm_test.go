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
		{
			name: "slice in src overwrites slice in dst entirely",
			dst:  map[string]any{"items": []any{"a", "b"}},
			src:  map[string]any{"items": []any{"c"}},
			want: map[string]any{"items": []any{"c"}},
		},
		{
			name: "slice in src overwrites nested map in dst",
			dst:  map[string]any{"k": map[string]any{"a": 1}},
			src:  map[string]any{"k": []any{"x", "y"}},
			want: map[string]any{"k": []any{"x", "y"}},
		},
		{
			name: "realistic traefik-style merge",
			dst: map[string]any{
				"globalArguments": []any{"--global.checknewversion=false"},
				"service": map[string]any{
					"type": "LoadBalancer",
					"annotations": map[string]any{
						"cloud.google.com/load-balancer-type": "External",
					},
				},
				"ports": map[string]any{
					"web":       map[string]any{"port": 80, "expose": true},
					"websecure": map[string]any{"port": 443, "expose": true},
				},
				"ingressClass": map[string]any{
					"enabled":        true,
					"isDefaultClass": true,
				},
			},
			src: map[string]any{
				"metrics": map[string]any{
					"prometheus": map[string]any{
						"enabled":    true,
						"entryPoint": "metrics",
						"service":    map[string]any{"enabled": true},
						"serviceMonitor": map[string]any{
							"enabled": true,
						},
					},
				},
				"ports": map[string]any{
					"metrics": map[string]any{"port": 9100, "expose": false},
				},
			},
			want: map[string]any{
				"globalArguments": []any{"--global.checknewversion=false"},
				"service": map[string]any{
					"type": "LoadBalancer",
					"annotations": map[string]any{
						"cloud.google.com/load-balancer-type": "External",
					},
				},
				"ports": map[string]any{
					"web":       map[string]any{"port": 80, "expose": true},
					"websecure": map[string]any{"port": 443, "expose": true},
					"metrics":   map[string]any{"port": 9100, "expose": false},
				},
				"ingressClass": map[string]any{
					"enabled":        true,
					"isDefaultClass": true,
				},
				"metrics": map[string]any{
					"prometheus": map[string]any{
						"enabled":    true,
						"entryPoint": "metrics",
						"service":    map[string]any{"enabled": true},
						"serviceMonitor": map[string]any{
							"enabled": true,
						},
					},
				},
			},
		},
		{
			name: "realistic argocd-style merge preserves all sibling components",
			dst: map[string]any{
				"global": map[string]any{
					"image": map[string]any{"tag": "v2.10.0"},
				},
				"controller": map[string]any{
					"replicas": 1,
					"metrics":  map[string]any{"enabled": false},
				},
				"server": map[string]any{
					"replicas":  1,
					"metrics":   map[string]any{"enabled": false},
					"extraArgs": []any{"--insecure"},
				},
				"repoServer": map[string]any{
					"replicas": 1,
					"metrics":  map[string]any{"enabled": false},
				},
				"applicationSet": map[string]any{
					"replicas": 1,
					"metrics":  map[string]any{"enabled": false},
				},
			},
			src: map[string]any{
				"controller":     map[string]any{"metrics": map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}}},
				"server":         map[string]any{"metrics": map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}}},
				"repoServer":     map[string]any{"metrics": map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}}},
				"applicationSet": map[string]any{"metrics": map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}}},
			},
			want: map[string]any{
				"global": map[string]any{
					"image": map[string]any{"tag": "v2.10.0"},
				},
				"controller": map[string]any{
					"replicas": 1,
					"metrics":  map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}},
				},
				"server": map[string]any{
					"replicas":  1,
					"extraArgs": []any{"--insecure"},
					"metrics":   map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}},
				},
				"repoServer": map[string]any{
					"replicas": 1,
					"metrics":  map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}},
				},
				"applicationSet": map[string]any{
					"replicas": 1,
					"metrics":  map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}},
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
		{
			// Patches are applied in slice order; the last patch wins.
			name: "multiple patches applied in order, last wins",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{"key": "default"},
			},
			patch: func() []v1alpha1.PlatformPatch {
				return []v1alpha1.PlatformPatch{
					patch(t, map[string]any{"key": "patch1"})[0],
					patch(t, map[string]any{"key": "patch2"})[0],
					patch(t, map[string]any{"key": "patch3"})[0],
				}
			}(),
			want: map[string]any{"key": "patch3"},
		},
		{
			name: "multiple patches each contribute different keys",
			patch: func() []v1alpha1.PlatformPatch {
				return []v1alpha1.PlatformPatch{
					patch(t, map[string]any{"a": 1})[0],
					patch(t, map[string]any{"b": 2})[0],
					patch(t, map[string]any{"c": 3})[0],
				}
			}(),
			want: map[string]any{"a": float64(1), "b": float64(2), "c": float64(3)},
		},
		{
			name: "realistic external-secrets-operator merge",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{
					"replicaCount": 1,
					"resources": map[string]any{
						"requests": map[string]any{"cpu": "10m", "memory": "32Mi"},
						"limits":   map[string]any{"memory": "128Mi"},
					},
					"serviceMonitor": map[string]any{"enabled": false},
					"metrics": map[string]any{
						"service": map[string]any{"enabled": false},
					},
				},
			},
			configValues: map[string]any{
				"serviceMonitor": map[string]any{"enabled": true},
				"metrics": map[string]any{
					"service": map[string]any{"enabled": true},
				},
			},
			want: map[string]any{
				"replicaCount": 1,
				"resources": map[string]any{
					"requests": map[string]any{"cpu": "10m", "memory": "32Mi"},
					"limits":   map[string]any{"memory": "128Mi"},
				},
				"serviceMonitor": map[string]any{"enabled": true},
				"metrics": map[string]any{
					"service": map[string]any{"enabled": true},
				},
			},
		},
		{
			name: "realistic argocd three-layer merge",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{
					"global": map[string]any{
						"image": map[string]any{"tag": "v2.10.0"},
					},
					"controller": map[string]any{
						"replicas": 1,
						"metrics":  map[string]any{"enabled": false},
					},
					"server": map[string]any{
						"replicas":  1,
						"metrics":   map[string]any{"enabled": false},
						"extraArgs": []any{"--insecure"},
					},
					"repoServer": map[string]any{
						"replicas": 1,
						"metrics":  map[string]any{"enabled": false},
					},
					"applicationSet": map[string]any{
						"replicas": 1,
						"metrics":  map[string]any{"enabled": false},
					},
				},
			},
			configValues: map[string]any{
				"controller":     map[string]any{"metrics": map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}}},
				"server":         map[string]any{"metrics": map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}}},
				"repoServer":     map[string]any{"metrics": map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}}},
				"applicationSet": map[string]any{"metrics": map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}}},
			},
			patch: patch(t, map[string]any{
				"global": map[string]any{
					"image": map[string]any{"tag": "v2.11.0"},
				},
				"server": map[string]any{
					"replicas": 2,
				},
			}),
			want: map[string]any{
				"global": map[string]any{
					"image": map[string]any{"tag": "v2.11.0"},
				},
				"controller": map[string]any{
					"replicas": 1,
					"metrics":  map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}},
				},
				"server": map[string]any{
					"replicas":  float64(2), // JSON-unmarshalled patch value
					"extraArgs": []any{"--insecure"},
					"metrics":   map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}},
				},
				"repoServer": map[string]any{
					"replicas": 1,
					"metrics":  map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}},
				},
				"applicationSet": map[string]any{
					"replicas": 1,
					"metrics":  map[string]any{"enabled": true, "serviceMonitor": map[string]any{"enabled": true}},
				},
			},
		},
		{
			name: "patch deep-overrides one nested key while all siblings survive",
			defaults: componentDefaultSpec{
				ValuesObject: map[string]any{
					"component": map[string]any{
						"replicas": 1,
						"image":    map[string]any{"repository": "myrepo", "tag": "v1"},
						"resources": map[string]any{
							"requests": map[string]any{"cpu": "100m", "memory": "128Mi"},
							"limits":   map[string]any{"cpu": "500m", "memory": "512Mi"},
						},
					},
				},
			},
			patch: patch(t, map[string]any{
				"component": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{"memory": "1Gi"},
					},
				},
			}),
			want: map[string]any{
				"component": map[string]any{
					"replicas": 1,
					"image":    map[string]any{"repository": "myrepo", "tag": "v1"},
					"resources": map[string]any{
						"requests": map[string]any{"cpu": "100m", "memory": "128Mi"},
						"limits":   map[string]any{"cpu": "500m", "memory": "1Gi"},
					},
				},
			},
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
