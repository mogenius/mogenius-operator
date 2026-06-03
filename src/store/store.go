package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"sort"
	"strconv"

	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
	vgo "github.com/valkey-io/valkey-go"

	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "k8s.io/api/apps/v1"
	v1batch "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

const (
	VALKEY_RESOURCE_PREFIX = "resources"

	// VALKEY_RESOURCE_INDEX_PREFIX is the root for ZSET-based pagination
	// indexes. One ZSET per (apiVersion, kind, namespace, sortType) holds
	// resource names so a paginated read can ZRANGE the page directly and
	// MGET only those names from the primary keys under VALKEY_RESOURCE_PREFIX.
	VALKEY_RESOURCE_INDEX_PREFIX = "resources-idx"

	// VALKEY_RESOURCE_INDEX_NS_PREFIX is the root for the per-(apiVersion, kind)
	// namespace registry: a SET whose members are the namespaces a kind lives
	// in. The cluster-wide paginated read reads this with one SMEMBERS per kind
	// instead of doing a full-keyspace SCAN to discover namespaces.
	VALKEY_RESOURCE_INDEX_NS_PREFIX = "resources-idx-ns"

	resourceIndexSortByCreation = "by-creation"
	resourceIndexSortByName     = "by-name"

	sortByName    = "name"
	sortOrderAsc  = "asc"
	sortOrderDesc = "desc"
)

var AuditLogLimit = int64(100)        // Default limit for audit log entries IMPORTANT: this is set per resource not globally
var AuditLogTTL = time.Hour * 24 * 14 // Default TTL for audit log entries (14 days)

// OnAuditLogCreated is called after an audit log entry is persisted.
// Set this callback to emit real-time events (e.g. via WebSocket).
var OnAuditLogCreated func(entry AuditLogEntry)

var ErrNotFound = errors.New("not found")

// KubernetesGetter is an interface for fetching secrets directly from Kubernetes cluster
type KubernetesGetter interface {
	GetSecret(namespace, name string) (*coreV1.Secret, error)
}

var valkeyClient valkeyclient.ValkeyClient

func Setup(
	logManagerModule logging.SlogManager,
	valkey valkeyclient.ValkeyClient,
	auditLogLimitStr string,
) error {
	valkeyClient = valkey
	auditLogLimit, _ := strconv.ParseInt(auditLogLimitStr, 10, 64)
	if auditLogLimit > 0 {
		AuditLogLimit = auditLogLimit
	}

	return nil
}

func SearchResourceByKeyParts(valkeyClient valkeyclient.ValkeyClient, parts ...string) ([]unstructured.Unstructured, error) {
	key := CreateResourceKey(parts...)

	items, err := valkeyclient.GetObjectsByPrefix[unstructured.Unstructured](valkeyClient, valkeyclient.ORDER_NONE, key)

	if len(items) == 0 {
		return nil, ErrNotFound

	}

	return items, err
}

func SearchByNamespaceAndName(valkeyClient valkeyclient.ValkeyClient, namespace string, name string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, &name)

	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, []string{})

	return items, err
}

func SearchByGroupKindNameNamespace(valkeyClient valkeyclient.ValkeyClient, apiVersion string, kind string, name string, namespace *string) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(&apiVersion, &kind, namespace, &name)

	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, []string{})

	return items, err
}

func SearchResourceByNamespace(valkeyClient valkeyclient.ValkeyClient, namespace string, whitelist []*utils.ResourceDescriptor) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, nil)

	var searchKeys []string
	if len(whitelist) > 0 {
		for _, item := range whitelist {
			searchKey := CreateResourceKey(item.ApiVersion, item.Kind, namespace)
			searchKeys = append(searchKeys, searchKey)
		}
	}

	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, searchKeys)

	return items, err
}

func DropAllResourcesFromValkey(valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) error {
	// Drop the primary keys together with the secondary pagination indexes and
	// the namespace registry. Dropping only "resources:*" would leave the
	// "resources-idx:*" ZSETs and "resources-idx-ns:*" SETs behind as orphans,
	// inflating paginated totalCounts after a restart. The three prefixes are
	// disjoint glob patterns ("resources:" matches neither "resources-idx:" nor
	// "resources-idx-ns:", and "resources-idx:" does not match the "-ns" form).
	err := valkeyClient.DeleteMultiple(
		VALKEY_RESOURCE_PREFIX+":*",
		VALKEY_RESOURCE_INDEX_PREFIX+":*",
		VALKEY_RESOURCE_INDEX_NS_PREFIX+":*",
	)
	if err != nil {
		logger.Error("failed to DropAllResourcesFromValkey", "error", err)
	}
	return err
}

func DropKey(valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger, key string) error {
	err := valkeyClient.DeleteMultiple(key)
	if err != nil {
		logger.Error("failed to DropKey", "error", err)
	}
	return err
}

func CreateResourceKey(parts ...string) string {
	parts = append([]string{VALKEY_RESOURCE_PREFIX}, parts...)
	return strings.Join(parts, ":")
}

func CreateKeyPattern(apiVersion, kind, namespace, name *string) string {
	parts := make([]string, 5)

	parts[0] = VALKEY_RESOURCE_PREFIX

	if apiVersion != nil && *apiVersion != "" {
		parts[1] = *apiVersion
	} else {
		parts[1] = "*"
	}

	if kind != nil && *kind != "" {
		parts[2] = *kind
	} else {
		parts[2] = "*"
	}

	if namespace != nil && *namespace != "" {
		parts[3] = *namespace
	} else {
		parts[3] = "*"
	}

	if name != nil && *name != "" {
		parts[4] = *name
	} else {
		parts[4] = "*"
	}

	pattern := strings.Join(parts, ":")
	return pattern
}

func GetResource(valkeyClient valkeyclient.ValkeyClient, apiVersion string, kind string, namespace string, name string, logger *slog.Logger) (*unstructured.Unstructured, error) {
	return valkeyclient.GetObjectForKey[unstructured.Unstructured](valkeyClient, VALKEY_RESOURCE_PREFIX, apiVersion, kind, namespace, name)
}

func GetResourceByKindAndNamespace(valkeyClient valkeyclient.ValkeyClient, apiVersion string, kind string, namespace string, logger *slog.Logger) []unstructured.Unstructured {
	pattern := CreateKeyPattern(&apiVersion, &kind, &namespace, nil)
	storeResults, err := valkeyclient.GetObjectsByPrefix[unstructured.Unstructured](valkeyClient, valkeyclient.ORDER_NONE, pattern)
	if err != nil {
		logger.Error("failed to get resources by kind and namespace", "apiVersion", apiVersion, "kind", kind, "namespace", namespace, "error", err)
		return []unstructured.Unstructured{}
	}

	results := make([]unstructured.Unstructured, 0, len(storeResults))
	hasNamespaceFilter := namespace != ""
	hasKindFilter := kind != ""

	for _, ref := range storeResults {
		// Skip only if filters are set AND don't match
		if (hasNamespaceFilter && ref.GetNamespace() != namespace) || (hasKindFilter && ref.GetKind() != kind) {
			continue
		}
		results = append(results, ref)
	}
	return results
}

// PaginatedResources is the return shape of GetResourcesByWhitelistPaginated:
// only the requested page of objects together with the total count behind the
// pagination cursor so the frontend can render "page X of Y".
type PaginatedResources struct {
	Items      []unstructured.Unstructured `json:"items"`
	TotalCount int                         `json:"totalCount"`
}

// resourceIndexKey returns the ZSET key that holds resource names for a given
// (apiVersion, kind, namespace, sortType) tuple. Members of the ZSET are
// resource names; scores are creationTimestamp.Unix() for the by-creation
// index and 0 for the by-name index (which is queried via ZRANGEBYLEX).
//
// namespace may be empty for cluster-scoped resources; the empty segment is
// preserved so the key shape matches the primary key under VALKEY_RESOURCE_PREFIX.
func resourceIndexKey(apiVersion, kind, namespace, sortType string) string {
	return strings.Join([]string{VALKEY_RESOURCE_INDEX_PREFIX, apiVersion, kind, namespace, sortType}, ":")
}

// SetResourceWithIndex writes a resource to the primary string key and updates
// both ZSET indexes (by-creation, by-name) in a single MULTI/EXEC transaction.
// All three writes succeed or all three are skipped, so a read via the index
// can never see a member whose primary key is missing (modulo TTL expiry, which
// the caller handles by ignoring empty MGET slots).
//
// apiVersion/kind/namespace/name are passed explicitly because Unstructured
// objects coming off the DynamicClient frequently have empty TypeMeta in
// .Object - the watcher already knows the typed values from its
// ResourceDescriptor and the primary key shape must match exactly.
//
// ttl controls both the primary key and the index keys; the indexes share the
// resource TTL so they age out together when the watcher stops refreshing.
func SetResourceWithIndex(
	valkey valkeyclient.ValkeyClient,
	apiVersion, kind, namespace, name string,
	obj *unstructured.Unstructured,
	ttl time.Duration,
) error {
	payload, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal resource for store: %w", err)
	}

	primaryKey := CreateResourceKey(apiVersion, kind, namespace, name)
	byCreationKey := resourceIndexKey(apiVersion, kind, namespace, resourceIndexSortByCreation)
	byNameKey := resourceIndexKey(apiVersion, kind, namespace, resourceIndexSortByName)
	nsRegistryKey := resourceNamespaceRegistryKey(apiVersion, kind)

	creationScore := float64(obj.GetCreationTimestamp().Time.Unix())
	ttlSeconds := int64(ttl.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = int64((time.Hour).Seconds())
	}

	// The SADD records that this kind lives in `namespace` so the cluster-wide
	// read path can discover namespaces via SMEMBERS instead of a keyspace SCAN.
	// namespace is "" for cluster-scoped resources; the empty member is kept so
	// the discovered shard matches the primary key shape exactly.
	client := valkey.GetValkeyClient()
	cmds := []vgo.Completed{
		client.B().Multi().Build(),
		client.B().Set().Key(primaryKey).Value(string(payload)).ExSeconds(ttlSeconds).Build(),
		client.B().Zadd().Key(byCreationKey).ScoreMember().ScoreMember(creationScore, name).Build(),
		client.B().Expire().Key(byCreationKey).Seconds(ttlSeconds).Build(),
		client.B().Zadd().Key(byNameKey).ScoreMember().ScoreMember(0, name).Build(),
		client.B().Expire().Key(byNameKey).Seconds(ttlSeconds).Build(),
		client.B().Sadd().Key(nsRegistryKey).Member(namespace).Build(),
		client.B().Expire().Key(nsRegistryKey).Seconds(ttlSeconds).Build(),
		client.B().Exec().Build(),
	}

	if err := checkMultiExec(client.DoMulti(valkey.GetContext(), cmds...)); err != nil {
		return fmt.Errorf("set resource with index pipeline: %w", err)
	}
	return nil
}

// DeleteResourceWithIndex removes the primary key and both index members in
// one MULTI/EXEC so a paginated read never resolves an index member to a
// missing key (again, modulo TTL expiry).
func DeleteResourceWithIndex(
	valkey valkeyclient.ValkeyClient,
	apiVersion, kind, namespace, name string,
) error {
	primaryKey := CreateResourceKey(apiVersion, kind, namespace, name)
	byCreationKey := resourceIndexKey(apiVersion, kind, namespace, resourceIndexSortByCreation)
	byNameKey := resourceIndexKey(apiVersion, kind, namespace, resourceIndexSortByName)

	// The namespace registry member is intentionally left untouched: we can't
	// tell from a single delete whether this was the last resource of the kind
	// in the namespace, and a stale namespace entry is harmless - it resolves
	// to an empty/expired ZSET shard (ZCARD 0) that the reader skips.
	client := valkey.GetValkeyClient()
	cmds := []vgo.Completed{
		client.B().Multi().Build(),
		client.B().Del().Key(primaryKey).Build(),
		client.B().Zrem().Key(byCreationKey).Member(name).Build(),
		client.B().Zrem().Key(byNameKey).Member(name).Build(),
		client.B().Exec().Build(),
	}

	if err := checkMultiExec(client.DoMulti(valkey.GetContext(), cmds...)); err != nil {
		return fmt.Errorf("delete resource with index pipeline: %w", err)
	}
	return nil
}

// checkMultiExec validates the responses of a MULTI ... EXEC pipeline sent via
// DoMulti. The per-response Error() check catches connection failures,
// queue-time command errors and EXECABORT (where the EXEC reply itself is an
// error). It additionally walks the EXEC reply array so per-command runtime
// errors (e.g. a WRONGTYPE that only surfaces during EXEC) are not silently
// swallowed - those are nested inside the array element, not on the array
// result itself.
func checkMultiExec(resps []vgo.ValkeyResult) error {
	for _, resp := range resps {
		if err := resp.Error(); err != nil {
			return err
		}
	}
	if len(resps) == 0 {
		return nil
	}
	// The last response is the EXEC reply: an array with one entry per queued
	// command. A nil reply means the transaction was discarded (only happens
	// with WATCH, which we don't use, so treat it as a failure).
	execResults, err := resps[len(resps)-1].ToArray()
	if err != nil {
		if errors.Is(err, vgo.Nil) {
			return fmt.Errorf("transaction discarded")
		}
		return err
	}
	for _, r := range execResults {
		if err := r.Error(); err != nil {
			return err
		}
	}
	return nil
}

// resourceNamespaceRegistryKey returns the SET key holding the namespaces a
// given (apiVersion, kind) currently lives in. It is sort-type independent
// because both indexes share the same namespace set.
func resourceNamespaceRegistryKey(apiVersion, kind string) string {
	return strings.Join([]string{VALKEY_RESOURCE_INDEX_NS_PREFIX, apiVersion, kind}, ":")
}

// indexShard identifies a single (apiVersion, kind, namespace) ZSET shard
// participating in a multi-kind/multi-namespace paginated read.
type indexShard struct {
	apiVersion string
	kind       string
	namespace  string
}

// rankedMember carries one ZSET member with the shard it came from and the
// score used to merge across shards. For sortBy==name the score is always 0
// and the merge compares Member strings instead.
type rankedMember struct {
	shard  indexShard
	member string
	score  float64
}

// GetResourcesByWhitelistPaginated paginates across every (apiVersion, kind,
// namespace) shard derived from whitelist + namespaceWhitelist. Each shard is
// one ZSET in the secondary index. The function reads up to (offset+limit)
// top members from each shard, merges them by sort key, slices the global
// page, and MGETs only that page from the primary keys.
//
// whitelist          - resource descriptors to include. Must contain at least
//
//	one entry; this path is intentionally narrower than
//	GetWorkspaceResources because pagination over "every
//	resource kind in the cluster" doesn't have a meaningful
//	ordering across heterogeneous kinds.
//
// blacklist          - descriptors to drop from whitelist (mirrors
//
//	GetWorkspaceResources for symmetry).
//
// namespaceWhitelist - namespaces to include per (apiVersion, kind). Empty
//
//	means "all namespaces": the function discovers them by
//	scanning index keys, which is cheap because there's
//	one key per namespace per kind, not one per resource.
//
// totalCount is the sum of ZCARDs across the matching shards. Stale members
// (primary key already expired but ZSET member still present) are filtered
// out of items via MGET nil responses; totalCount still includes them since
// the watcher's next resync overwrites the index.
func GetResourcesByWhitelistPaginated(
	valkey valkeyclient.ValkeyClient,
	whitelist []*utils.ResourceDescriptor,
	blacklist []*utils.ResourceDescriptor,
	namespaceWhitelist []string,
	offset, limit int,
	sortBy, sortOrder string,
	logger *slog.Logger,
) (PaginatedResources, error) {
	if offset < 0 {
		offset = 0
	}
	if len(whitelist) == 0 {
		return PaginatedResources{Items: []unstructured.Unstructured{}, TotalCount: 0}, nil
	}

	useNameSort := sortBy == sortByName
	sortType := resourceIndexSortByCreation
	if useNameSort {
		sortType = resourceIndexSortByName
	}

	shards, err := resolveIndexShards(valkey, whitelist, blacklist, namespaceWhitelist)
	if err != nil {
		logger.Error("failed to resolve index shards for paginated read", "error", err)
		return PaginatedResources{Items: []unstructured.Unstructured{}}, err
	}
	if len(shards) == 0 {
		return PaginatedResources{Items: []unstructured.Unstructured{}, TotalCount: 0}, nil
	}

	// How many members we need from each shard to guarantee the global page
	// can be filled: offset+limit is the worst case (one shard contributes
	// everything). limit<=0 means "no upper bound" - we have to pull the
	// whole shard.
	perShardCount := offset + limit
	pullAll := limit <= 0

	all := make([]rankedMember, 0)
	total := 0
	for _, shard := range shards {
		indexKey := resourceIndexKey(shard.apiVersion, shard.kind, shard.namespace, sortType)

		members, shardTotal, err := readShardTopMembers(valkey, indexKey, sortOrder, perShardCount, pullAll, useNameSort)
		if err != nil {
			logger.Warn("failed to read shard for paginated index",
				"indexKey", indexKey, "error", err)
			continue
		}
		total += shardTotal
		for _, m := range members {
			all = append(all, rankedMember{shard: shard, member: m.Member, score: m.Score})
		}
	}

	if len(all) == 0 {
		return PaginatedResources{Items: []unstructured.Unstructured{}, TotalCount: total}, nil
	}

	sortRankedMembers(all, useNameSort, sortOrder)

	if offset >= len(all) {
		return PaginatedResources{Items: []unstructured.Unstructured{}, TotalCount: total}, nil
	}
	end := offset + limit
	if pullAll || end > len(all) {
		end = len(all)
	}
	page := all[offset:end]

	keys := make([]string, 0, len(page))
	for _, rm := range page {
		keys = append(keys, CreateResourceKey(rm.shard.apiVersion, rm.shard.kind, rm.shard.namespace, rm.member))
	}

	client := valkey.GetValkeyClient()
	values, err := client.Do(valkey.GetContext(), client.B().Mget().Key(keys...).Build()).ToArray()
	if err != nil {
		logger.Error("failed to MGET paginated whitelist resources", "error", err)
		return PaginatedResources{Items: []unstructured.Unstructured{}, TotalCount: total}, err
	}

	items := make([]unstructured.Unstructured, 0, len(values))
	// Members whose primary key is gone (TTL expiry, or a delete event the
	// watcher missed) but that are still present in the ZSET index. They are
	// excluded from items below; we also prune them from the index and discount
	// them from total so totalCount converges instead of drifting upward.
	stale := make([]rankedMember, 0)
	for i, v := range values {
		raw, err := v.ToString()
		if err != nil {
			if errors.Is(err, vgo.Nil) {
				stale = append(stale, page[i])
				continue
			}
			logger.Warn("paginated MGET entry not readable", "key", keys[i], "error", err)
			continue
		}
		var obj unstructured.Unstructured
		if err := json.Unmarshal([]byte(raw), &obj); err != nil {
			logger.Warn("failed to unmarshal paginated resource", "key", keys[i], "error", err)
			continue
		}
		items = append(items, obj)
	}

	if len(stale) > 0 {
		pruneStaleIndexMembers(valkey, stale, logger)
		total -= len(stale)
		if total < 0 {
			total = 0
		}
	}

	return PaginatedResources{Items: items, TotalCount: total}, nil
}

// pruneStaleIndexMembers best-effort removes index members whose primary key no
// longer resolves, from both the by-creation and by-name ZSETs. This is lazy
// self-healing for delete events the watcher missed: without it such members
// linger forever in an actively-written shard (its ZSET TTL keeps being
// refreshed). There is a small race where the watcher re-creates the resource
// between the MGET miss and this ZREM; the next resync re-adds the member, so
// the worst case is one resync window of absence from the index. Failures are
// logged, never fatal - a stale member is a cosmetic count error, not data loss.
func pruneStaleIndexMembers(valkey valkeyclient.ValkeyClient, stale []rankedMember, logger *slog.Logger) {
	client := valkey.GetValkeyClient()
	cmds := make([]vgo.Completed, 0, len(stale)*2)
	for _, rm := range stale {
		byCreationKey := resourceIndexKey(rm.shard.apiVersion, rm.shard.kind, rm.shard.namespace, resourceIndexSortByCreation)
		byNameKey := resourceIndexKey(rm.shard.apiVersion, rm.shard.kind, rm.shard.namespace, resourceIndexSortByName)
		cmds = append(cmds,
			client.B().Zrem().Key(byCreationKey).Member(rm.member).Build(),
			client.B().Zrem().Key(byNameKey).Member(rm.member).Build(),
		)
	}
	for _, resp := range client.DoMulti(valkey.GetContext(), cmds...) {
		if err := resp.Error(); err != nil {
			logger.Warn("failed to prune stale index member", "error", err)
		}
	}
}

// resolveIndexShards expands (whitelist - blacklist) x namespaceWhitelist into
// concrete shards. When namespaceWhitelist is empty, it discovers namespaces
// per (apiVersion, kind) with a single SMEMBERS on the namespace registry SET
// maintained by SetResourceWithIndex - O(namespaces) and no full-keyspace SCAN.
func resolveIndexShards(
	valkey valkeyclient.ValkeyClient,
	whitelist []*utils.ResourceDescriptor,
	blacklist []*utils.ResourceDescriptor,
	namespaceWhitelist []string,
) ([]indexShard, error) {
	blacklisted := make(map[string]struct{}, len(blacklist))
	for _, b := range blacklist {
		if b == nil {
			continue
		}
		blacklisted[b.ApiVersion+"/"+b.Kind] = struct{}{}
	}

	client := valkey.GetValkeyClient()
	ctx := valkey.GetContext()

	shards := make([]indexShard, 0, len(whitelist))
	// Dedup so a duplicate (apiVersion,kind) in the whitelist or a repeated
	// namespace can't produce the same shard twice (which would double-count
	// totalCount and return the resource twice).
	seen := make(map[indexShard]struct{}, len(whitelist))
	addShard := func(s indexShard) {
		if _, dup := seen[s]; dup {
			return
		}
		seen[s] = struct{}{}
		shards = append(shards, s)
	}
	for _, w := range whitelist {
		if w == nil {
			continue
		}
		if _, skip := blacklisted[w.ApiVersion+"/"+w.Kind]; skip {
			continue
		}

		if len(namespaceWhitelist) > 0 {
			for _, ns := range namespaceWhitelist {
				addShard(indexShard{apiVersion: w.ApiVersion, kind: w.Kind, namespace: ns})
			}
			continue
		}

		// Cluster-wide: read the namespaces this kind lives in from its
		// registry SET. Stale namespaces (the kind no longer has resources
		// there) are harmless - they resolve to an empty ZSET the reader skips.
		nsRegistryKey := resourceNamespaceRegistryKey(w.ApiVersion, w.Kind)
		namespaces, err := client.Do(ctx, client.B().Smembers().Key(nsRegistryKey).Build()).AsStrSlice()
		if err != nil {
			return nil, fmt.Errorf("discover namespaces for %s/%s: %w", w.ApiVersion, w.Kind, err)
		}
		for _, ns := range namespaces {
			addShard(indexShard{apiVersion: w.ApiVersion, kind: w.Kind, namespace: ns})
		}
	}
	return shards, nil
}

// readShardTopMembers fetches the top members from a single shard together
// with their scores. count is the number of members to pull (offset+limit
// from the multi-shard caller). pullAll bypasses the count and reads the
// whole ZSET, needed when the outer call has no limit.
func readShardTopMembers(
	valkey valkeyclient.ValkeyClient,
	indexKey string,
	sortOrder string,
	count int,
	pullAll bool,
	useNameSort bool,
) ([]vgo.ZScore, int, error) {
	client := valkey.GetValkeyClient()
	ctx := valkey.GetContext()

	totalI64, err := client.Do(ctx, client.B().Zcard().Key(indexKey).Build()).AsInt64()
	if err != nil {
		return nil, 0, fmt.Errorf("zcard: %w", err)
	}
	total := int(totalI64)
	if total == 0 {
		return nil, 0, nil
	}
	if pullAll || count > total {
		count = total
	}
	if count <= 0 {
		return nil, total, nil
	}

	if useNameSort {
		// score is always 0 in the by-name index; we don't need WITHSCORES,
		// the merge will sort lex on Member. Use ZRANGEBYLEX with LIMIT to
		// only pull the slice we need from each shard.
		desc := sortOrder == sortOrderDesc
		var cmd vgo.Completed
		if desc {
			cmd = client.B().Zrevrangebylex().Key(indexKey).Max("+").Min("-").Limit(0, int64(count)).Build()
		} else {
			cmd = client.B().Zrangebylex().Key(indexKey).Min("-").Max("+").Limit(0, int64(count)).Build()
		}
		names, err := client.Do(ctx, cmd).AsStrSlice()
		if err != nil {
			return nil, total, fmt.Errorf("zrangebylex: %w", err)
		}
		out := make([]vgo.ZScore, 0, len(names))
		for _, n := range names {
			out = append(out, vgo.ZScore{Member: n, Score: 0})
		}
		return out, total, nil
	}

	// creationTimestamp index: default desc (newest first) when no order given.
	desc := sortOrder != sortOrderAsc
	var cmd vgo.Completed
	if desc {
		cmd = client.B().Zrevrange().Key(indexKey).Start(0).Stop(int64(count - 1)).Withscores().Build()
	} else {
		cmd = client.B().Zrange().Key(indexKey).Min("0").Max(strconv.Itoa(count - 1)).Withscores().Build()
	}
	scores, err := client.Do(ctx, cmd).AsZScores()
	if err != nil {
		return nil, total, fmt.Errorf("zrange withscores: %w", err)
	}
	return scores, total, nil
}

// sortRankedMembers orders the merged member list in place using the same
// rules as the single-shard paginated path: creationTimestamp defaults to
// desc, name to asc, with the resource name as a tiebreaker so the order is
// stable across requests.
func sortRankedMembers(items []rankedMember, useNameSort bool, sortOrder string) {
	if useNameSort {
		desc := sortOrder == sortOrderDesc
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].member == items[j].member {
				return false
			}
			if desc {
				return items[i].member > items[j].member
			}
			return items[i].member < items[j].member
		})
		return
	}

	desc := sortOrder != sortOrderAsc
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].member < items[j].member
		}
		if desc {
			return items[i].score > items[j].score
		}
		return items[i].score < items[j].score
	})
}

func GetIngressClasses() []networkingv1.IngressClass {
	ingressClasses, err := valkeyclient.GetObjectsByPrefix[networkingv1.IngressClass](valkeyClient, valkeyclient.ORDER_NONE, VALKEY_RESOURCE_PREFIX, utils.IngressClassResource.ApiVersion, utils.IngressClassResource.Kind, "*")
	if err != nil {
		return ingressClasses
	}
	return ingressClasses
}

func GetPod(namespace string, name string) *coreV1.Pod {
	pod, err := valkeyclient.GetObjectForKey[coreV1.Pod](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.PodResource.ApiVersion, utils.PodResource.Kind, namespace, name)
	if err != nil || pod == nil {
		return nil
	}
	return pod
}

func GetPods(namespace string) []coreV1.Pod {
	pods, err := valkeyclient.GetObjectsByPrefix[coreV1.Pod](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.PodResource.ApiVersion, utils.PodResource.Kind, namespace, "*")
	if err != nil || pods == nil {
		return nil
	}
	return pods
}

func GetReplicaset(namespace string, name string) *v1.ReplicaSet {
	replicaSet, err := valkeyclient.GetObjectForKey[v1.ReplicaSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.ReplicaSetResource.ApiVersion, utils.ReplicaSetResource.Kind, namespace, name)
	if err != nil || replicaSet == nil {
		return nil
	}
	return replicaSet
}

func GetDeployment(namespace string, name string) *v1.Deployment {
	deployment, err := valkeyclient.GetObjectForKey[v1.Deployment](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.DeploymentResource.ApiVersion, utils.DeploymentResource.Kind, namespace, name)
	if err != nil || deployment == nil {
		return nil
	}
	return deployment
}

func GetDeployments(namespace string, name string) []v1.Deployment {
	deployments, err := valkeyclient.GetObjectsByPrefix[v1.Deployment](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.DeploymentResource.ApiVersion, utils.DeploymentResource.Kind, namespace, name)
	if err != nil || deployments == nil {
		return nil
	}
	return deployments
}

func GetSecret(namespace string, name string) *coreV1.Secret {
	secret, err := valkeyclient.GetObjectForKey[coreV1.Secret](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.SecretResource.ApiVersion, utils.SecretResource.Kind, namespace, name)
	if err != nil || secret == nil {
		return nil
	}

	return secret
}

func GetSecrets(namespace string, name string) []coreV1.Secret {
	secrets, err := valkeyclient.GetObjectsByPrefix[coreV1.Secret](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.SecretResource.ApiVersion, utils.SecretResource.Kind, namespace, name)
	if err != nil || secrets == nil {
		return nil
	}
	return secrets
}

func GetService(namespace string, name string) *coreV1.Service {
	service, err := valkeyclient.GetObjectForKey[coreV1.Service](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.ServiceResource.ApiVersion, utils.ServiceResource.Kind, namespace, name)
	if err != nil || service == nil {
		return nil
	}

	return service
}

func GetServices(namespace string, name string) []coreV1.Service {
	services, err := valkeyclient.GetObjectsByPrefix[coreV1.Service](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.ServiceResource.ApiVersion, utils.ServiceResource.Kind, namespace, name)
	if err != nil || services == nil {
		return nil
	}
	return services
}

func GetStatefulSet(namespace string, name string) *v1.StatefulSet {
	statefulSet, err := valkeyclient.GetObjectForKey[v1.StatefulSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.StatefulSetResource.ApiVersion, utils.StatefulSetResource.Kind, namespace, name)
	if err != nil || statefulSet == nil {
		return nil
	}
	return statefulSet
}

func GetDaemonSet(namespace string, name string) *v1.DaemonSet {
	daemonSet, err := valkeyclient.GetObjectForKey[v1.DaemonSet](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.DaemonSetResource.ApiVersion, utils.DaemonSetResource.Kind, namespace, name)
	if err != nil || daemonSet == nil {
		return nil
	}
	return daemonSet
}

func GetJob(namespace string, name string) *v1batch.Job {
	job, err := valkeyclient.GetObjectForKey[v1batch.Job](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.JobResource.ApiVersion, utils.JobResource.Kind, namespace, name)
	if err != nil || job == nil {
		return nil
	}
	return job
}

func GetConfigMap(namespace string, name string) *coreV1.ConfigMap {
	configMap, err := valkeyclient.GetObjectForKey[coreV1.ConfigMap](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Kind, namespace, name)
	if err != nil || configMap == nil {
		return nil
	}

	return configMap
}

func GetCronJob(namespace string, name string) *v1batch.CronJob {
	cronJob, err := valkeyclient.GetObjectForKey[v1batch.CronJob](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.CronJobResource.ApiVersion, utils.CronJobResource.Kind, namespace, name)
	if err != nil || cronJob == nil {
		return nil
	}
	return cronJob
}

func GetNode(name string) *coreV1.Node {
	node, err := valkeyclient.GetObjectForKey[coreV1.Node](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.NodeResource.ApiVersion, utils.NodeResource.Kind, "", name)
	if err != nil || node == nil {
		return nil
	}
	return node
}

func GetNodes() []coreV1.Node {
	nodes, err := valkeyclient.GetObjectsByPrefix[coreV1.Node](valkeyClient, valkeyclient.ORDER_ASC, VALKEY_RESOURCE_PREFIX, utils.NodeResource.ApiVersion, utils.NodeResource.Kind, "", "*")
	if err != nil {
		return nil
	}

	return nodes
}

func DeleteNode(name string) error {
	return valkeyClient.DeleteSingle(VALKEY_RESOURCE_PREFIX, utils.NodeResource.ApiVersion, utils.NodeResource.Kind, "", name)
}

func GetAllGrants(namespace string) ([]v1alpha1.Grant, error) {
	pattern := CreateKeyPattern(&utils.GrantResource.ApiVersion, &utils.GrantResource.Kind, &namespace, nil)
	grants, err := valkeyclient.GetObjectsByPrefix[v1alpha1.Grant](valkeyClient, valkeyclient.ORDER_ASC, pattern)
	if err != nil || grants == nil {
		return nil, err
	}
	return grants, nil
}

func GetGrant(namespace string, name string) (*v1alpha1.Grant, error) {
	grant, err := valkeyclient.GetObjectForKey[v1alpha1.Grant](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.GrantResource.ApiVersion, utils.GrantResource.Kind, namespace, name)
	if err != nil || grant == nil {
		return nil, err
	}
	return grant, nil
}

func GetAllUsers(namespace string) ([]v1alpha1.User, error) {
	pattern := CreateKeyPattern(&utils.UserResource.ApiVersion, &utils.UserResource.Kind, &namespace, nil)
	users, err := valkeyclient.GetObjectsByPrefix[v1alpha1.User](valkeyClient, valkeyclient.ORDER_ASC, pattern)
	if err != nil || users == nil {
		return nil, err
	}
	return users, nil
}

func GetUser(namespace string, name string) (*v1alpha1.User, error) {
	user, err := valkeyclient.GetObjectForKey[v1alpha1.User](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.UserResource.ApiVersion, utils.UserResource.Kind, namespace, name)
	if err != nil || user == nil {
		return nil, err
	}
	return user, nil
}

func GetAllWorkspaces(namespace string) ([]v1alpha1.Workspace, error) {
	pattern := CreateKeyPattern(&utils.WorkspaceResource.ApiVersion, &utils.WorkspaceResource.Kind, &namespace, nil)
	workspaces, err := valkeyclient.GetObjectsByPrefix[v1alpha1.Workspace](valkeyClient, valkeyclient.ORDER_ASC, pattern)
	if err != nil || workspaces == nil {
		return nil, err
	}
	return workspaces, nil
}

func GetWorkspace(namespace string, name string) (*v1alpha1.Workspace, error) {
	workspace, err := valkeyclient.GetObjectForKey[v1alpha1.Workspace](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.WorkspaceResource.ApiVersion, utils.WorkspaceResource.Kind, namespace, name)
	if err != nil || workspace == nil {
		return nil, err
	}
	return workspace, nil
}

func GetYamlFromUnstructuredResource(obj *unstructured.Unstructured) (string, error) {
	cleanedObj := removeUnusedFields(obj)
	jsonData, err := cleanedObj.MarshalJSON()
	if err != nil {
		return "", err
	}
	yamlData, err := yaml.JSONToYAML(jsonData)
	if err != nil {
		return "", err
	}
	return string(yamlData), nil
}

// Audit Log
type AuditLogEntry struct {
	Pattern   string       `json:"pattern" validate:"required"`
	Payload   any          `json:"payload,omitempty"`
	Diff      string       `json:"diff,omitempty"`
	Result    any          `json:"result,omitempty"`
	Error     string       `json:"error,omitempty"`
	CreatedAt time.Time    `json:"createdAt"`
	User      structs.User `json:"user"`
	Workspace string       `json:"workspace,omitempty"`
}

type AuditLogResponse struct {
	Data       []AuditLogEntry `json:"data"`
	TotalCount int             `json:"totalCount"`
}

func AddToAuditLog[T any](datagram structs.Datagram, logger *slog.Logger, result T, err error, oldObj *unstructured.Unstructured, updatedObj *unstructured.Unstructured) (T, error) {
	resourceNamespace := ""
	resourceName := ""

	auditLogEntry := auditLogFromDatagram(datagram, result, err)
	if oldObj != nil || updatedObj != nil {
		patch, diffErr := Diff(oldObj, updatedObj)
		if diffErr != nil {
			logger.Error("failed to create kubectl style diff", "error", diffErr)
			return result, err
		}
		auditLogEntry.Diff = patch
	}
	if oldObj != nil {
		resourceNamespace = oldObj.GetNamespace()
		resourceName = oldObj.GetName()
	} else if updatedObj != nil {
		resourceNamespace = updatedObj.GetNamespace()
		resourceName = updatedObj.GetName()
	} else if payload, ok := datagram.Payload.(map[string]any); ok {
		if ns, ok := payload["namespace"].(string); ok {
			resourceNamespace = ns
		}
		if name, ok := payload["name"].(string); ok {
			resourceName = name
		}
		if pod, ok := payload["pod"].(string); ok {
			resourceName = pod
		}
	} else if yamlData, ok := payload["yamlData"].(string); ok {
		var unstruct unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yamlData), &unstruct)
		if err == nil {
			resourceNamespace = unstruct.GetNamespace()
			resourceName = unstruct.GetName()
		} else {
			return result, fmt.Errorf("failed to guess Namespace and ResourceName from datagram payload: %w", err)
		}
	}

	auditLogAddErr := valkeyClient.SetObjectWithAutoincrementLimit(auditLogEntry, AuditLogLimit, AuditLogTTL, "audit-log", resourceNamespace, resourceName)
	if auditLogAddErr != nil {
		logger.Error("failed to add to audit log", "error", auditLogAddErr)
	} else if OnAuditLogCreated != nil {
		go OnAuditLogCreated(auditLogEntry)
	}
	return result, err
}

func ListAuditLog(limit int, offset int, namespaces []string, clusterWide bool, search string) ([]AuditLogEntry, int, error) {
	if limit <= 0 {
		limit = 100
	}

	// Load ALL entries (no pagination yet) so we can sort by CreatedAt before paginating.
	// GetObjectsByPrefixWithSizeAndNs applies offset/limit on unsorted SCAN keys,
	// which can cause newer entries to be missed.
	const maxEntries = 10000
	allEntries, _, err := valkeyclient.GetObjectsByPrefixWithSizeAndNs[AuditLogEntry](valkeyClient, maxEntries, 0, namespaces, clusterWide, "audit-log")
	if err != nil {
		return []AuditLogEntry{}, 0, err
	}

	// Filter by search term (case-insensitive) across key fields
	if search != "" {
		searchLower := strings.ToLower(search)
		filtered := make([]AuditLogEntry, 0, len(allEntries))
		for _, entry := range allEntries {
			if auditLogEntryMatchesSearch(entry, searchLower) {
				filtered = append(filtered, entry)
			}
		}
		allEntries = filtered
	}

	totalCount := len(allEntries)

	// Sort by CreatedAt descending (newest first)
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].CreatedAt.After(allEntries[j].CreatedAt)
	})

	// Apply pagination
	if offset >= totalCount {
		return []AuditLogEntry{}, totalCount, nil
	}
	end := min(offset+limit, totalCount)

	return allEntries[offset:end], totalCount, nil
}

func auditLogEntryMatchesSearch(entry AuditLogEntry, searchLower string) bool {
	if strings.Contains(strings.ToLower(entry.Pattern), searchLower) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Workspace), searchLower) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.User.FirstName), searchLower) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.User.LastName), searchLower) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.User.Email), searchLower) {
		return true
	}
	if payload, ok := entry.Payload.(map[string]any); ok {
		for _, key := range []string{"name", "targetName", "kind", "namespace"} {
			if val, ok := payload[key].(string); ok && strings.Contains(strings.ToLower(val), searchLower) {
				return true
			}
		}
	}
	return false
}

func auditLogFromDatagram(datagram structs.Datagram, result any, err error) AuditLogEntry {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return AuditLogEntry{
		Pattern:   datagram.Pattern,
		Payload:   datagram.Payload,
		CreatedAt: datagram.CreatedAt,
		User:      datagram.User,
		Workspace: datagram.Workspace,
		Error:     errStr,
		Result:    result,
	}
}

// AddAiChatAuditLog writes an audit log entry for AI chat interactions (messages and tool uses).
// Keys follow the pattern audit-log:ai-chat:<user-email>:<num>.
// These entries are always included in ListAuditLog results regardless of namespace filter.
func AddAiChatAuditLog(logger *slog.Logger, pattern string, payload any, result any, errStr string, user structs.User, workspace string) {
	entry := AuditLogEntry{
		Pattern:   pattern,
		Payload:   payload,
		Result:    result,
		Error:     errStr,
		CreatedAt: time.Now(),
		User:      user,
		Workspace: workspace,
	}

	storeErr := valkeyClient.SetObjectWithAutoincrementLimit(entry, AuditLogLimit, AuditLogTTL, "audit-log", "ai-chat", user.Email)
	if storeErr != nil {
		logger.Error("failed to add AI chat audit log", "error", storeErr)
	} else if OnAuditLogCreated != nil {
		go OnAuditLogCreated(entry)
	}
}

func Diff(oldObj, newObj *unstructured.Unstructured) (string, error) {
	modified := []byte{}
	original := []byte{}
	err := error(nil)
	ns := ""
	resourceName := ""

	if oldObj != nil {
		ns = oldObj.GetNamespace()
		resourceName = oldObj.GetName()
	} else if newObj != nil {
		ns = newObj.GetNamespace()
		resourceName = newObj.GetName()
	} else {
		return "", fmt.Errorf("both oldObj and newObj are nil, cannot create diff")
	}

	if oldObj != nil {
		oldObj = removeUnusedFields(oldObj)
		original, err = yaml.Marshal(oldObj.Object)
		if err != nil {
			return "", fmt.Errorf("failed to marshal original data: %w", err)
		}
	}

	if newObj != nil {
		newObj = removeUnusedFields(newObj)
		modified, err = yaml.Marshal(newObj.Object)
		if err != nil {
			return "", fmt.Errorf("failed to marshal modified data: %w", err)
		}
	}

	diff, err := unifiedDiff(original, modified, ns, resourceName)
	if err != nil {
		return "", fmt.Errorf("failed to create unified diff: %w", err)
	}
	return diff, nil
}

// unifiedDiff computes a unified diff between two YAML payloads. It used
// to round-trip through /tmp/original.yaml and /tmp/modified.yaml with
// fixed names, which raced badly under concurrent audit-log writes:
// writer A's bytes could be overwritten by writer B's before A's reader
// saw them, producing audit entries with the wrong diff. The bytes are
// already in memory; difflib doesn't need files.
func unifiedDiff(a, b []byte, ns, resourceName string) (string, error) {
	label := ns + "/" + resourceName
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(a)),
		B:        difflib.SplitLines(string(b)),
		FromFile: label,
		ToFile:   label,
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(diff)
}

func removeUnusedFields(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil {
		return obj
	}

	obj.SetManagedFields(nil)
	unstructured.RemoveNestedField(obj.Object, "status")
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(obj.Object, "metadata", "generation")
	unstructured.RemoveNestedField(obj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj.Object, "metadata", "uid")
	unstructured.RemoveNestedField(obj.Object, "metadata", "creationTimestamp")

	return obj
}
