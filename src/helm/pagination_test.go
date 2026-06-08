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
			page, total := paginateReleases(tc.input, tc.req)
			assert.Equal(t, tc.wantTotal, total)
			assert.Equal(t, tc.wantNames, names(page))
		})
	}
}
