package core

import (
	"testing"
	"time"

	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func makeItem(name, namespace, uid string, createdAt time.Time) unstructured.Unstructured {
	obj := unstructured.Unstructured{}
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetUID(types.UID(uid))
	obj.SetCreationTimestamp(metav1.NewTime(createdAt))
	return obj
}

func TestDedupeUnstructuredByUID_KeepsFirstOccurrence(t *testing.T) {
	a := makeItem("a", "ns", "uid-1", time.Unix(100, 0))
	b := makeItem("b", "ns", "uid-2", time.Unix(200, 0))
	aDup := makeItem("a-renamed", "ns", "uid-1", time.Unix(300, 0))

	out := dedupeUnstructuredByUID([]unstructured.Unstructured{a, b, aDup})

	assert.Len(t, out, 2)
	assert.Equal(t, "a", out[0].GetName())
	assert.Equal(t, "b", out[1].GetName())
}

func TestDedupeUnstructuredByUID_PreservesUIDLessItems(t *testing.T) {
	noUID1 := makeItem("x", "ns", "", time.Unix(100, 0))
	noUID2 := makeItem("y", "ns", "", time.Unix(200, 0))

	out := dedupeUnstructuredByUID([]unstructured.Unstructured{noUID1, noUID2})

	assert.Len(t, out, 2)
}

func TestDedupeUnstructuredByUID_EmptyInput(t *testing.T) {
	out := dedupeUnstructuredByUID(nil)
	assert.Empty(t, out)
}

func TestSortUnstructured_CreationTimestampDescDefault(t *testing.T) {
	older := makeItem("a", "ns", "uid-1", time.Unix(100, 0))
	newer := makeItem("b", "ns", "uid-2", time.Unix(200, 0))
	items := []unstructured.Unstructured{older, newer}

	sortUnstructured(items, "", "")

	assert.Equal(t, "b", items[0].GetName(), "newer should come first by default")
	assert.Equal(t, "a", items[1].GetName())
}

func TestSortUnstructured_CreationTimestampExplicitAsc(t *testing.T) {
	older := makeItem("a", "ns", "uid-1", time.Unix(100, 0))
	newer := makeItem("b", "ns", "uid-2", time.Unix(200, 0))
	items := []unstructured.Unstructured{newer, older}

	sortUnstructured(items, "creationTimestamp", "asc")

	assert.Equal(t, "a", items[0].GetName())
	assert.Equal(t, "b", items[1].GetName())
}

func TestSortUnstructured_NameAscDefault(t *testing.T) {
	b := makeItem("b", "ns", "uid-1", time.Unix(100, 0))
	a := makeItem("a", "ns", "uid-2", time.Unix(200, 0))
	items := []unstructured.Unstructured{b, a}

	sortUnstructured(items, "name", "")

	assert.Equal(t, "a", items[0].GetName())
	assert.Equal(t, "b", items[1].GetName())
}

func TestSortUnstructured_NameDesc(t *testing.T) {
	a := makeItem("a", "ns", "uid-1", time.Unix(100, 0))
	b := makeItem("b", "ns", "uid-2", time.Unix(200, 0))
	items := []unstructured.Unstructured{a, b}

	sortUnstructured(items, "name", "desc")

	assert.Equal(t, "b", items[0].GetName())
	assert.Equal(t, "a", items[1].GetName())
}

func TestSortUnstructured_StableByUIDOnEqualKey(t *testing.T) {
	// Same creation timestamp: UID breaks the tie deterministically.
	sameTime := time.Unix(100, 0)
	a := makeItem("a", "ns", "uid-zzz", sameTime)
	b := makeItem("b", "ns", "uid-aaa", sameTime)
	items := []unstructured.Unstructured{a, b}

	sortUnstructured(items, "creationTimestamp", "desc")

	// Both timestamps equal -> UID tiebreaker ascending -> "uid-aaa" first.
	assert.Equal(t, types.UID("uid-aaa"), items[0].GetUID())
	assert.Equal(t, types.UID("uid-zzz"), items[1].GetUID())
}

func TestSortUnstructured_UnknownSortByFallsBackToCreationTimestamp(t *testing.T) {
	older := makeItem("a", "ns", "uid-1", time.Unix(100, 0))
	newer := makeItem("b", "ns", "uid-2", time.Unix(200, 0))
	items := []unstructured.Unstructured{older, newer}

	sortUnstructured(items, "bogus-field", "")

	// Unknown sortBy + no order -> creationTimestamp desc default.
	assert.Equal(t, "b", items[0].GetName())
}

// A whitelist containing a cluster-scoped kind (e.g. Namespace) cannot be
// served by the (kind, namespace) pagination index, so the fast path must bail
// out to the in-memory path. (MOG-4362)
func TestIndexableWorkspaceNamespaces_ClusterScopedKindFallsBack(t *testing.T) {
	a := &api{}
	ns := utils.NamespaceResource // Namespaced: false
	req := WorkspaceResourcesPaginatedRequest{
		Whitelist:          []*utils.ResourceDescriptor{&ns},
		NamespaceWhitelist: []string{"some-ns"},
	}

	// Even for the cluster-wide case (empty workspaceName) the cluster-scoped
	// kind must force the fallback; the check runs before any store access.
	got, ok := a.indexableWorkspaceNamespaces("", req)
	assert.False(t, ok)
	assert.Nil(t, got)

	// And for a named workspace it must bail before touching the store/config.
	got, ok = a.indexableWorkspaceNamespaces("my-workspace", req)
	assert.False(t, ok)
	assert.Nil(t, got)
}

// A purely namespaced whitelist in the cluster-wide case stays on the index
// fast path and returns the namespace whitelist verbatim.
func TestIndexableWorkspaceNamespaces_NamespacedClusterWideUsesIndex(t *testing.T) {
	a := &api{}
	dep := utils.DeploymentResource // Namespaced: true
	req := WorkspaceResourcesPaginatedRequest{
		Whitelist:          []*utils.ResourceDescriptor{&dep},
		NamespaceWhitelist: []string{"ns-a", "ns-b"},
	}

	got, ok := a.indexableWorkspaceNamespaces("", req)
	assert.True(t, ok)
	assert.Equal(t, []string{"ns-a", "ns-b"}, got)
}

func TestFilterUnstructuredBySearch(t *testing.T) {
	items := []unstructured.Unstructured{
		makeSearchItem("nginx-frontend", "default", "Deployment", "apps/v1"),
		makeSearchItem("api-server", "mo-prod", "Deployment", "apps/v1"),
		makeSearchItem("cache", "default", "Service", "v1"),
	}

	assert.Len(t, filterUnstructuredBySearch(items, nil), 3)
	assert.Len(t, filterUnstructuredBySearch(items, store.BuildSearchFilterGroups("NGINX", nil, nil)), 1)
	// namespace match
	assert.Len(t, filterUnstructuredBySearch(items, store.BuildSearchFilterGroups("mo-prod", nil, nil)), 1)
	// kind match
	assert.Len(t, filterUnstructuredBySearch(items, store.BuildSearchFilterGroups("service", nil, nil)), 1)
	assert.Empty(t, filterUnstructuredBySearch(items, store.BuildSearchFilterGroups("does-not-exist", nil, nil)))
	// structured: name contains "nginx" AND namespace equals "default"
	assert.Len(t, filterUnstructuredBySearch(items, store.BuildSearchFilterGroups("", []store.SearchFilter{
		{Field: "name", Value: "nginx"},
		{Field: "namespace", Operator: store.SearchOperatorEquals, Value: "Default"},
	}, nil)), 1)
	// group with or-joined name constraints
	assert.Len(t, filterUnstructuredBySearch(items, store.BuildSearchFilterGroups("", nil, []store.SearchFilterGroup{
		{Field: "name", Operator: store.SearchGroupOperatorOr, Constraints: []store.SearchConstraint{{Value: "nginx"}, {Value: "cache"}}},
	})), 2)
}

func makeSearchItem(name, namespace, kind, apiVersion string) unstructured.Unstructured {
	item := unstructured.Unstructured{Object: map[string]any{}}
	item.SetName(name)
	item.SetNamespace(namespace)
	item.SetKind(kind)
	item.SetAPIVersion(apiVersion)
	return item
}
