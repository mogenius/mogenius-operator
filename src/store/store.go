package store

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/logging"
	moMetrics "mogenius-operator/src/metrics"
	"mogenius-operator/src/secrets"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"sort"
	"strconv"
	"sync/atomic"

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

	// VALKEY_RESOURCE_INDEX_NODE_PREFIX is the root for the per-node pod
	// index: a SET per node whose members are the primary keys of the pods
	// scheduled there. The node-metrics DaemonSet reads its own node's pods
	// every few seconds; without this index each read scanned and
	// deserialized every pod in the cluster on every node.
	VALKEY_RESOURCE_INDEX_NODE_PREFIX = "resources-idx-node"

	resourceIndexSortByCreation = "by-creation"
	resourceIndexSortByName     = "by-name"

	sortByName    = "name"
	sortOrderAsc  = "asc"
	sortOrderDesc = "desc"
)

var AuditLogLimit = int64(100)        // Default limit for audit log entries IMPORTANT: this is set per resource not globally
var AuditLogTTL = time.Hour * 24 * 14 // Default TTL for audit log entries (14 days), override via MO_AUDIT_LOG_TTL

// OnAuditLogCreated is called after an audit log entry is persisted.
// Set this callback to emit real-time events (e.g. via WebSocket). It is
// invoked from a single dispatcher goroutine, so implementations may block
// briefly and entries arrive in write order.
var OnAuditLogCreated func(entry AuditLogEntry)

const (
	// auditLogIndexKey is a ZSET over all audit log entry keys with
	// score = CreatedAt in unix milliseconds. It lets ListAuditLog serve
	// time-ordered pages without the full-keyspace SCAN + bulk MGET the
	// key-prefix layout would otherwise require. The "idx:" prefix keeps
	// these keys out of the "audit-log*" prefix scans (legacy list path).
	auditLogIndexKey = "idx:audit-log"
	// auditLogIndexReadyKey marks that the one-time backfill of
	// pre-index entries into the ZSET has completed.
	auditLogIndexReadyKey = "idx:audit-log:ready"

	auditEventQueueSize = 256
)

var (
	auditLogIndexEnsured atomic.Bool

	// Real-time event dispatch: a single worker drains this queue and
	// calls OnAuditLogCreated, replacing the per-entry fire-and-forget
	// goroutines (unbounded, unordered, no shutdown path). When the queue
	// is full the event is dropped and counted — the persisted entry is
	// unaffected.
	auditEventQueue chan AuditLogEntry
	auditEventQuit  chan struct{}
	auditEventSeq   atomic.Int64
	// auditBootId identifies this process run in pushed events. Seq is
	// monotonic per boot; together they let the platform detect gaps in
	// the event stream (and restarts) and resync via audit-log/list.
	auditBootId string

	auditLogger *slog.Logger
)

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
	auditLogTTLStr string,
) error {
	valkeyClient = valkey
	auditLogger = logManagerModule.CreateLogger("audit-log")
	auditLogLimit, _ := strconv.ParseInt(auditLogLimitStr, 10, 64)
	if auditLogLimit > 0 {
		AuditLogLimit = auditLogLimit
	}
	if auditLogTTLStr != "" {
		ttl, err := time.ParseDuration(auditLogTTLStr)
		if err != nil {
			return fmt.Errorf("invalid audit log TTL %q: %w", auditLogTTLStr, err)
		}
		if ttl > 0 {
			AuditLogTTL = ttl
		}
	}

	startAuditEventDispatcher()

	return nil
}

// startAuditEventDispatcher launches the single worker that forwards
// persisted audit entries to OnAuditLogCreated in write order. The quit
// channel (closed via shutdown hook) defines its lifetime; after shutdown,
// remaining events are dropped and counted instead of leaking goroutines.
func startAuditEventDispatcher() {
	auditEventQueue = make(chan AuditLogEntry, auditEventQueueSize)
	auditEventQuit = make(chan struct{})
	auditBootId = utils.NanoId()

	go func() {
		for {
			select {
			case entry := <-auditEventQueue:
				if cb := OnAuditLogCreated; cb != nil {
					entry.Seq = auditEventSeq.Add(1)
					entry.BootId = auditBootId
					cb(entry)
				}
			case <-auditEventQuit:
				return
			}
		}
	}()

	shutdown.Add(func() {
		close(auditEventQuit)
	})
}

// dispatchAuditEvent hands a persisted entry to the dispatcher without
// blocking the caller. Drops (full queue or shutdown) only affect the
// real-time push — the entry itself is already stored.
func dispatchAuditEvent(entry AuditLogEntry) {
	select {
	case auditEventQueue <- entry:
	default:
		moMetrics.IncAuditLogEventDropped()
		if auditLogger != nil {
			auditLogger.Warn("audit event queue full, dropping real-time event", "pattern", entry.Pattern)
		}
	}
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
	// Non-empty whitelist: filter by apiVersion/kind parsed from the key
	// segments. The previous implementation passed prefix keys (without the
	// name segment) into GetObjectsByPattern's EXACT-match keyword filter,
	// so a whitelisted search always returned zero results.
	if len(whitelist) > 0 {
		allowed := make([]utils.ResourceDescriptor, 0, len(whitelist))
		for _, item := range whitelist {
			if item != nil {
				allowed = append(allowed, *item)
			}
		}
		return GetResourcesByNamespaceAndKinds(valkeyClient, namespace, allowed)
	}

	pattern := CreateKeyPattern(nil, nil, &namespace, nil)
	items, err := valkeyclient.GetObjectsByPattern[unstructured.Unstructured](valkeyClient, pattern, nil)

	return items, err
}

// GetResourcesByNamespaceAndKinds returns all stored resources in the given
// namespace whose apiVersion/kind matches one of the allowed descriptors,
// using a SINGLE keyspace scan plus chunked MGETs. The per-kind alternative
// (one scan per descriptor) costs O(kinds × keyspace) — with 80-150 watched
// kinds that dominated namespace-scoped queries. apiVersion/kind are derived
// from the key segments, which are written from the watcher's
// ResourceDescriptor and therefore authoritative even when the stored
// object's TypeMeta is empty.
func GetResourcesByNamespaceAndKinds(valkeyClient valkeyclient.ValkeyClient, namespace string, allowed []utils.ResourceDescriptor) ([]unstructured.Unstructured, error) {
	pattern := CreateKeyPattern(nil, nil, &namespace, nil)
	keys, err := valkeyClient.Keys(pattern)
	if err != nil {
		return []unstructured.Unstructured{}, fmt.Errorf("scan resources in namespace %q: %w", namespace, err)
	}

	allowedSet := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r.ApiVersion+":"+r.Kind] = struct{}{}
	}

	selected := make([]string, 0, len(keys))
	for _, key := range keys {
		if resourceKeyMatches(key, namespace, allowedSet) {
			selected = append(selected, key)
		}
	}

	results, err := valkeyclient.GetObjectsForKeys[unstructured.Unstructured](valkeyClient, selected)
	if err != nil {
		return results, fmt.Errorf("fetch resources in namespace %q: %w", namespace, err)
	}
	return results, nil
}

// resourceKeyMatches reports whether a primary key belongs to the given
// namespace and one of the allowed apiVersion:kind pairs.
//
// Key layout: resources:<apiVersion>:<kind>:<namespace>:<name>.
// apiVersion/kind/namespace cannot contain ':' (apiVersion uses '/'), but
// the NAME can: RBAC path-segment names like
// "system:controller:bootstrap-signer" exist in every cluster, so SplitN
// keeps those colons inside parts[4]. The namespace re-check guards against
// glob wildcards matching across segment boundaries. An empty namespace
// disables the namespace check (cluster-wide query).
func resourceKeyMatches(key, namespace string, allowedSet map[string]struct{}) bool {
	parts := strings.SplitN(key, ":", 5)
	if len(parts) != 5 {
		return false
	}
	if namespace != "" && parts[3] != namespace {
		return false
	}
	_, ok := allowedSet[parts[1]+":"+parts[2]]
	return ok
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
		VALKEY_RESOURCE_INDEX_NODE_PREFIX+":*",
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
	}
	if nodeName := podNodeName(apiVersion, kind, obj); nodeName != "" {
		nodeIndexKey := resourceNodeIndexKey(nodeName)
		cmds = append(cmds,
			client.B().Sadd().Key(nodeIndexKey).Member(primaryKey).Build(),
			client.B().Expire().Key(nodeIndexKey).Seconds(ttlSeconds).Build(),
		)
	}
	cmds = append(cmds, client.B().Exec().Build())

	if err := checkMultiExec(client.DoMulti(valkey.GetContext(), cmds...)); err != nil {
		return fmt.Errorf("set resource with index pipeline: %w", err)
	}
	return nil
}

// podNodeName returns spec.nodeName when obj is a scheduled Pod, "" otherwise.
func podNodeName(apiVersion, kind string, obj *unstructured.Unstructured) string {
	if obj == nil || kind != "Pod" || apiVersion != "v1" {
		return ""
	}
	nodeName, _, _ := unstructured.NestedString(obj.Object, "spec", "nodeName")
	return nodeName
}

// resourceNodeIndexKey returns the SET key holding the primary keys of the
// pods scheduled on the given node.
func resourceNodeIndexKey(nodeName string) string {
	return VALKEY_RESOURCE_INDEX_NODE_PREFIX + ":" + nodeName
}

// DeleteResourceWithIndex removes the primary key and both index members in
// one MULTI/EXEC so a paginated read never resolves an index member to a
// missing key (again, modulo TTL expiry).
func DeleteResourceWithIndex(
	valkey valkeyclient.ValkeyClient,
	apiVersion, kind, namespace, name string,
	obj *unstructured.Unstructured,
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
	}
	if nodeName := podNodeName(apiVersion, kind, obj); nodeName != "" {
		cmds = append(cmds, client.B().Srem().Key(resourceNodeIndexKey(nodeName)).Member(primaryKey).Build())
	}
	cmds = append(cmds, client.B().Exec().Build())

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

func GetWorkspaceDashboard(namespace string, name string) (*v1alpha1.WorkspaceDashboard, error) {
	dashboard, err := valkeyclient.GetObjectForKey[v1alpha1.WorkspaceDashboard](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.WorkspaceDashboardResource.ApiVersion, utils.WorkspaceDashboardResource.Kind, namespace, name)
	if err != nil || dashboard == nil {
		return nil, err
	}
	return dashboard, nil
}

// GetPodsOnNode returns the pods scheduled on nodeName via the per-node SET
// index (one SMEMBERS plus chunked MGETs). The full-scan fallback covers an
// empty/missing index (e.g. right after the store was wiped, before the
// watcher re-populated it). Members whose primary key already expired are
// skipped by GetObjectsForKeys.
func GetPodsOnNode(nodeName string) []coreV1.Pod {
	client := valkeyClient.GetValkeyClient()
	members, err := client.Do(valkeyClient.GetContext(), client.B().Smembers().Key(resourceNodeIndexKey(nodeName)).Build()).AsStrSlice()
	if err == nil && len(members) > 0 {
		pods, err := valkeyclient.GetObjectsForKeys[coreV1.Pod](valkeyClient, members)
		if err == nil {
			return pods
		}
	}

	pods := GetPods("*")
	result := make([]coreV1.Pod, 0, len(pods))
	for _, pod := range pods {
		if pod.Spec.NodeName == nodeName {
			result = append(result, pod)
		}
	}
	return result
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

func GetAllAgents(namespace string) ([]v1alpha1.Agent, error) {
	pattern := CreateKeyPattern(&utils.AgentResource.ApiVersion, &utils.AgentResource.Kind, &namespace, nil)
	agents, err := valkeyclient.GetObjectsByPrefix[v1alpha1.Agent](valkeyClient, valkeyclient.ORDER_ASC, pattern)
	if err != nil || agents == nil {
		return nil, err
	}
	return agents, nil
}

func GetAgent(namespace string, name string) (*v1alpha1.Agent, error) {
	agent, err := valkeyclient.GetObjectForKey[v1alpha1.Agent](valkeyClient, VALKEY_RESOURCE_PREFIX, utils.AgentResource.ApiVersion, utils.AgentResource.Kind, namespace, name)
	if err != nil || agent == nil {
		return nil, err
	}
	return agent, nil
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
	RequestId  string       `json:"requestId,omitempty"`
	Pattern    string       `json:"pattern" validate:"required"`
	Kind       string       `json:"kind,omitempty"`
	ApiVersion string       `json:"apiVersion,omitempty"`
	Namespace  string       `json:"namespace,omitempty"`
	Name       string       `json:"name,omitempty"`
	Success    bool         `json:"success"`
	Payload    any          `json:"payload,omitempty"`
	Diff       string       `json:"diff,omitempty"`
	Result     any          `json:"result,omitempty"`
	Error      string       `json:"error,omitempty"`
	CreatedAt  time.Time    `json:"createdAt"`
	User       structs.User `json:"user"`
	Workspace  string       `json:"workspace,omitempty"`

	// Seq and BootId are stamped only on the copy pushed as a real-time
	// AuditLogEvent (never persisted): Seq is monotonic per process run,
	// BootId identifies the run. A consumer that sees a gap in Seq for the
	// same BootId knows it missed events and can resync via audit-log/list.
	Seq    int64  `json:"seq,omitempty"`
	BootId string `json:"bootId,omitempty"`
}

type AuditLogResponse struct {
	Data       []AuditLogEntry `json:"data"`
	TotalCount int             `json:"totalCount"`
}

// auditLogFallbackBucket is the namespace segment of the Valkey key for
// entries that cannot be attributed to a concrete resource. Grouping them
// by pattern (instead of the shared "audit-log:::" bucket) keeps the
// per-resource entry limit from being drained by unrelated actions.
const auditLogFallbackBucket = "_cluster"

func AddToAuditLog[T any](datagram structs.Datagram, logger *slog.Logger, result T, err error, oldObj *unstructured.Unstructured, updatedObj *unstructured.Unstructured) (T, error) {
	auditLogEntry := auditLogFromDatagram(datagram, result, err)

	// Never persist secret values: replace Secret data with hashed
	// placeholders before diffing and before the entry is stored/pushed.
	oldObj = redactSecretData(oldObj)
	updatedObj = redactSecretData(updatedObj)

	if oldObj != nil || updatedObj != nil {
		patch, diffErr := Diff(oldObj, updatedObj)
		if diffErr != nil {
			// Still persist the entry — a missing diff must not suppress the audit trail.
			logger.Error("failed to create kubectl style diff", "error", diffErr)
		} else {
			auditLogEntry.Diff = patch
		}
	}

	if ref := updatedObj; ref != nil || oldObj != nil {
		if oldObj != nil {
			ref = oldObj
		}
		auditLogEntry.Namespace = ref.GetNamespace()
		auditLogEntry.Name = ref.GetName()
		auditLogEntry.Kind = ref.GetKind()
		auditLogEntry.ApiVersion = ref.GetAPIVersion()
	} else if payload := datagram.PayloadMap(); payload != nil {
		if ns, ok := payload["namespace"].(string); ok {
			auditLogEntry.Namespace = ns
		}
		if name, ok := payload["name"].(string); ok {
			auditLogEntry.Name = name
		}
		if pod, ok := payload["pod"].(string); ok {
			auditLogEntry.Name = pod
		}
		if kind, ok := payload["kind"].(string); ok {
			auditLogEntry.Kind = kind
		}
		if apiVersion, ok := payload["apiVersion"].(string); ok {
			auditLogEntry.ApiVersion = apiVersion
		}
		if auditLogEntry.Namespace == "" || auditLogEntry.Name == "" {
			if yamlData, ok := payload["yamlData"].(string); ok {
				var unstruct unstructured.Unstructured
				if yaml.Unmarshal([]byte(yamlData), &unstruct) == nil {
					if auditLogEntry.Namespace == "" {
						auditLogEntry.Namespace = unstruct.GetNamespace()
					}
					if auditLogEntry.Name == "" {
						auditLogEntry.Name = unstruct.GetName()
					}
					if auditLogEntry.Kind == "" {
						auditLogEntry.Kind = unstruct.GetKind()
					}
					if auditLogEntry.ApiVersion == "" {
						auditLogEntry.ApiVersion = unstruct.GetAPIVersion()
					}
				}
			}
		}
	}

	sanitizeAuditLogEntry(&auditLogEntry)

	bucketNamespace := auditLogEntry.Namespace
	bucketName := auditLogEntry.Name
	if bucketNamespace == "" && bucketName == "" {
		bucketNamespace = auditLogFallbackBucket
		bucketName = datagram.Pattern
	}

	entryKey, auditLogAddErr := valkeyClient.SetObjectWithAutoincrementLimit(auditLogEntry, AuditLogLimit, AuditLogTTL, "audit-log", bucketNamespace, bucketName)
	if auditLogAddErr != nil {
		moMetrics.IncAuditLogWriteFailure()
		logger.Error("failed to add to audit log", "error", auditLogAddErr)
	} else {
		moMetrics.IncAuditLogWritten("api")
		addToAuditLogIndex(entryKey, auditLogEntry.CreatedAt)
		dispatchAuditEvent(auditLogEntry)
	}
	return result, err
}

// addToAuditLogIndex records an entry key in the time-ordered ZSET index
// and prunes index members older than the entry TTL (entries expire on
// their own; without the prune their index members would linger forever).
// Best effort: a failed index write only degrades listing, never the entry.
func addToAuditLogIndex(entryKey string, createdAt time.Time) {
	client := valkeyClient.GetValkeyClient()
	ctx := valkeyClient.GetContext()
	cutoff := time.Now().Add(-AuditLogTTL).UnixMilli()
	cmds := []vgo.Completed{
		client.B().Zadd().Key(auditLogIndexKey).ScoreMember().
			ScoreMember(float64(createdAt.UnixMilli()), entryKey).Build(),
		client.B().Zremrangebyscore().Key(auditLogIndexKey).
			Min("-inf").Max(strconv.FormatInt(cutoff, 10)).Build(),
	}
	for _, resp := range client.DoMulti(ctx, cmds...) {
		if respErr := resp.Error(); respErr != nil && auditLogger != nil {
			auditLogger.Warn("failed to update audit log index", "error", respErr)
		}
	}
}

// redactSecretData returns a deep copy of obj with every data/stringData
// value replaced by a placeholder carrying a truncated SHA-256 of the
// original value. The hash keeps "did this key change?" visible in diffs
// without persisting the secret itself. Non-Secret objects pass through
// unchanged.
func redactSecretData(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil || obj.GetKind() != "Secret" {
		return obj
	}
	redacted := obj.DeepCopy()
	for _, field := range []string{"data", "stringData"} {
		values, found, _ := unstructured.NestedMap(redacted.Object, field)
		if !found {
			continue
		}
		for key, value := range values {
			str, _ := value.(string)
			values[key] = redactedValuePlaceholder(str)
		}
		_ = unstructured.SetNestedMap(redacted.Object, values, field)
	}
	return redacted
}

func redactedValuePlaceholder(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("***[REDACTED:sha256:%x]***", sum[:4])
}

// sanitizeAuditLogEntry removes sensitive material from an entry before it
// is persisted and pushed to the platform: Secret manifests embedded in
// payload/result are redacted, and configured operator secrets (API keys
// etc.) are masked in all serialized fields.
func sanitizeAuditLogEntry(entry *AuditLogEntry) {
	entry.Payload = redactSecretYamlInPayload(entry.Payload)
	entry.Payload = redactSensitiveKeys(entry.Payload)
	switch res := entry.Result.(type) {
	case *unstructured.Unstructured:
		entry.Result = redactSecretData(res)
	case unstructured.Unstructured:
		entry.Result = redactSecretData(&res)
	}
	entry.Payload = eraseConfigSecrets(entry.Payload)
	entry.Result = eraseConfigSecrets(entry.Result)
	entry.Diff = secrets.EraseSecrets(entry.Diff)
	entry.Error = secrets.EraseSecrets(entry.Error)
}

// sensitiveAuditPayloadKeys are payload field names whose values are
// credentials by construction (e.g. helm repo add/patch requests carry a
// repo password). Matched case-insensitively against map keys.
var sensitiveAuditPayloadKeys = map[string]struct{}{
	"password":      {},
	"token":         {},
	"apikey":        {},
	"accesstoken":   {},
	"authtoken":     {},
	"bearertoken":   {},
	"clientsecret":  {},
	"authorization": {},
}

// redactSensitiveKeys walks JSON-decoded payload structures and replaces
// values of credential-carrying keys. Maps are copied, never mutated in
// place (the datagram payload may still be referenced elsewhere).
func redactSensitiveKeys(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		copied := make(map[string]any, len(typed))
		for k, v := range typed {
			if _, sensitive := sensitiveAuditPayloadKeys[strings.ToLower(k)]; sensitive {
				if str, ok := v.(string); ok && str != "" {
					copied[k] = secrets.REDACTED
					continue
				}
			}
			copied[k] = redactSensitiveKeys(v)
		}
		return copied
	case []any:
		copied := make([]any, len(typed))
		for i, v := range typed {
			copied[i] = redactSensitiveKeys(v)
		}
		return copied
	default:
		return value
	}
}

// redactSecretYamlInPayload redacts Secret manifests that arrive as a
// yamlData string inside a payload map (e.g. create/update workload
// requests). The payload map is copied, never mutated in place.
func redactSecretYamlInPayload(payload any) any {
	payloadMap, ok := payload.(map[string]any)
	if !ok {
		return payload
	}
	yamlData, ok := payloadMap["yamlData"].(string)
	if !ok || yamlData == "" {
		return payload
	}
	var unstruct unstructured.Unstructured
	if yaml.Unmarshal([]byte(yamlData), &unstruct) != nil || unstruct.GetKind() != "Secret" {
		return payload
	}
	redactedYaml, err := yaml.Marshal(redactSecretData(&unstruct).Object)
	if err != nil {
		redactedYaml = []byte(secrets.REDACTED)
	}
	copied := make(map[string]any, len(payloadMap))
	for k, v := range payloadMap {
		copied[k] = v
	}
	copied["yamlData"] = string(redactedYaml)
	return copied
}

// eraseConfigSecrets masks configured operator secret values (see
// secrets.EraseSecrets) inside an arbitrary JSON-serializable value. When
// nothing matches, the original value is returned untouched.
func eraseConfigSecrets(value any) any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	masked := secrets.EraseSecrets(string(raw))
	if masked == string(raw) {
		return value
	}
	var out any
	if err := json.Unmarshal([]byte(masked), &out); err != nil {
		// Masking broke the JSON structure (secret spanned syntax); fall
		// back to the masked string so no secret survives.
		return masked
	}
	return out
}

// maxAuditListEntries caps how many entries a single list request will
// deserialize (search still has to inspect entry contents). The indexed
// path applies the cap newest-first, so at worst the OLDEST entries fall
// off — never the newest.
const maxAuditListEntries = 10000

func ListAuditLog(limit int, offset int, namespaces []string, clusterWide bool, workspaceName string, search string) ([]AuditLogEntry, int, error) {
	if limit <= 0 {
		limit = 100
	}

	entries, totalCount, err := listAuditLogIndexed(limit, offset, namespaces, clusterWide, workspaceName, search)
	if err != nil {
		// The index is an optimization, never a single point of failure:
		// fall back to the scan-based path so listing keeps working.
		if auditLogger != nil {
			auditLogger.Warn("audit log index read failed, falling back to keyspace scan", "error", err)
		}
		return listAuditLogScan(limit, offset, namespaces, clusterWide, workspaceName, search)
	}
	return entries, totalCount, nil
}

// listAuditLogIndexed serves a page from the time-ordered ZSET index:
// one ZREVRANGE over entry keys (small strings), key-level namespace
// filtering, then MGET of only what is needed — the requested page when
// there is no search term, or the newest maxAuditListEntries candidates
// when entry contents must be searched. Stale index members (entry expired
// or pruned) are removed on sight and the selection retried.
func listAuditLogIndexed(limit int, offset int, namespaces []string, clusterWide bool, workspaceName string, search string) ([]AuditLogEntry, int, error) {
	if err := ensureAuditLogIndex(); err != nil {
		return nil, 0, err
	}

	client := valkeyClient.GetValkeyClient()
	ctx := valkeyClient.GetContext()

	nsSet := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		nsSet[ns] = struct{}{}
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Newest first; members are entry keys, not entry payloads.
		members, err := client.Do(ctx,
			client.B().Zrevrange().Key(auditLogIndexKey).Start(0).Stop(-1).Build()).AsStrSlice()
		if err != nil {
			return nil, 0, err
		}

		// Key-level filtering, preserving the global newest-first order.
		// ai-chat keys carry no namespace; on workspace-scoped queries they
		// must be loaded to check entry.Workspace before they may appear.
		aiKeys := make([]string, 0)
		for _, key := range members {
			if !clusterWide && auditKeyNamespace(key) == "ai-chat" && workspaceName != "" {
				aiKeys = append(aiKeys, key)
			}
		}
		aiEntries, staleAi, err := mgetAuditEntries(aiKeys)
		if err != nil {
			return nil, 0, err
		}

		type candidate struct {
			key   string
			entry *AuditLogEntry // pre-loaded (ai-chat); nil = load on demand
		}
		candidates := make([]candidate, 0, len(members))
		for _, key := range members {
			ns := auditKeyNamespace(key)
			if ns == "" {
				continue
			}
			if clusterWide {
				candidates = append(candidates, candidate{key: key})
				continue
			}
			if ns == "ai-chat" {
				if entry, ok := aiEntries[key]; ok && workspaceName != "" && entry.Workspace == workspaceName {
					candidates = append(candidates, candidate{key: key, entry: entry})
				}
				continue
			}
			if _, ok := nsSet[ns]; ok {
				candidates = append(candidates, candidate{key: key})
			}
		}

		var stale []string
		var entries []AuditLogEntry
		var totalCount int

		if search == "" {
			// No content filter: page over keys first, load only the page.
			totalCount = len(candidates)
			if offset >= totalCount {
				removeStaleAuditIndexMembers(staleAi)
				return []AuditLogEntry{}, totalCount, nil
			}
			end := min(offset+limit, totalCount)
			page := candidates[offset:end]

			toLoad := make([]string, 0, len(page))
			for _, c := range page {
				if c.entry == nil {
					toLoad = append(toLoad, c.key)
				}
			}
			loaded, stalePage, err := mgetAuditEntries(toLoad)
			if err != nil {
				return nil, 0, err
			}
			stale = append(staleAi, stalePage...)
			if len(stalePage) == 0 {
				entries = make([]AuditLogEntry, 0, len(page))
				for _, c := range page {
					if c.entry != nil {
						entries = append(entries, *c.entry)
					} else if entry, ok := loaded[c.key]; ok {
						entries = append(entries, *entry)
					}
				}
			}
		} else {
			// Content search: load the newest candidates up to the cap.
			searchKeys := make([]string, 0, min(len(candidates), maxAuditListEntries))
			preloaded := make(map[string]*AuditLogEntry)
			for _, c := range candidates {
				if len(searchKeys) >= maxAuditListEntries {
					if auditLogger != nil {
						auditLogger.Warn("audit log search capped; oldest entries not searched",
							"cap", maxAuditListEntries, "candidates", len(candidates))
					}
					break
				}
				searchKeys = append(searchKeys, c.key)
				if c.entry != nil {
					preloaded[c.key] = c.entry
				}
			}
			toLoad := make([]string, 0, len(searchKeys))
			for _, key := range searchKeys {
				if _, ok := preloaded[key]; !ok {
					toLoad = append(toLoad, key)
				}
			}
			loaded, stalePage, err := mgetAuditEntries(toLoad)
			if err != nil {
				return nil, 0, err
			}
			stale = append(staleAi, stalePage...)
			searchLower := strings.ToLower(search)
			entries = make([]AuditLogEntry, 0)
			for _, key := range searchKeys {
				entry, ok := preloaded[key]
				if !ok {
					entry, ok = loaded[key]
				}
				if !ok {
					continue
				}
				if auditLogEntryMatchesSearch(*entry, searchLower) {
					entries = append(entries, *entry)
				}
			}
			totalCount = len(entries)
			if offset >= totalCount {
				removeStaleAuditIndexMembers(stale)
				return []AuditLogEntry{}, totalCount, nil
			}
			entries = entries[offset:min(offset+limit, totalCount)]
		}

		removeStaleAuditIndexMembers(stale)
		if search == "" && len(stale) > len(staleAi) {
			// A page member expired between ZREVRANGE and MGET; the index
			// is clean now, recompute the page.
			lastErr = fmt.Errorf("audit log index contained %d stale members", len(stale))
			continue
		}

		// Defensive final ordering (index order should already match).
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].CreatedAt.After(entries[j].CreatedAt)
		})
		return entries, totalCount, nil
	}
	return nil, 0, fmt.Errorf("audit log index unstable after %d attempts: %w", maxAttempts, lastErr)
}

// auditKeyNamespace extracts the namespace segment of an audit entry key
// (audit-log:<namespace>:<name>:<n>). Returns "" for malformed keys.
func auditKeyNamespace(key string) string {
	split := strings.Split(key, ":")
	if len(split) < 3 {
		return ""
	}
	return split[1]
}

// mgetAuditEntries loads entries for the given keys in chunks. Keys whose
// value is gone (expired/pruned) or unreadable are reported as stale so the
// caller can drop them from the index.
func mgetAuditEntries(keys []string) (map[string]*AuditLogEntry, []string, error) {
	entries := make(map[string]*AuditLogEntry, len(keys))
	stale := []string{}
	if len(keys) == 0 {
		return entries, stale, nil
	}
	client := valkeyClient.GetValkeyClient()
	ctx := valkeyClient.GetContext()
	for start := 0; start < len(keys); start += valkeyclient.MAX_CHUNK_GET_SIZE {
		chunk := keys[start:min(start+valkeyclient.MAX_CHUNK_GET_SIZE, len(keys))]
		values, err := client.Do(ctx, client.B().Mget().Key(chunk...).Build()).ToArray()
		if err != nil {
			return nil, nil, err
		}
		for i, value := range values {
			raw, valueErr := value.ToString()
			if valueErr != nil {
				stale = append(stale, chunk[i])
				continue
			}
			var entry AuditLogEntry
			if err := json.Unmarshal([]byte(raw), &entry); err != nil {
				stale = append(stale, chunk[i])
				continue
			}
			entries[chunk[i]] = &entry
		}
	}
	return entries, stale, nil
}

func removeStaleAuditIndexMembers(stale []string) {
	if len(stale) == 0 {
		return
	}
	client := valkeyClient.GetValkeyClient()
	ctx := valkeyClient.GetContext()
	_ = client.Do(ctx,
		client.B().Zrem().Key(auditLogIndexKey).Member(stale...).Build()).Error()
}

// ensureAuditLogIndex backfills entries written before the index existed
// (one full scan, once per deployment; the ready marker persists in
// Valkey). New entries are indexed on write, so after this the scan path
// is never needed again.
func ensureAuditLogIndex() error {
	if auditLogIndexEnsured.Load() {
		return nil
	}
	ready, err := valkeyClient.Exists(auditLogIndexReadyKey)
	if err != nil {
		return err
	}
	if ready {
		auditLogIndexEnsured.Store(true)
		return nil
	}

	keys, err := valkeyClient.Keys("audit-log:*")
	if err != nil {
		return err
	}
	entryKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.HasSuffix(key, ":counter") {
			continue
		}
		entryKeys = append(entryKeys, key)
	}

	entries, _, err := mgetAuditEntries(entryKeys)
	if err != nil {
		return err
	}

	client := valkeyClient.GetValkeyClient()
	ctx := valkeyClient.GetContext()
	// Legacy entries may predate the CreatedAt fallback and carry a zero
	// timestamp; give those the oldest still-listable score instead of 0,
	// which the TTL-based index prune would remove immediately.
	minScore := float64(time.Now().Add(-AuditLogTTL).Add(time.Minute).UnixMilli())
	const zaddChunk = 500
	pending := 0
	builder := client.B().Zadd().Key(auditLogIndexKey).ScoreMember()
	flush := func() error {
		if pending == 0 {
			return nil
		}
		if err := client.Do(ctx, builder.Build()).Error(); err != nil {
			return err
		}
		builder = client.B().Zadd().Key(auditLogIndexKey).ScoreMember()
		pending = 0
		return nil
	}
	for key, entry := range entries {
		score := float64(entry.CreatedAt.UnixMilli())
		if score < minScore {
			score = minScore
		}
		builder = builder.ScoreMember(score, key)
		pending++
		if pending >= zaddChunk {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}

	if err := valkeyClient.Set("1", 0, auditLogIndexReadyKey); err != nil {
		return err
	}
	auditLogIndexEnsured.Store(true)
	if auditLogger != nil {
		auditLogger.Info("audit log index backfilled", "entries", len(entries))
	}
	return nil
}

// listAuditLogScan is the pre-index implementation (full keyspace scan +
// bulk MGET). Kept as the fallback when the index path fails.
func listAuditLogScan(limit int, offset int, namespaces []string, clusterWide bool, workspaceName string, search string) ([]AuditLogEntry, int, error) {
	// Load ALL entries (no pagination yet) so we can sort by CreatedAt before paginating.
	// GetObjectsByPrefixWithSizeAndNs applies offset/limit on unsorted SCAN keys,
	// which can cause newer entries to be missed.
	allEntries, _, err := valkeyclient.GetObjectsByPrefixWithSizeAndNs[AuditLogEntry](valkeyClient, maxAuditListEntries, 0, namespaces, clusterWide, "audit-log")
	if err != nil {
		return []AuditLogEntry{}, 0, err
	}

	// AI entries live under audit-log:ai-chat:<email> and are not
	// namespace-scoped, so the namespace key filter above drops them. For
	// workspace-scoped queries, load them separately and keep only the
	// requested workspace — entries from other workspaces must not leak
	// into a workspace-scoped view.
	if !clusterWide {
		aiEntries, _, aiErr := valkeyclient.GetObjectsByPrefixWithSizeAndNs[AuditLogEntry](valkeyClient, maxAuditListEntries, 0, nil, true, "audit-log", "ai-chat")
		if aiErr != nil {
			return []AuditLogEntry{}, 0, aiErr
		}
		for _, entry := range aiEntries {
			if workspaceName != "" && entry.Workspace == workspaceName {
				allEntries = append(allEntries, entry)
			}
		}
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
	if strings.Contains(strings.ToLower(entry.Kind), searchLower) && entry.Kind != "" {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Namespace), searchLower) && entry.Namespace != "" {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Name), searchLower) && entry.Name != "" {
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
	createdAt := datagram.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return AuditLogEntry{
		RequestId: datagram.Id,
		Pattern:   datagram.Pattern,
		Success:   err == nil,
		Payload:   datagram.Payload,
		CreatedAt: createdAt,
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
		Success:   errStr == "",
		Payload:   payload,
		Result:    result,
		Error:     errStr,
		CreatedAt: time.Now(),
		User:      user,
		Workspace: workspace,
	}
	sanitizeAuditLogEntry(&entry)

	entryKey, storeErr := valkeyClient.SetObjectWithAutoincrementLimit(entry, AuditLogLimit, AuditLogTTL, "audit-log", "ai-chat", user.Email)
	if storeErr != nil {
		moMetrics.IncAuditLogWriteFailure()
		logger.Error("failed to add AI chat audit log", "error", storeErr)
	} else {
		moMetrics.IncAuditLogWritten("ai-chat")
		addToAuditLogIndex(entryKey, entry.CreatedAt)
		dispatchAuditEvent(entry)
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
