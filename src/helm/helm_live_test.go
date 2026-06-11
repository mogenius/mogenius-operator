package helm

import (
	"os"
	"sort"
	"testing"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/storage/driver"
)

// TestLivePaginatedListing exercises the new two-phase listing internals
// (metadata stub index + per-page decode + slicing) against the cluster in the
// ambient kubeconfig. It deliberately bypasses the valkey repoName lookup (which
// needs the full operator bootstrap) and verifies only the parts this change
// introduced. Skipped unless HELM_LIVE_TEST=1; assumes HELM_LIVE_NS is seeded
// (see _work/helm-load-test/seed.sh). Manual verification aid, not a CI test.
func TestLivePaginatedListing(t *testing.T) {
	if os.Getenv("HELM_LIVE_TEST") != "1" {
		t.Skip("set HELM_LIVE_TEST=1 to run the live paginated listing test")
	}
	ns := os.Getenv("HELM_LIVE_NS")
	if ns == "" {
		ns = "helm-load-test"
	}

	// Phase 1: metadata-only stub index (no gzip blob).
	stubs, err := listReleaseStubsCached(ns)
	if err != nil {
		t.Fatalf("listReleaseStubsCached failed: %v", err)
	}
	t.Logf("stub index: %d current releases in %q", len(stubs), ns)
	if len(stubs) == 0 {
		t.Fatalf("no releases found in %q - seed it first", ns)
	}

	seen := map[string]bool{}
	for _, hr := range stubs {
		if hr.Name == "" || hr.Namespace == "" {
			t.Errorf("stub missing name/namespace: %+v", hr)
		}
		if hr.Version <= 0 {
			t.Errorf("stub %q has non-positive version %d", hr.Name, hr.Version)
		}
		if hr.Info == nil || hr.Info.LastDeployed.IsZero() {
			t.Errorf("stub %q missing lastDeployed (modifiedAt label)", hr.Name)
		}
		key := hr.Namespace + "/" + hr.Name
		if seen[key] {
			t.Errorf("duplicate release in index (dedup failed): %s", key)
		}
		seen[key] = true
	}

	// Phase 1 sort + slice via the shared pagination function.
	page1, total := paginateHelmReleases(stubs, HelmReleaseListPaginatedRequest{SortBy: "name", Offset: 0, Limit: 5}, nil)
	if total != len(stubs) {
		t.Errorf("total %d != stub count %d", total, len(stubs))
	}
	if len(page1) > 5 {
		t.Errorf("page1 returned %d items, expected <= 5", len(page1))
	}
	if !sort.SliceIsSorted(page1, func(i, j int) bool { return page1[i].Name < page1[j].Name }) {
		t.Errorf("page1 not sorted by name")
	}

	page2, _ := paginateHelmReleases(stubs, HelmReleaseListPaginatedRequest{SortBy: "name", Offset: 5, Limit: 5}, nil)
	if len(page1) > 0 && len(page2) > 0 && page1[0].Name == page2[0].Name {
		t.Errorf("page1 and page2 start with same release %q - slicing broken", page1[0].Name)
	}

	// Phase 2: decode the full release only for the page, and confirm it matches
	// the stub identity and carries chart metadata (proving the blob was fetched).
	settings := NewCli()
	settings.SetNamespace(ns)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), ns, ""); err != nil {
		t.Fatalf("actionConfig init: %v", err)
	}
	secretsDriver, ok := actionConfig.Releases.Driver.(*driver.Secrets)
	if !ok {
		t.Fatalf("expected secrets driver, got %T", actionConfig.Releases.Driver)
	}
	for _, stub := range page1 {
		re, err := decodePageRelease(secretsDriver, stub.Namespace, stub.Name, stub.Version)
		if err != nil {
			t.Errorf("decodePageRelease(%s) failed: %v", stub.Name, err)
			continue
		}
		if re.Name != stub.Name || re.Version != stub.Version {
			t.Errorf("decoded release mismatch: got %s v%d, want %s v%d", re.Name, re.Version, stub.Name, stub.Version)
		}
		if re.Chart == nil || re.Chart.Metadata == nil || re.Chart.Metadata.Name == "" {
			t.Errorf("decoded release %q missing chart metadata", stub.Name)
		}
	}
	t.Logf("decoded %d page releases successfully", len(page1))
}
