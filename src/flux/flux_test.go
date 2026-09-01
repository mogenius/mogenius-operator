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

// onlySourceRef asserts that refs holds exactly one reference and returns it.
func onlySourceRef(t *testing.T, refs []fluxSourceRef) fluxSourceRef {
	t.Helper()
	if len(refs) != 1 {
		t.Fatalf("resolveSourceRefs returned %d refs (%+v), want exactly 1", len(refs), refs)
	}
	return refs[0]
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
	ref := onlySourceRef(t, resolveSourceRefs(obj))
	if ref.Kind != "GitRepository" || ref.Name != "podinfo" {
		t.Errorf("resolveSourceRefs = %+v, want kind GitRepository name podinfo", ref)
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
	ref := onlySourceRef(t, resolveSourceRefs(obj))
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
	ref := onlySourceRef(t, resolveSourceRefs(obj))
	if ref.Kind != "HelmRepository" || ref.Name != "podinfo-repo" || ref.Namespace != "flux-system" {
		t.Errorf("resolveSourceRefs = %+v, want HelmRepository podinfo-repo in flux-system", ref)
	}
	if ref.BestEffort {
		t.Error("a sourceRef read from the spec must not be BestEffort")
	}
}

// A HelmRelease built from a spec.chart template must annotate BOTH the
// HelmRepository and the HelmChart helm-controller generated from it: the
// HelmChart is what resolves spec.chart.spec.version, and it only sees a new
// chart version once the repository index has been refreshed.
func TestResolveSourceRefHelmReleaseIncludesStatusHelmChart(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "helm.toolkit.fluxcd.io/v2",
		"kind":       "HelmRelease",
		"metadata":   map[string]any{"name": "podinfo", "namespace": "apps"},
		"spec": map[string]any{
			"chart": map[string]any{
				"spec": map[string]any{
					"chart":   "podinfo",
					"version": "6.x",
					"sourceRef": map[string]any{
						"kind": "HelmRepository",
						"name": "podinfo-repo",
					},
				},
			},
		},
		"status": map[string]any{
			"helmChart": "apps/apps-podinfo",
		},
	}}
	refs := resolveSourceRefs(obj)
	if len(refs) != 2 {
		t.Fatalf("resolveSourceRefs = %+v, want 2 refs (HelmRepository, HelmChart)", refs)
	}
	if refs[0].Kind != "HelmRepository" || refs[0].Name != "podinfo-repo" {
		t.Errorf("refs[0] = %+v, want the HelmRepository first so the index refreshes before the chart re-resolves", refs[0])
	}
	if refs[1].Kind != kindHelmChart || refs[1].Name != "apps-podinfo" || refs[1].Namespace != "apps" {
		t.Errorf("refs[1] = %+v, want HelmChart apps-podinfo in apps", refs[1])
	}
	if !refs[1].BestEffort {
		t.Error("the HelmChart is derived from status and must be BestEffort")
	}
}

func TestStatusHelmChartRefRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "no-slash", "/podinfo", "apps/", "  "} {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata":   map[string]any{"name": "podinfo", "namespace": "apps"},
			"status":     map[string]any{"helmChart": value},
		}}
		if ref := statusHelmChartRef(obj); ref != nil {
			t.Errorf("statusHelmChartRef(%q) = %+v, want nil", value, ref)
		}
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
		// a leftover from a chart-template migration must not be annotated
		"status": map[string]any{
			"helmChart": "flux-system/flux-system-podinfo",
		},
	}}
	ref := onlySourceRef(t, resolveSourceRefs(obj))
	if ref.Kind != "OCIRepository" || ref.Name != "podinfo-oci" {
		t.Errorf("resolveSourceRefs = %+v, want chartRef (OCIRepository podinfo-oci) to win over chart.spec.sourceRef and status.helmChart", ref)
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
	if refs := resolveSourceRefs(obj); len(refs) != 0 {
		t.Errorf("resolveSourceRefs = %+v, want none for a source kind without sourceRef", refs)
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
	if refs := resolveSourceRefs(obj); len(refs) != 0 {
		t.Errorf("resolveSourceRefs = %+v, want none for an incomplete sourceRef", refs)
	}
	if refs := resolveSourceRefs(nil); len(refs) != 0 {
		t.Errorf("resolveSourceRefs(nil) = %+v, want none", refs)
	}
}

func TestSourceRefKindGVRsCoverChartRefKinds(t *testing.T) {
	// chartRef may point at a HelmChart, and helm-controller generates one
	// for a spec.chart template. Neither is directly addressable via the
	// socket commands, but both must be resolvable as a source.
	for _, kind := range []string{"GitRepository", "OCIRepository", "HelmRepository", "Bucket", kindHelmChart} {
		if _, ok := fluxSourceRefKindGVRs[kind]; !ok {
			t.Errorf("fluxSourceRefKindGVRs is missing kind %q", kind)
		}
	}
}
