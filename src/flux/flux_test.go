package flux

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResolveKindGVR(t *testing.T) {
	cases := []struct {
		kind string
		want schema.GroupVersionResource
	}{
		{"Kustomization", schema.GroupVersionResource{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"}},
		{"HelmRelease", schema.GroupVersionResource{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"}},
		{"GitRepository", schema.GroupVersionResource{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"}},
		{"OCIRepository", schema.GroupVersionResource{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "ocirepositories"}},
		{"HelmRepository", schema.GroupVersionResource{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmrepositories"}},
		{"Bucket", schema.GroupVersionResource{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "buckets"}},
		// kind matching is case-insensitive
		{"kustomization", schema.GroupVersionResource{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"}},
		{"ocirepository", schema.GroupVersionResource{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "ocirepositories"}},
	}
	for _, tc := range cases {
		got, err := resolveKindGVR(tc.kind)
		if err != nil {
			t.Errorf("resolveKindGVR(%q) returned error: %v", tc.kind, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveKindGVR(%q) = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

func TestResolveKindGVRRejectsUnsupportedKinds(t *testing.T) {
	for _, kind := range []string{"", "Deployment", "Application", "HelmChart", "FluxInstance"} {
		if _, err := resolveKindGVR(kind); err == nil {
			t.Errorf("resolveKindGVR(%q) = nil error, want error", kind)
		}
	}
}

func TestResolveSourceRefKustomization(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"name": "podinfo", "namespace": "flux-system"},
		"spec": map[string]any{
			"sourceRef": map[string]any{
				"kind": "GitRepository",
				"name": "podinfo",
			},
		},
	}}
	ref := resolveSourceRef(obj)
	if ref == nil {
		t.Fatal("resolveSourceRef returned nil, want a sourceRef")
	}
	if ref.Kind != "GitRepository" || ref.Name != "podinfo" {
		t.Errorf("resolveSourceRef = %+v, want kind GitRepository name podinfo", ref)
	}
	if ref.Namespace != "flux-system" {
		t.Errorf("sourceRef namespace = %q, want fallback to object namespace flux-system", ref.Namespace)
	}
}

func TestResolveSourceRefCrossNamespace(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"name": "podinfo", "namespace": "apps"},
		"spec": map[string]any{
			"sourceRef": map[string]any{
				"kind":      "GitRepository",
				"name":      "podinfo",
				"namespace": "flux-system",
			},
		},
	}}
	ref := resolveSourceRef(obj)
	if ref == nil {
		t.Fatal("resolveSourceRef returned nil, want a sourceRef")
	}
	if ref.Namespace != "flux-system" {
		t.Errorf("sourceRef namespace = %q, want explicit flux-system", ref.Namespace)
	}
}

func TestResolveSourceRefHelmReleaseChartSpec(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata":   map[string]any{"name": "podinfo", "namespace": "flux-system"},
		"spec": map[string]any{
			"chart": map[string]any{
				"spec": map[string]any{
					"chart": "podinfo",
					"sourceRef": map[string]any{
						"kind": "HelmRepository",
						"name": "podinfo-repo",
					},
				},
			},
		},
	}}
	ref := resolveSourceRef(obj)
	if ref == nil {
		t.Fatal("resolveSourceRef returned nil, want a sourceRef")
	}
	if ref.Kind != "HelmRepository" || ref.Name != "podinfo-repo" || ref.Namespace != "flux-system" {
		t.Errorf("resolveSourceRef = %+v, want HelmRepository podinfo-repo in flux-system", ref)
	}
}

func TestResolveSourceRefHelmReleaseChartRefWins(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata":   map[string]any{"name": "podinfo", "namespace": "flux-system"},
		"spec": map[string]any{
			"chartRef": map[string]any{
				"kind": "OCIRepository",
				"name": "podinfo-oci",
			},
			"chart": map[string]any{
				"spec": map[string]any{
					"sourceRef": map[string]any{
						"kind": "HelmRepository",
						"name": "podinfo-repo",
					},
				},
			},
		},
	}}
	ref := resolveSourceRef(obj)
	if ref == nil {
		t.Fatal("resolveSourceRef returned nil, want a sourceRef")
	}
	if ref.Kind != "OCIRepository" || ref.Name != "podinfo-oci" {
		t.Errorf("resolveSourceRef = %+v, want chartRef (OCIRepository podinfo-oci) to win over chart.spec.sourceRef", ref)
	}
}

func TestResolveSourceRefNoneForSourceKinds(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "GitRepository",
		"metadata":   map[string]any{"name": "podinfo", "namespace": "flux-system"},
		"spec": map[string]any{
			"url": "https://github.com/stefanprodan/podinfo",
		},
	}}
	if ref := resolveSourceRef(obj); ref != nil {
		t.Errorf("resolveSourceRef = %+v, want nil for a source kind without sourceRef", ref)
	}
}

func TestResolveSourceRefIncompleteRefIsSkipped(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
		"kind":       "Kustomization",
		"metadata":   map[string]any{"name": "podinfo", "namespace": "flux-system"},
		"spec": map[string]any{
			"sourceRef": map[string]any{
				"kind": "GitRepository",
				// name missing
			},
		},
	}}
	if ref := resolveSourceRef(obj); ref != nil {
		t.Errorf("resolveSourceRef = %+v, want nil for an incomplete sourceRef", ref)
	}
	if ref := resolveSourceRef(nil); ref != nil {
		t.Errorf("resolveSourceRef(nil) = %+v, want nil", ref)
	}
}

func TestSourceRefKindGVRsCoverChartRefKinds(t *testing.T) {
	// chartRef may point at a HelmChart, which is not directly addressable
	// via the socket commands but must be resolvable as a source.
	for _, kind := range []string{"GitRepository", "OCIRepository", "HelmRepository", "Bucket", "HelmChart"} {
		if _, ok := fluxSourceRefKindGVRs[kind]; !ok {
			t.Errorf("fluxSourceRefKindGVRs is missing kind %q", kind)
		}
	}
}
