package helm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	release "helm.sh/helm/v4/pkg/release/v1"
)

func rel(name string, deployed time.Time) *release.Release {
	return &release.Release{
		Name: name,
		Info: &release.Info{LastDeployed: deployed},
	}
}

func names(releases []*release.Release) []string {
	out := make([]string, 0, len(releases))
	for _, r := range releases {
		out = append(out, r.Name)
	}
	return out
}

func TestPaginateReleases(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// a: oldest, b: middle, c: newest
	a := rel("alpha", base)
	b := rel("bravo", base.Add(time.Hour))
	c := rel("charlie", base.Add(2*time.Hour))

	cases := []struct {
		name      string
		input     []*release.Release
		req       HelmReleaseListPaginatedRequest
		wantNames []string
		wantTotal int
	}{
		{
			name:      "default sort is lastDeployed desc (newest first)",
			input:     []*release.Release{a, b, c},
			req:       HelmReleaseListPaginatedRequest{},
			wantNames: []string{"charlie", "bravo", "alpha"},
			wantTotal: 3,
		},
		{
			name:      "lastDeployed asc",
			input:     []*release.Release{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "lastDeployed", SortOrder: "asc"},
			wantNames: []string{"alpha", "bravo", "charlie"},
			wantTotal: 3,
		},
		{
			name:      "sort by name asc (default order)",
			input:     []*release.Release{c, a, b},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name"},
			wantNames: []string{"alpha", "bravo", "charlie"},
			wantTotal: 3,
		},
		{
			name:      "sort by name desc",
			input:     []*release.Release{a, c, b},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", SortOrder: "desc"},
			wantNames: []string{"charlie", "bravo", "alpha"},
			wantTotal: 3,
		},
		{
			name:      "filter is case-insensitive substring",
			input:     []*release.Release{a, b, c},
			req:       HelmReleaseListPaginatedRequest{Filter: "AR", SortBy: "name"},
			wantNames: []string{"charlie"}, // only "charlie" contains "ar"
			wantTotal: 1,
		},
		{
			name:      "offset+limit slices a page; total reflects full set",
			input:     []*release.Release{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", Offset: 1, Limit: 1},
			wantNames: []string{"bravo"},
			wantTotal: 3,
		},
		{
			name:      "limit 0 means no limit",
			input:     []*release.Release{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", Limit: 0},
			wantNames: []string{"alpha", "bravo", "charlie"},
			wantTotal: 3,
		},
		{
			name:      "offset beyond total yields empty page but correct total",
			input:     []*release.Release{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", Offset: 10, Limit: 5},
			wantNames: []string{},
			wantTotal: 3,
		},
		{
			name:      "limit larger than remaining clamps to end",
			input:     []*release.Release{a, b, c},
			req:       HelmReleaseListPaginatedRequest{SortBy: "name", Offset: 2, Limit: 10},
			wantNames: []string{"charlie"},
			wantTotal: 3,
		},
		{
			name:      "nil Info sorts as zero time",
			input:     []*release.Release{c, {Name: "noinfo"}},
			req:       HelmReleaseListPaginatedRequest{}, // lastDeployed desc
			wantNames: []string{"charlie", "noinfo"},
			wantTotal: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, total := paginateReleases(tc.input, tc.req, nil)
			assert.Equal(t, tc.wantTotal, total)
			assert.Equal(t, tc.wantNames, names(page))
		})
	}
}

func TestPaginateReleasesWorkspaceScope(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	a := &release.Release{Name: "alpha", Namespace: "ns1", Info: &release.Info{LastDeployed: base}}
	b := &release.Release{Name: "bravo", Namespace: "ns2", Info: &release.Info{LastDeployed: base.Add(time.Hour)}}
	c := &release.Release{Name: "charlie", Namespace: "ns1", Info: &release.Info{LastDeployed: base.Add(2 * time.Hour)}}
	all := []*release.Release{a, b, c}

	t.Run("scopes to the workspace allow-set by namespace+name", func(t *testing.T) {
		scope := &HelmWorkspaceScope{Allowed: map[string]struct{}{
			WorkspaceHelmKey("ns1", "alpha"): {},
			WorkspaceHelmKey("ns2", "bravo"): {},
		}}
		page, total := paginateReleases(all, HelmReleaseListPaginatedRequest{SortBy: "name"}, scope)
		assert.Equal(t, 2, total)
		assert.Equal(t, []string{"alpha", "bravo"}, names(page))
	})

	t.Run("namespace must match - same name in another namespace is excluded", func(t *testing.T) {
		scope := &HelmWorkspaceScope{Allowed: map[string]struct{}{
			WorkspaceHelmKey("ns2", "alpha"): {}, // alpha lives in ns1, not ns2
		}}
		page, total := paginateReleases(all, HelmReleaseListPaginatedRequest{SortBy: "name"}, scope)
		assert.Equal(t, 0, total)
		assert.Equal(t, []string{}, names(page))
	})

	t.Run("empty allow-set yields nothing (workspace with no helm resources)", func(t *testing.T) {
		scope := &HelmWorkspaceScope{Allowed: map[string]struct{}{}}
		page, total := paginateReleases(all, HelmReleaseListPaginatedRequest{}, scope)
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
		page, total := paginateReleases(
			all,
			HelmReleaseListPaginatedRequest{Filter: "l", SortBy: "name", Offset: 1, Limit: 1},
			scope,
		)
		assert.Equal(t, 2, total)                         // alpha, charlie match scope+filter
		assert.Equal(t, []string{"charlie"}, names(page)) // offset 1 of [alpha, charlie]
	})
}
