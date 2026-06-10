package helm

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	release "helm.sh/helm/v4/pkg/release/v1"
)

func rel(name string, deployed time.Time) *HelmRelease {
	return &HelmRelease{
		Name: name,
		Info: &release.Info{LastDeployed: deployed},
	}
}

func names(releases []*HelmRelease) []string {
	out := make([]string, 0, len(releases))
	for _, r := range releases {
		out = append(out, r.Name)
	}
	return out
}

func TestPaginateHelmReleases(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// a: oldest, b: middle, c: newest
	a := rel("alpha", base)
	b := rel("bravo", base.Add(time.Hour))
	c := rel("charlie", base.Add(2*time.Hour))

	cases := []struct {
		name      string
		input     []*HelmRelease
		req       HelmReleaseListPaginatedRequest
		wantNames []string
		wantTotal int
	}{
		{
			name:      "default sort is lastDeployed desc (newest first)",
			input:     []*HelmRelease{a, b, c},
			req:       HelmReleaseListPaginatedRequest{},
			wantNames: []string{"charlie", "bravo", "alpha"},
			wantTotal: 3,
		},
		{
			name:      "lastDeployed asc",
			input:     []*HelmRelease{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "lastDeployed", SortOrder: "asc"},
			wantNames: []string{"alpha", "bravo", "charlie"},
			wantTotal: 3,
		},
		{
			name:      "sort by name asc (default order)",
			input:     []*HelmRelease{c, a, b},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name"},
			wantNames: []string{"alpha", "bravo", "charlie"},
			wantTotal: 3,
		},
		{
			name:      "sort by name desc",
			input:     []*HelmRelease{a, c, b},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", SortOrder: "desc"},
			wantNames: []string{"charlie", "bravo", "alpha"},
			wantTotal: 3,
		},
		{
			name:      "filter is case-insensitive substring",
			input:     []*HelmRelease{a, b, c},
			req:       HelmReleaseListPaginatedRequest{Filter: "AR", SortBy: "name"},
			wantNames: []string{"charlie"}, // only "charlie" contains "ar"
			wantTotal: 1,
		},
		{
			name:      "offset+limit slices a page; total reflects full set",
			input:     []*HelmRelease{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", Offset: 1, Limit: 1},
			wantNames: []string{"bravo"},
			wantTotal: 3,
		},
		{
			name:      "limit 0 means no limit",
			input:     []*HelmRelease{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", Limit: 0},
			wantNames: []string{"alpha", "bravo", "charlie"},
			wantTotal: 3,
		},
		{
			name:      "offset beyond total yields empty page but correct total",
			input:     []*HelmRelease{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", Offset: 10, Limit: 5},
			wantNames: []string{},
			wantTotal: 3,
		},
		{
			name:      "limit larger than remaining clamps to end",
			input:     []*HelmRelease{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", Offset: 2, Limit: 10},
			wantNames: []string{"charlie"},
			wantTotal: 3,
		},
		{
			name:      "nil Info sorts as zero time",
			input:     []*HelmRelease{c, {Name: "noinfo"}},
			req:       HelmReleaseListPaginatedRequest{}, // lastDeployed desc
			wantNames: []string{"charlie", "noinfo"},
			wantTotal: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, total := paginateHelmReleases(tc.input, tc.req, nil)
			assert.Equal(t, tc.wantTotal, total)
			assert.Equal(t, tc.wantNames, names(page))
		})
	}
}

func TestPaginateHelmReleasesWorkspaceScope(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	a := &HelmRelease{Name: "alpha", Namespace: "ns1", Info: &release.Info{LastDeployed: base}}
	b := &HelmRelease{Name: "bravo", Namespace: "ns2", Info: &release.Info{LastDeployed: base.Add(time.Hour)}}
	c := &HelmRelease{Name: "charlie", Namespace: "ns1", Info: &release.Info{LastDeployed: base.Add(2 * time.Hour)}}
	all := []*HelmRelease{a, b, c}

	t.Run("scopes to the workspace allow-set by namespace+name", func(t *testing.T) {
		scope := &HelmWorkspaceScope{Allowed: map[string]struct{}{
			WorkspaceHelmKey("ns1", "alpha"): {},
			WorkspaceHelmKey("ns2", "bravo"): {},
		}}
		page, total := paginateHelmReleases(all, HelmReleaseListPaginatedRequest{SortBy: "name"}, scope)
		assert.Equal(t, 2, total)
		assert.Equal(t, []string{"alpha", "bravo"}, names(page))
	})

	t.Run("namespace must match - same name in another namespace is excluded", func(t *testing.T) {
		scope := &HelmWorkspaceScope{Allowed: map[string]struct{}{
			WorkspaceHelmKey("ns2", "alpha"): {}, // alpha lives in ns1, not ns2
		}}
		page, total := paginateHelmReleases(all, HelmReleaseListPaginatedRequest{SortBy: "name"}, scope)
		assert.Equal(t, 0, total)
		assert.Equal(t, []string{}, names(page))
	})

	t.Run("empty allow-set yields nothing (workspace with no helm resources)", func(t *testing.T) {
		scope := &HelmWorkspaceScope{Allowed: map[string]struct{}{}}
		page, total := paginateHelmReleases(all, HelmReleaseListPaginatedRequest{}, scope)
		assert.Equal(t, 0, total)
		assert.Equal(t, []string{}, names(page))
	})

	t.Run("workspace scope composes with filter, sort and slicing", func(t *testing.T) {
		scope := &HelmWorkspaceScope{Allowed: map[string]struct{}{
			WorkspaceHelmKey("ns1", "alpha"):   {},
			WorkspaceHelmKey("ns1", "charlie"): {},
			WorkspaceHelmKey("ns2", "bravo"):   {},
		}}
		// filter "l" matches alpha + charlie (both contain "l"); bravo excluded
		page, total := paginateHelmReleases(
			all,
			HelmReleaseListPaginatedRequest{Filter: "l", SortBy: "name", Offset: 1, Limit: 1},
			scope,
		)
		assert.Equal(t, 2, total)                         // alpha, charlie match scope+filter
		assert.Equal(t, []string{"charlie"}, names(page)) // offset 1 of [alpha, charlie]
	})
}

// Argo entries participate in the same sort/filter/scope/slice as real releases.
func TestPaginateHelmReleasesWithArgoEntries(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	real := &HelmRelease{Name: "bravo", Namespace: "ns1", Info: &release.Info{LastDeployed: base.Add(time.Hour)}}
	// Argo entry: name sorting uses .Name, lastDeployed sorting uses Argo.CreatedAt,
	// scope keys on Argo.ParentNamespace.
	argo := NewArgoHelmRelease("alpha", &ArgoReleaseInfo{
		ParentName:      "alpha-app",
		ParentNamespace: "argocd",
		DestNamespace:   "ns1",
		CreatedAt:       base.Add(2 * time.Hour), // newest
		RepoName:        "mogenius",
		ChartName:       "nginx",
		Version:         "1.2.3",
	})

	t.Run("argo entry sorts by its creationTimestamp under lastDeployed", func(t *testing.T) {
		page, total := paginateHelmReleases([]*HelmRelease{real, argo}, HelmReleaseListPaginatedRequest{}, nil)
		assert.Equal(t, 2, total)
		assert.Equal(t, []string{"alpha", "bravo"}, names(page)) // argo newest -> first
	})

	t.Run("argo entry is scoped by argocd install namespace + release name", func(t *testing.T) {
		scope := &HelmWorkspaceScope{Allowed: map[string]struct{}{
			WorkspaceHelmKey("argocd", "alpha"): {}, // matches the argo entry's ParentNamespace
		}}
		page, total := paginateHelmReleases([]*HelmRelease{real, argo}, HelmReleaseListPaginatedRequest{SortBy: "name"}, scope)
		assert.Equal(t, 1, total)
		assert.Equal(t, []string{"alpha"}, names(page))
	})
}

// The Argo entry serializes to the "git-ops-argo-cd-application" shape the
// frontend expects; a real release keeps the default helm-release shape.
func TestHelmReleaseMarshalJSON(t *testing.T) {
	t.Run("argo entry emits the git-ops shape", func(t *testing.T) {
		argo := NewArgoHelmRelease("my-release", &ArgoReleaseInfo{
			Application:     map[string]any{"kind": "Application", "metadata": map[string]any{"name": "my-app"}},
			ParentName:      "my-app",
			ParentNamespace: "argocd",
			ValuesObject:    "replicaCount: 2\n",
			ChartName:       "nginx",
			Version:         "1.2.3",
			AppVersion:      "1.25.0",
			DestNamespace:   "prod",
			RepoName:        "mogenius",
		})

		raw, err := json.Marshal(argo)
		assert.NoError(t, err)

		var got map[string]any
		assert.NoError(t, json.Unmarshal(raw, &got))

		assert.Equal(t, "git-ops-argo-cd-application", got["type"])
		assert.Equal(t, "my-release", got["releaseName"])
		assert.Equal(t, "my-release", got["name"])
		assert.Equal(t, "nginx", got["chartName"])
		assert.Equal(t, "1.2.3", got["version"])
		assert.Equal(t, "1.25.0", got["appVersion"])
		assert.Equal(t, "prod", got["namespace"])
		assert.Equal(t, "mogenius", got["repoName"])

		data := got["data"].(map[string]any)
		assert.Equal(t, "replicaCount: 2\n", data["valuesObject"])
		assert.NotNil(t, data["application"])

		parent := data["parentApplication"].(map[string]any)
		assert.Equal(t, "my-app", parent["resourceName"])
		assert.Equal(t, "argocd", parent["namespace"])
		assert.Equal(t, "Application", parent["kind"])
		assert.Equal(t, "argoproj.io/v1alpha1", parent["apiVersion"])
	})

	t.Run("real release keeps the default shape", func(t *testing.T) {
		re := &HelmRelease{Name: "real", Namespace: "ns", RepoName: "repo"}
		raw, err := json.Marshal(re)
		assert.NoError(t, err)

		var got map[string]any
		assert.NoError(t, json.Unmarshal(raw, &got))

		assert.Equal(t, "real", got["name"])
		assert.Equal(t, "ns", got["namespace"])
		assert.Equal(t, "repo", got["repoName"])
		_, hasType := got["type"]
		assert.False(t, hasType) // no argo "type" marker on real releases
	})
}
