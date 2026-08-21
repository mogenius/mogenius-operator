package store

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	vgo "github.com/valkey-io/valkey-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFilterMembersByFilters(t *testing.T) {
	members := []vgo.ZScore{
		{Member: "nginx-frontend"},
		{Member: "api-server"},
		{Member: "NGINX-ingress"},
	}

	kept := filterMembersByGroups(members, []SearchFilterGroup{{Field: "name", Constraints: []SearchConstraint{{Value: "nginx"}}}}, indexShard{})

	assert.Len(t, kept, 2)
	assert.Equal(t, "nginx-frontend", kept[0].Member)
	assert.Equal(t, "NGINX-ingress", kept[1].Member)
}

func TestFilterMembersByFilters_EqualsAndAnd(t *testing.T) {
	members := []vgo.ZScore{{Member: "nginx"}, {Member: "nginx-frontend"}}

	// equals: exact case-insensitive match only
	kept := filterMembersByGroups(members, []SearchFilterGroup{
		{Field: "name", Constraints: []SearchConstraint{{Operator: SearchOperatorEquals, Value: "NGINX"}}},
	}, indexShard{})
	assert.Len(t, kept, 1)
	assert.Equal(t, "nginx", kept[0].Member)

	// AND within a group
	members = []vgo.ZScore{{Member: "nginx-frontend"}, {Member: "nginx-backend"}}
	kept = filterMembersByGroups(members, []SearchFilterGroup{
		{Field: "name", Constraints: []SearchConstraint{{Value: "nginx"}, {Value: "front"}}},
	}, indexShard{})
	assert.Len(t, kept, 1)
	assert.Equal(t, "nginx-frontend", kept[0].Member)

	// OR within a group
	members = []vgo.ZScore{{Member: "nginx-frontend"}, {Member: "api-server"}, {Member: "cache"}}
	kept = filterMembersByGroups(members, []SearchFilterGroup{
		{Field: "name", Operator: SearchGroupOperatorOr, Constraints: []SearchConstraint{{Value: "nginx"}, {Value: "api"}}},
	}, indexShard{})
	assert.Len(t, kept, 2)
}

func TestParseSearchQuery(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected SearchQuery
	}{
		{"plain term", "argo", SearchQuery{Term: "argo"}},
		{"trims and lowers", "  ArGo ", SearchQuery{Term: "argo"}},
		{"namespace scope", "namespace:argo", SearchQuery{Term: "argo", Field: "namespace"}},
		{"name scope", "name:nginx", SearchQuery{Term: "nginx", Field: "name"}},
		{"kind scope", "kind:Deployment", SearchQuery{Term: "deployment", Field: "kind"}},
		{"group alias", "group:apps", SearchQuery{Term: "apps", Field: "group"}},
		{"unknown prefix stays plain", "foo:bar", SearchQuery{Term: "foo:bar"}},
		{"negation prefix", "!argo", SearchQuery{Term: "argo", Negate: true}},
		{"negation with field scope", "!namespace:argo", SearchQuery{Term: "argo", Field: "namespace", Negate: true}},
		{"negation trims inner space", "! argo ", SearchQuery{Term: "argo", Negate: true}},
		{"lone bang is empty", "!", SearchQuery{}},
		{"empty", "  ", SearchQuery{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseSearchQuery(tt.input))
		})
	}
}

func TestSearchFilterGroupMatches_Scoped(t *testing.T) {
	group := func(field, op string, constraints ...SearchConstraint) SearchFilterGroup {
		return SearchFilterGroup{Field: field, Operator: op, Constraints: constraints}
	}

	// namespace-scoped must not match a resource whose NAME contains the term
	g := group("namespace", "", SearchConstraint{Value: "argo"})
	assert.True(t, g.Matches("api-server", "Deployment", "argocd", "apps/v1"))
	assert.False(t, g.Matches("argocd-server", "Deployment", "default", "apps/v1"))

	g = group("name", "", SearchConstraint{Value: "argo"})
	assert.True(t, g.Matches("argocd-server", "Deployment", "default", "apps/v1"))
	assert.False(t, g.Matches("api-server", "Deployment", "argocd", "apps/v1"))

	// equals: exact case-insensitive comparison
	g = group("namespace", "", SearchConstraint{Operator: SearchOperatorEquals, Value: "Argo"})
	assert.True(t, g.Matches("x", "Deployment", "argo", "apps/v1"))
	assert.False(t, g.Matches("x", "Deployment", "argocd", "apps/v1"))

	// or-joined constraints on one field
	g = group("name", SearchGroupOperatorOr, SearchConstraint{Value: "argo"}, SearchConstraint{Value: "nginx"})
	assert.True(t, g.Matches("nginx-frontend", "Deployment", "default", "apps/v1"))
	assert.True(t, g.Matches("argocd-server", "Deployment", "default", "apps/v1"))
	assert.False(t, g.Matches("api-server", "Deployment", "default", "apps/v1"))

	// and-joined constraints on one field
	g = group("name", "", SearchConstraint{Value: "argo"}, SearchConstraint{Value: "server"})
	assert.True(t, g.Matches("argocd-server", "Deployment", "default", "apps/v1"))
	assert.False(t, g.Matches("argocd-repo", "Deployment", "default", "apps/v1"))

	// unscoped matches any attribute
	g = group("", "", SearchConstraint{Value: "argo"})
	assert.True(t, g.Matches("argocd-server", "Deployment", "default", "apps/v1"))
	assert.True(t, g.Matches("api-server", "Deployment", "argocd", "apps/v1"))

	// notContains on a concrete field
	g = group("name", "", SearchConstraint{Operator: SearchOperatorNotContains, Value: "argo"})
	assert.True(t, g.Matches("api-server", "Deployment", "default", "apps/v1"))
	assert.False(t, g.Matches("argocd-server", "Deployment", "default", "apps/v1"))

	// notEquals on a concrete field
	g = group("namespace", "", SearchConstraint{Operator: SearchOperatorNotEquals, Value: "argocd"})
	assert.True(t, g.Matches("x", "Deployment", "default", "apps/v1"))
	assert.False(t, g.Matches("x", "Deployment", "argocd", "apps/v1"))

	// unscoped negation: matches only when NO attribute contains the term
	// (De Morgan of the positive any-attribute OR)
	g = group("", "", SearchConstraint{Operator: SearchOperatorNotContains, Value: "argo"})
	assert.True(t, g.Matches("api-server", "Deployment", "default", "apps/v1"))
	assert.False(t, g.Matches("api-server", "Deployment", "argocd", "apps/v1"))     // namespace contains argo
	assert.False(t, g.Matches("argocd-server", "Deployment", "default", "apps/v1")) // name contains argo
}

func TestShardSearchPlan(t *testing.T) {
	shard := indexShard{apiVersion: "apps/v1", kind: "Deployment", namespace: "argocd"}

	nameGroup := func(value string) SearchFilterGroup {
		return SearchFilterGroup{Field: "name", Constraints: []SearchConstraint{{Value: value}}}
	}
	nsGroup := func(value string) SearchFilterGroup {
		return SearchFilterGroup{Field: "namespace", Constraints: []SearchConstraint{{Value: value}}}
	}
	anyGroup := func(value string) SearchFilterGroup {
		return SearchFilterGroup{Constraints: []SearchConstraint{{Value: value}}}
	}

	// shard-scoped group passes -> drops out of the member plan
	memberGroups, excluded := shardSearchPlan(shard, []SearchFilterGroup{nsGroup("argo")})
	assert.False(t, excluded)
	assert.Empty(t, memberGroups)

	// shard-scoped group fails -> shard excluded entirely
	_, excluded = shardSearchPlan(shard, []SearchFilterGroup{nsGroup("prod")})
	assert.True(t, excluded)

	// name group stays a member group
	memberGroups, excluded = shardSearchPlan(shard, []SearchFilterGroup{nameGroup("argo")})
	assert.False(t, excluded)
	assert.Len(t, memberGroups, 1)

	// any-attribute groups always stay as member groups (resolved per member
	// against the shard's fixed attributes), never shortcut at shard level
	memberGroups, excluded = shardSearchPlan(shard, []SearchFilterGroup{anyGroup("argo")})
	assert.False(t, excluded)
	assert.Len(t, memberGroups, 1)
	assert.Equal(t, "", memberGroups[0].Field)

	// negated shard-scoped group: shard attribute violates notContains -> exclude
	nsNot := SearchFilterGroup{Field: "namespace", Constraints: []SearchConstraint{{Operator: SearchOperatorNotContains, Value: "argo"}}}
	_, excluded = shardSearchPlan(shard, []SearchFilterGroup{nsNot})
	assert.True(t, excluded)
}

func TestFilterMembersByGroups_AnyAttributeNegationUsesShard(t *testing.T) {
	// !argo (unscoped notContains) against a shard in the "argocd" namespace:
	// every member is excluded because the shard's namespace contains "argo".
	argocdShard := indexShard{apiVersion: "apps/v1", kind: "Deployment", namespace: "argocd"}
	members := []vgo.ZScore{{Member: "api-server"}, {Member: "cache"}}
	groups := []SearchFilterGroup{{Constraints: []SearchConstraint{{Operator: SearchOperatorNotContains, Value: "argo"}}}}
	assert.Empty(t, filterMembersByGroups(members, groups, argocdShard))

	// same filter against a neutral namespace: only members whose NAME contains
	// "argo" are dropped.
	defaultShard := indexShard{apiVersion: "apps/v1", kind: "Deployment", namespace: "default"}
	members = []vgo.ZScore{{Member: "argocd-server"}, {Member: "api-server"}}
	kept := filterMembersByGroups(members, groups, defaultShard)
	assert.Len(t, kept, 1)
	assert.Equal(t, "api-server", kept[0].Member)
}

func TestBuildSearchFilterGroups_Negation(t *testing.T) {
	// "!testname" free text -> single notContains constraint on any attribute
	groups := BuildSearchFilterGroups("!testname", nil, nil)
	assert.Len(t, groups, 1)
	assert.Equal(t, SearchFilterGroup{Field: "", Constraints: []SearchConstraint{{Operator: SearchOperatorNotContains, Value: "testname"}}}, groups[0])

	// "!namespace:argo" -> notContains scoped to namespace
	groups = BuildSearchFilterGroups("!namespace:argo", nil, nil)
	assert.Len(t, groups, 1)
	assert.Equal(t, SearchFilterGroup{Field: "namespace", Constraints: []SearchConstraint{{Operator: SearchOperatorNotContains, Value: "argo"}}}, groups[0])
}

func TestBuildSearchFilterGroups(t *testing.T) {
	groups := BuildSearchFilterGroups("namespace:argo",
		[]SearchFilter{
			{Field: "name", Value: "test"},
			{Field: "kind", Value: "  "}, // empty value dropped
		},
		[]SearchFilterGroup{
			{Field: "name", Operator: SearchGroupOperatorOr, Constraints: []SearchConstraint{{Value: "a"}, {Value: " "}}},
			{Field: "kind", Constraints: []SearchConstraint{{Value: "  "}}}, // all constraints empty -> dropped
		})

	assert.Len(t, groups, 3)
	assert.Equal(t, SearchFilterGroup{Field: "namespace", Constraints: []SearchConstraint{{Value: "argo"}}}, groups[0])
	assert.Equal(t, SearchFilterGroup{Field: "name", Constraints: []SearchConstraint{{Value: "test"}}}, groups[1])
	// empty constraint pruned, non-empty kept
	assert.Equal(t, SearchFilterGroup{Field: "name", Operator: SearchGroupOperatorOr, Constraints: []SearchConstraint{{Value: "a"}}}, groups[2])
}

func TestEnsureTypeMeta(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "web", "namespace": "myns"},
	}}
	ensureTypeMeta(obj, "apps/v1", "Deployment")
	assert.Equal(t, "apps/v1", obj.GetAPIVersion())
	assert.Equal(t, "Deployment", obj.GetKind())

	// populated values are never overwritten
	ensureTypeMeta(obj, "v1", "Pod")
	assert.Equal(t, "apps/v1", obj.GetAPIVersion())
	assert.Equal(t, "Deployment", obj.GetKind())

	// nil object and empty values are no-ops
	ensureTypeMeta(nil, "v1", "Pod")
	empty := &unstructured.Unstructured{Object: map[string]any{}}
	ensureTypeMeta(empty, "", "")
	assert.Equal(t, "", empty.GetAPIVersion())
	assert.Equal(t, "", empty.GetKind())
}

func TestStampTypeMetaFromKey(t *testing.T) {
	newObj := func(apiVersion, kind string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"metadata": map[string]any{"name": "web"},
		}}
		if apiVersion != "" {
			obj.SetAPIVersion(apiVersion)
		}
		if kind != "" {
			obj.SetKind(kind)
		}
		return obj
	}

	// empty TypeMeta is backfilled from the key segments
	obj := newObj("", "")
	stampTypeMetaFromKey(obj, "resources:apps/v1:Deployment:myns:web")
	assert.Equal(t, "apps/v1", obj.GetAPIVersion())
	assert.Equal(t, "Deployment", obj.GetKind())

	// RBAC path-segment names keep their colons in the name segment
	obj = newObj("", "")
	stampTypeMetaFromKey(obj, "resources:rbac.authorization.k8s.io/v1:Role:kube-system:system:controller:bootstrap-signer")
	assert.Equal(t, "rbac.authorization.k8s.io/v1", obj.GetAPIVersion())
	assert.Equal(t, "Role", obj.GetKind())

	// populated TypeMeta is left untouched
	obj = newObj("v1", "Pod")
	stampTypeMetaFromKey(obj, "resources:apps/v1:Deployment:myns:web")
	assert.Equal(t, "v1", obj.GetAPIVersion())
	assert.Equal(t, "Pod", obj.GetKind())

	// malformed key is a no-op
	obj = newObj("", "")
	stampTypeMetaFromKey(obj, "resources:v1:Pod")
	assert.Equal(t, "", obj.GetAPIVersion())
	assert.Equal(t, "", obj.GetKind())
}

// Reproduces the duplicate-page bug with bulk-created resources: many
// members share one creation second (one ZSET score). The shard pull
// (ZREVRANGE) returns equal-score members in REVERSE lexicographic order,
// so the merge sort must break score ties in the SAME direction — with an
// ascending tie-break, consecutive pages overlap and the lexicographically
// smaller members of the tie group become unreachable (MOG: mocli stuck at
// "loaded 50 of N").
func TestSortRankedMembers_DescTieBreakMatchesShardPull(t *testing.T) {
	// 130 members, one shared score — mirrors a load test creating
	// ~130 services within the same second (names not zero-padded).
	tieGroup := make([]string, 0, 130)
	for i := 871; i <= 1000; i++ {
		tieGroup = append(tieGroup, fmt.Sprintf("svc-load-%d", i))
	}
	// ZREVRANGE order for equal scores: reverse lexicographic.
	revLex := append([]string(nil), tieGroup...)
	sort.Sort(sort.Reverse(sort.StringSlice(revLex)))

	// page emulates GetResourcesByWhitelistPaginated for one shard:
	// pull the shard's top offset+limit members (ZREVRANGE prefix),
	// merge-sort them, slice [offset, offset+limit).
	page := func(offset, limit int) []string {
		count := min(offset+limit, len(revLex))
		pulled := make([]rankedMember, 0, count)
		for _, name := range revLex[:count] {
			pulled = append(pulled, rankedMember{member: name, score: 1785488352})
		}
		sortRankedMembers(pulled, false, sortOrderDesc)
		end := min(offset+limit, len(pulled))
		if offset >= len(pulled) {
			return nil
		}
		names := make([]string, 0, end-offset)
		for _, m := range pulled[offset:end] {
			names = append(names, m.member)
		}
		return names
	}

	pageOne := page(0, 50)
	pageTwo := page(50, 50)

	// Pages must be disjoint...
	seen := map[string]bool{}
	for _, name := range pageOne {
		seen[name] = true
	}
	for _, name := range pageTwo {
		assert.False(t, seen[name], "member %s returned on both pages", name)
		seen[name] = true
	}
	// ...and together cover exactly the shard's top 100 — no member of the
	// tie group may be skipped.
	assert.Len(t, seen, 100)
	for _, name := range revLex[:100] {
		assert.True(t, seen[name], "member %s unreachable via pagination", name)
	}
	// The page order must match the shard-pull order (newest-first
	// semantics: reverse-lex within one creation second).
	assert.Equal(t, revLex[:50], pageOne)
	assert.Equal(t, revLex[50:100], pageTwo)
}
