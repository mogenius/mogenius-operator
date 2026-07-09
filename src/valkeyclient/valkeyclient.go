package valkeyclient

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"net"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	valkeyclient "github.com/valkey-io/valkey-go"
)

type ValkeyClient interface {
	Connect() error
	Close()
	Set(value string, expiration time.Duration, keys ...string) error
	SetObject(value any, expiration time.Duration, keys ...string) error
	SetObjectWithAutoincrementLimit(value any, limit int64, ttl time.Duration, keys ...string) (string, error)
	Get(keys ...string) (string, error)
	GetObject(keys ...string) (any, error)
	List(limit int, keys ...string) ([]string, error)

	DeleteFromSortedListWithNsAndReleaseName(namespace string, releaseName string, keys ...string) error

	StoreSortedListEntry(data any, timestamp int64, keys ...string) error

	ClearNonEssentialKeys(includeTraffic bool, includePodStats bool, includeNodestats bool) (string, error)

	DeleteSingle(key ...string) error
	DeleteMultiple(patterns ...string) error
	Keys(pattern string) ([]string, error)
	Exists(keys ...string) (bool, error)

	GetValkeyClient() valkeyclient.Client
	GetContext() context.Context
	GetLogger() *slog.Logger
}

type SortOrder int

const (
	ORDER_NONE SortOrder = 0
	ORDER_ASC  SortOrder = 1
	ORDER_DESC SortOrder = 2

	MAX_CHUNK_GET_SIZE = 100

	// Defaults for time-series stream retention. Tuned for 1-minute write
	// cadence: 1440 entries = 24h, which keeps each stream around ~400 KiB
	// instead of the multi-MB streams produced by 10800 entries / 7d.
	// Override with MO_STATS_RETENTION_MAX_ENTRIES and MO_STATS_RETENTION_HOURS.
	defaultRetentionSize  int64         = 1440
	defaultRetentionHours time.Duration = 24 * time.Hour
)

// Exported for read-side queries that need to know how far back data is
// available. Updated at Connect() time from config.
var (
	MAX_RETENTION_SIZE int64         = defaultRetentionSize
	MAX_RETENTION_TIME time.Duration = defaultRetentionHours
)

type valkeyClient struct {
	logger *slog.Logger
	config config.ConfigModule

	ctx context.Context
	// internal valkey client used to connect to a valkey instance
	valkeyClient valkeyclient.Client
}

func NewValkeyClient(logger *slog.Logger, configModule config.ConfigModule) ValkeyClient {
	self := &valkeyClient{}

	self.ctx = context.Background()
	self.logger = logger
	self.config = configModule

	return self
}

func (self *valkeyClient) Connect() error {
	self.logger.Info("Connecting to valkey")

	if raw := self.config.Get("MO_STATS_RETENTION_MAX_ENTRIES"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil && n > 0 {
			MAX_RETENTION_SIZE = n
		}
	}
	if raw := self.config.Get("MO_STATS_RETENTION_HOURS"); raw != "" {
		if h, err := strconv.ParseInt(raw, 10, 64); err == nil && h > 0 {
			MAX_RETENTION_TIME = time.Duration(h) * time.Hour
		}
	}
	self.logger.Info("stats retention configured",
		"maxEntries", MAX_RETENTION_SIZE, "ttl", MAX_RETENTION_TIME)

	valkeyHost := self.config.Get("MO_VALKEY_ADDR")
	valkeyHost, valkeyPort, err := net.SplitHostPort(valkeyHost)
	assert.Assert(err == nil, err)
	assert.Assert(valkeyHost != "")
	assert.Assert(valkeyPort != "")
	valkeyAddr := valkeyHost + ":" + valkeyPort
	valkeyPwd := self.config.Get("MO_VALKEY_PASSWORD")

	client, err := valkeyclient.NewClient(valkeyclient.ClientOption{
		InitAddress:         []string{valkeyAddr},
		Password:            valkeyPwd,
		SelectDB:            0,
		DisableRetry:        true,
		ReadBufferEachConn:  512 * (1 << 10), // 512 KiB
		WriteBufferEachConn: 512 * (1 << 10), // 512 KiB
		ConnWriteTimeout:    10 * time.Second,
		MaxFlushDelay:       100 * time.Microsecond, // Reduce latency for pipelined commands
	})
	if err != nil {
		self.logger.Info("connection to Valkey failed", "valkeyAddr", valkeyAddr, "error", err)
		return fmt.Errorf("could not connect to Valkey: %s", err)
	}
	self.valkeyClient = client
	err = self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Ping().Build()).Error()
	if err != nil {
		self.logger.Info("connection to Valkey failed", "addr", valkeyAddr, "error", err)
		return fmt.Errorf("could not connect to Valkey: %w", err)
	}

	self.logger.Info("Connected to valkey", "addr", valkeyAddr)

	return nil
}

// Close shuts down the underlying valkey connection. valkey-go's Close waits
// for all pending (pipelined) calls to finish before closing, so buffered
// writes are flushed instead of lost on shutdown. Safe to call when never
// connected.
func (self *valkeyClient) Close() {
	if self.valkeyClient != nil {
		self.valkeyClient.Close()
	}
}

func (self *valkeyClient) GetValkeyClient() valkeyclient.Client {
	return self.valkeyClient
}

func (self *valkeyClient) GetContext() context.Context {
	return self.ctx
}

func (self *valkeyClient) GetLogger() *slog.Logger {
	return self.logger
}

func (self *valkeyClient) Set(value string, expiration time.Duration, keys ...string) error {
	key := createKey(keys...)

	cmd := self.valkeyClient.B().Set().Key(key).Value(value)
	if expiration != time.Duration(0) {
		cmd.Ex(expiration)
	}
	err := self.valkeyClient.Do(self.ctx, cmd.Build()).Error()
	if err != nil {
		self.logger.Error("Error setting value in Valkey", "key", key, "error", err)
		return err
	}

	return nil
}

func (self *valkeyClient) SetObject(value any, expiration time.Duration, keys ...string) error {
	key := createKey(keys...)

	objStr, err := json.Marshal(value)
	if err != nil {
		self.logger.Error("Error marshalling object for Valkey", "key", key, "error", err)
		return err
	}

	return self.Set(string(objStr), expiration, key)
}

// maxAutoincrementRetries bounds the EXISTS-then-SET loop below. Each
// retry corresponds to a single legacy key that occupies the slot the
// INCR happened to return; in practice this is at most `limit` retries,
// happens once per baseKey when upgrading from the previous SCAN+sort
// implementation, and never afterwards.
const maxAutoincrementRetries = 4096

// SetObjectWithAutoincrementLimit appends a numbered entry under baseKey
// (joined keys) and prunes older entries beyond `limit`. The previous
// implementation did SCAN + sort + SET without any atomic operation, so
// two concurrent callers could both compute nextNum=N+1 and the second
// SET silently overwrote the first - direct audit-log data loss under
// any concurrency.
//
// Numbering now comes from INCR on a dedicated `<baseKey>:counter` key,
// which is atomic in Valkey. If the resulting candidate key already
// exists (leftover from the pre-INCR algorithm, which always started
// at 1), we INCR again until we find a free slot. The counter carries the
// same TTL as the entries, refreshed on every write: it always outlives
// the entries it numbers (keeping the sequence monotonic while data
// exists), but no longer leaks a permanent key for every churned
// (namespace, name) pair.
//
// Pruning keeps at most `limit` numeric entries by deleting the lowest-
// numbered ones first - same observable semantic as the old code, just
// without the race.
//
// Returns the key the entry was stored under.
func (self *valkeyClient) SetObjectWithAutoincrementLimit(value any, limit int64, ttl time.Duration, keys ...string) (string, error) {
	baseKey := strings.Join(keys, ":")
	counterKey := baseKey + ":counter"

	jsonValue, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("error while serializing value: %w", err)
	}

	var newKey string
	var chosenNum int64
	for range maxAutoincrementRetries {
		nextNum, err := self.valkeyClient.Do(self.ctx,
			self.valkeyClient.B().Incr().Key(counterKey).Build()).AsInt64()
		if err != nil {
			return "", fmt.Errorf("error incrementing counter: %w", err)
		}
		candidate := fmt.Sprintf("%s:%d", baseKey, nextNum)
		existsCount, err := self.valkeyClient.Do(self.ctx,
			self.valkeyClient.B().Exists().Key(candidate).Build()).AsInt64()
		if err != nil {
			return "", fmt.Errorf("error checking candidate key: %w", err)
		}
		if existsCount == 0 {
			newKey = candidate
			chosenNum = nextNum
			break
		}
		// Slot is taken by legacy data; skip ahead and try the next.
	}
	if newKey == "" {
		return "", fmt.Errorf("could not find a free slot under %q after %d retries", baseKey, maxAutoincrementRetries)
	}

	var setCmd valkeyclient.Completed
	if ttl > 0 {
		setCmd = self.valkeyClient.B().Set().Key(newKey).Value(string(jsonValue)).Ex(ttl).Build()
	} else {
		setCmd = self.valkeyClient.B().Set().Key(newKey).Value(string(jsonValue)).Build()
	}
	if err := self.valkeyClient.Do(self.ctx, setCmd).Error(); err != nil {
		return "", fmt.Errorf("error setting entry: %w", err)
	}
	if ttl > 0 {
		_ = self.valkeyClient.Do(self.ctx,
			self.valkeyClient.B().Expire().Key(counterKey).Seconds(int64(ttl.Seconds())).Build()).Error()
	}

	if limit <= 0 {
		return newKey, nil
	}

	// Fast prune: numbering is monotonic, so the slot that just fell out
	// of the window is chosenNum-limit. A single UNLINK replaces the
	// full-keyspace SCAN that used to run on every write.
	if expiredNum := chosenNum - limit; expiredNum > 0 {
		expiredKey := fmt.Sprintf("%s:%d", baseKey, expiredNum)
		_ = self.valkeyClient.Do(self.ctx,
			self.valkeyClient.B().Unlink().Key(expiredKey).Build()).Error()
	}

	// Slow prune: occasionally sweep for stragglers (skipped slots,
	// lowered limits, legacy data). Best-effort: concurrent writers may
	// briefly leave the count slightly above limit; that's acceptable and
	// self-corrects on subsequent writes.
	if chosenNum%100 != 0 {
		return newKey, nil
	}
	existingKeys, err := self.Keys(baseKey + ":*")
	if err != nil {
		return newKey, nil
	}
	type numbered struct {
		key string
		n   int64
	}
	numerics := make([]numbered, 0, len(existingKeys))
	for _, k := range existingKeys {
		if k == counterKey {
			continue
		}
		n := extractNumber(k, baseKey)
		if n > 0 {
			numerics = append(numerics, numbered{k, n})
		}
	}
	if int64(len(numerics)) <= limit {
		return newKey, nil
	}
	sort.Slice(numerics, func(i, j int) bool { return numerics[i].n < numerics[j].n })
	toDelete := int64(len(numerics)) - limit
	delCmds := make([]valkeyclient.Completed, 0, toDelete)
	for i := range toDelete {
		delCmds = append(delCmds, self.valkeyClient.B().Del().Key(numerics[i].key).Build())
	}
	for _, resp := range self.valkeyClient.DoMulti(self.ctx, delCmds...) {
		_ = resp.Error()
	}
	return newKey, nil
}

func extractNumber(key, baseKey string) int64 {
	prefix := baseKey + ":"
	if !strings.HasPrefix(key, prefix) {
		return 0
	}

	numStr := strings.TrimPrefix(key, prefix)
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0
	}

	return num
}

func (self *valkeyClient) Get(keys ...string) (string, error) {
	key := createKey(keys...)

	val, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Get().Key(key).Build()).ToString()
	if err != nil {
		if err == valkeyclient.Nil {
			self.logger.Info("Key does not exist", "key", key)
			return "", nil
		}
		self.logger.Error("Error getting value from Valkey", "key", key, "error", err)
		return "", err
	}

	return val, nil
}

func (self *valkeyClient) GetObject(keys ...string) (any, error) {
	key := createKey(keys...)
	var result any
	val, err := self.Get(key)
	if err != nil {
		return result, err
	}
	if val == "" {
		// Key doesn't exist or is empty
		return nil, nil
	}
	// Correct usage of Unmarshal
	err = json.Unmarshal([]byte(val), &result)
	if err != nil {
		self.logger.Error("Error unmarshalling value from Valkey", "key", key, "error", err)
		return result, err
	}
	return result, nil
}

func (self *valkeyClient) List(limit int, keys ...string) ([]string, error) {
	key := createKey(keys...)

	selectedKeys, err := self.Keys(key)
	if err != nil {
		self.logger.Error("Error listing keys from Valkey", "pattern", key, "error", err)
		return []string{}, err
	}
	if len(selectedKeys) == 0 {
		self.logger.Debug("No keys found for pattern", "pattern", key)
		return selectedKeys, nil
	}

	// apply limit to the number of keys to fetch
	if limit > 0 && len(selectedKeys) > limit {
		selectedKeys = selectedKeys[:limit]
	}

	// Fetch the values in 100 chunks to avoid memory issues with large datasets
	chunks := make([][]string, 0, (len(selectedKeys)+MAX_CHUNK_GET_SIZE-1)/MAX_CHUNK_GET_SIZE)
	for i := 0; i < len(selectedKeys); i += MAX_CHUNK_GET_SIZE {
		end := min(i+MAX_CHUNK_GET_SIZE, len(selectedKeys))
		chunks = append(chunks, selectedKeys[i:end])
	}

	// Fetch the values for these keys
	result := make([]string, 0, len(selectedKeys))
	for _, v := range chunks {
		values, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Mget().Key(v...).Build()).AsStrSlice()
		if err != nil {
			self.logger.Error("Error fetching values from Valkey", "keys", v, "error", err)
			return result, err
		}
		result = append(result, values...)
	}

	return result, nil
}

func (self *valkeyClient) DeleteFromSortedListWithNsAndReleaseName(namespace string, releaseName string, keys ...string) error {
	key := createKey(keys...)

	cmd := self.valkeyClient.B().Xrevrange().Key(key).End("+").Start("-").Build()
	result := self.valkeyClient.Do(self.ctx, cmd)
	if err := result.Error(); err != nil {
		return fmt.Errorf("failed to query stream: %w", err)
	}

	// Parse the stream messages
	messages, err := result.AsXRange()
	if err != nil {
		return fmt.Errorf("failed to parse stream result: %w", err)
	}

	// Collect IDs to delete
	idsToDelete := make([]string, 0)
	for _, v := range messages {
		var obj map[string]any
		err := json.Unmarshal([]byte(v.FieldValues["data"]), &obj)
		if err != nil {
			return fmt.Errorf("error unmarshalling value from valkey: %w", err)
		}

		// Check if the object contains a "Payload" field
		payload, ok := obj["Payload"].(map[string]any)
		if !ok {
			continue
		}

		// Extract namespace and releaseName from the Payload
		if payload["namespace"] == namespace && payload["releaseName"] == releaseName {
			idsToDelete = append(idsToDelete, v.ID)
		}
	}

	// Batch delete all matching entries
	if len(idsToDelete) > 0 {
		delCmd := self.valkeyClient.B().Xdel().Key(key).Id(idsToDelete...).Build()
		if err := self.valkeyClient.Do(self.ctx, delCmd).Error(); err != nil {
			return fmt.Errorf("failed to delete entries from stream: %w", err)
		}
	}

	return nil
}

func (self *valkeyClient) ClearNonEssentialKeys(includeTraffic bool, includePodStats bool, includeNodestats bool) (string, error) {
	// resources & helm have to be kept
	prefixesToDelete := []string{
		"live-stats:*",
		"logs:cmd*",
		"logs:core*",
		"logs:client-provider*",
		"logs:db-stats*",
		"logs:http*",
		"logs:klog*",
		"logs:pod-stats-collector*",
		"logs:kubernetes*",
		"logs:leader-elector*",
		"logs:mokubernetes*",
		"logs:socketapi*",
		"logs:structs*",
		"logs:utils*",
		"logs:xterm*",
		"logs:traffic-collector*",
		"logs:valkey*",
		"logs:websocket-events-client*",
		"logs:websocket-job-client*",
		"maschine-stats:*",
		"status:*",
	}

	if includeTraffic {
		prefixesToDelete = append(prefixesToDelete, "traffic-stats:*")
	}
	if includePodStats {
		prefixesToDelete = append(prefixesToDelete, "pod-stats:*")
	}
	if includeNodestats {
		prefixesToDelete = append(prefixesToDelete, "node-stats:*")
	}

	self.logger.Info("Deleting non-essential keys from Valkey", "includeTraffic", includeTraffic, "includePodStats", includePodStats, "includeNodestats", includeNodestats)

	// Use pattern-based deletion for better performance
	cacheDeleteCounter := 0
	for _, pattern := range prefixesToDelete {
		cursor := uint64(0)
		batch := make([]string, 0, 100)

		for {
			result := self.valkeyClient.Do(self.ctx,
				self.valkeyClient.B().Scan().Cursor(cursor).Match(pattern).Count(1000).Build())

			if result.Error() != nil {
				return "", fmt.Errorf("error scanning keys for pattern %s: %v", pattern, result.Error())
			}

			scanResult, err := result.AsScanEntry()
			if err != nil {
				return "", fmt.Errorf("error parsing scan result for pattern %s: %w", pattern, err)
			}

			// Collect keys for deletion
			for _, key := range scanResult.Elements {
				batch = append(batch, key)

				// Delete in batches of 100
				if len(batch) >= 100 {
					if err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(batch...).Build()).Error(); err != nil {
						return "", fmt.Errorf("error deleting batch: %w", err)
					}
					cacheDeleteCounter += len(batch)
					batch = batch[:0]
				}
			}

			cursor = scanResult.Cursor
			if cursor == 0 {
				break
			}
		}

		// Delete remaining keys in batch
		if len(batch) > 0 {
			if err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(batch...).Build()).Error(); err != nil {
				return "", fmt.Errorf("error deleting final batch: %w", err)
			}
			cacheDeleteCounter += len(batch)
		}
	}

	resultMsg := fmt.Sprintf("Deleted %d non-essential keys from Valkey", cacheDeleteCounter)
	self.logger.Info(resultMsg, "deletedKeys", cacheDeleteCounter)

	return resultMsg, nil
}

func (self *valkeyClient) DeleteSingle(keys ...string) error {
	key := createKey(keys...)
	err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(key).Build()).Error()
	if err != nil {
		self.logger.Error("Error deleting key from Valkey", "key", keys, "error", err)
		return err
	}

	return nil
}

func (self *valkeyClient) DeleteMultiple(patterns ...string) error {
	if len(patterns) == 0 {
		return nil
	}

	cursor := uint64(0)
	batch := make([]string, 0, 100)
	totalDeleted := 0

	for {
		result := self.valkeyClient.Do(self.ctx,
			self.valkeyClient.B().Scan().Cursor(cursor).Count(1000).Build())

		if result.Error() != nil {
			return result.Error()
		}

		scanResult, err := result.AsScanEntry()
		if err != nil {
			return err
		}

		// Check each key against patterns
		for _, key := range scanResult.Elements {
			for _, pattern := range patterns {
				if matched, _ := filepath.Match(pattern, key); matched {
					batch = append(batch, key)
					break
				}
			}

			// Delete batch when full
			if len(batch) >= 100 {
				if err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(batch...).Build()).Error(); err != nil {
					self.logger.Error("Error deleting batch in DeleteMultiple", "error", err)
					return err
				}
				totalDeleted += len(batch)
				batch = batch[:0]
			}
		}

		cursor = scanResult.Cursor
		if cursor == 0 {
			break
		}
	}

	// Delete remaining keys
	if len(batch) > 0 {
		if err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Del().Key(batch...).Build()).Error(); err != nil {
			self.logger.Error("Error deleting final batch in DeleteMultiple", "error", err)
			return err
		}
		totalDeleted += len(batch)
	}

	self.logger.Debug("Successfully deleted keys", "count", totalDeleted, "patterns", patterns)
	return nil
}

func (self *valkeyClient) Keys(pattern string) ([]string, error) {
	// Pre-allocate with reasonable capacity to reduce reallocations
	allKeys := make([]string, 0, 1000)
	cursor := uint64(0)

	for {
		// Use larger count for fewer network round trips
		cmd := self.valkeyClient.B().Scan().Cursor(cursor).Match(pattern).Count(5000).Build()
		result, err := self.valkeyClient.Do(self.ctx, cmd).ToArray()
		if err != nil {
			self.logger.Error("Error scanning keys from Valkey", "pattern", pattern, "cursor", cursor, "error", err)
			return nil, err
		}

		if len(result) != 2 {
			self.logger.Error("Unexpected SCAN response format", "pattern", pattern, "result_length", len(result))
			return nil, fmt.Errorf("unexpected SCAN response format")
		}

		// Get new cursor
		newCursor, err := result[0].AsUint64()
		if err != nil {
			self.logger.Error("Error parsing cursor from SCAN response", "pattern", pattern, "error", err)
			return nil, err
		}

		// Get keys from this iteration
		keys, err := result[1].AsStrSlice()
		if err != nil {
			self.logger.Error("Error parsing keys from SCAN response", "pattern", pattern, "error", err)
			return nil, err
		}

		// Filter out empty strings
		for _, key := range keys {
			if key != "" {
				allKeys = append(allKeys, key)
			}
		}

		cursor = newCursor
		if cursor == 0 {
			break // Scan completed
		}
	}

	return allKeys, nil
}

func (self *valkeyClient) Exists(keys ...string) (bool, error) {
	key := createKey(keys...)

	exists, err := self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Exists().Key(key).Build()).AsInt64()
	if err != nil {
		self.logger.Error("Error checking if key exists in Valkey", "key", key, "error", err)
		return false, err
	}

	return exists > 0, nil
}

func GetObjectsByPattern[T any](store ValkeyClient, pattern string, keywords []string) ([]T, error) {
	keyList, err := store.Keys(pattern)
	if err != nil {
		return []T{}, err
	}

	// filter for keywords in a single pass
	if len(keywords) > 0 {
		filteredKeys := make([]string, 0, len(keyList))
		for _, key := range keyList {
			if slices.Contains(keywords, key) {
				filteredKeys = append(filteredKeys, key)
			}
		}
		keyList = filteredKeys
	}

	return GetObjectsForKeys[T](store, keyList)
}

// GetObjectsForKeys fetches and unmarshals the values for an explicit key
// list via chunked MGETs (100 keys per roundtrip).
func GetObjectsForKeys[T any](store ValkeyClient, keyList []string) ([]T, error) {
	if len(keyList) == 0 {
		return []T{}, nil
	}

	client := store.GetValkeyClient()
	result := make([]T, 0, len(keyList))

	for i := 0; i < len(keyList); i += MAX_CHUNK_GET_SIZE {
		end := min(i+MAX_CHUNK_GET_SIZE, len(keyList))
		values, err := client.Do(store.GetContext(), client.B().Mget().Key(keyList[i:end]...).Build()).AsStrSlice()
		if err != nil {
			return result, err
		}
		for _, v := range values {
			// MGET yields an empty entry for keys that were deleted or
			// expired between key discovery and fetch; skip those instead
			// of failing the whole batch.
			if v == "" {
				continue
			}
			var obj T
			if err := json.Unmarshal([]byte(v), &obj); err != nil {
				return result, fmt.Errorf("error unmarshalling value from Valkey: %w", err)
			}
			result = append(result, obj)
		}
	}

	return result, nil
}

func GetObjectsByPrefix[T any](store ValkeyClient, order SortOrder, keys ...string) ([]T, error) {
	pattern := createKey(keys...)

	client := store.GetValkeyClient()

	// Get the keys
	keyList, err := store.Keys(pattern)
	if err != nil {
		return []T{}, err
	}
	if len(keyList) == 0 {
		return []T{}, nil
	}

	// Sort keys
	sortStringsByTimestamp(keyList, order)

	// Fetch the values in 100 chunks to avoid memory issues with large datasets
	chunks := make([][]string, 0, (len(keyList)+MAX_CHUNK_GET_SIZE-1)/MAX_CHUNK_GET_SIZE)
	for i := 0; i < len(keyList); i += MAX_CHUNK_GET_SIZE {
		end := min(i+MAX_CHUNK_GET_SIZE, len(keyList))
		chunks = append(chunks, keyList[i:end])
	}

	result := make([]T, 0, len(keyList))

	for _, v := range chunks {
		values, err := client.Do(store.GetContext(), client.B().Mget().Key(v...).Build()).AsStrSlice()
		if err != nil {
			return result, err
		}
		for _, v := range values {
			var obj T
			if err := json.Unmarshal([]byte(v), &obj); err != nil {
				return result, fmt.Errorf("error unmarshalling value from Valkey: %w", err)
			}
			result = append(result, obj)
		}
	}
	return result, nil
}

func GetObjectsByPrefixWithSizeAndNs[T any](store ValkeyClient, limit int, offset int, namespaces []string, clusterWide bool, keys ...string) ([]T, int, error) {
	key := createKey(keys...)
	pattern := key + "*"
	client := store.GetValkeyClient()

	// Get the keys
	keyList, err := store.Keys(pattern)
	if err != nil {
		return []T{}, 0, err
	}

	// Drop bookkeeping keys that share the prefix but do not hold a T.
	// SetObjectWithAutoincrementLimit maintains a `<baseKey>:counter` key
	// whose value is a plain integer (from INCR); unmarshalling it into T
	// would fail with "cannot unmarshal number into Go value of type ...".
	if len(keyList) > 0 {
		filtered := keyList[:0]
		for _, k := range keyList {
			if strings.HasSuffix(k, ":counter") {
				continue
			}
			filtered = append(filtered, k)
		}
		keyList = filtered
	}

	// Filter by namespace in a single pass if needed
	if !clusterWide && len(namespaces) > 0 {
		// Build namespace set for O(1) lookup instead of O(n) slices.Contains
		nsSet := make(map[string]struct{}, len(namespaces))
		for _, ns := range namespaces {
			nsSet[ns] = struct{}{}
		}

		filteredKeys := make([]string, 0, len(keyList))
		for _, key := range keyList {
			split := strings.Split(key, ":")
			if len(split) >= 3 {
				if _, ok := nsSet[split[1]]; ok {
					filteredKeys = append(filteredKeys, key)
				}
			}
		}
		keyList = filteredKeys
	}

	totalCount := len(keyList)
	if totalCount == 0 {
		return []T{}, 0, nil
	}

	// Apply offset and limit efficiently
	if offset >= totalCount {
		return []T{}, totalCount, nil
	}
	end := min(offset+limit, totalCount)
	keyList = keyList[offset:end]

	if len(keyList) == 0 {
		return []T{}, totalCount, nil
	}

	// Pre-allocate result slice
	result := make([]T, 0, len(keyList))

	values, err := client.Do(store.GetContext(), client.B().Mget().Key(keyList...).Build()).AsStrSlice()
	if err != nil {
		return result, totalCount, err
	}
	for _, v := range values {
		var obj T
		if err := json.Unmarshal([]byte(v), &obj); err != nil {
			return result, totalCount, fmt.Errorf("error unmarshalling value from Valkey: %w", err)
		}
		result = append(result, obj)
	}
	return result, totalCount, nil
}

func GetObjectForKey[T any](store ValkeyClient, keys ...string) (*T, error) {
	key := createKey(keys...)
	client := store.GetValkeyClient()
	data, err := client.Do(store.GetContext(), client.B().Get().Key(key).Build()).ToString()
	if err != nil {
		return nil, err
	}
	var obj T
	err = json.Unmarshal([]byte(data), &obj)
	return &obj, err
}

func createKey(parts ...string) string {
	key := strings.Join(parts, ":")

	// Remove trailing ":*:*" patterns
	for strings.HasSuffix(key, ":*:*") {
		key = strings.TrimSuffix(key, ":*")
	}

	return key
}

func sortStringsByTimestamp(stringsToSort []string, order SortOrder) {
	if order == ORDER_NONE || len(stringsToSort) == 0 {
		return
	}

	// Pre-extract timestamps once instead of O(n log n) times during sort comparisons
	type indexedTimestamp struct {
		index     int
		timestamp int
	}

	timestamps := make([]indexedTimestamp, len(stringsToSort))
	for i, s := range stringsToSort {
		parts := strings.Split(s, ":")
		ts, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			ts = 0 // Default for unparseable timestamps
		}
		timestamps[i] = indexedTimestamp{index: i, timestamp: ts}
	}

	// Sort the indices by timestamp
	sort.Slice(timestamps, func(i, j int) bool {
		if order == ORDER_ASC {
			return timestamps[i].timestamp < timestamps[j].timestamp
		}
		return timestamps[i].timestamp > timestamps[j].timestamp
	})

	// Reorder original slice based on sorted indices
	result := make([]string, len(stringsToSort))
	for i, t := range timestamps {
		result[i] = stringsToSort[t.index]
	}
	copy(stringsToSort, result)
}

func parseStreamMessages[T any](logger *slog.Logger, messages []valkeyclient.XRangeEntry) ([]T, error) {
	dataPoints := make([]T, 0, len(messages))

	for _, msg := range messages {
		if dataStr, ok := msg.FieldValues["data"]; ok {
			var dataPoint T
			err := json.Unmarshal([]byte(dataStr), &dataPoint)
			if err != nil {
				logger.Error("Failed to unmarshal stream data", "ID", msg.ID, "error", err)
				continue
			}
			dataPoints = append(dataPoints, dataPoint)
		}
	}
	return dataPoints, nil
}

func (self *valkeyClient) StoreSortedListEntry(data any, timestamp int64, keys ...string) error {
	streamKey := createKey(keys...)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	//  make sure it's in milliseconds because valkey requires it
	if timestamp < 10000000000 {
		timestamp = timestamp * 1000
	}

	id := fmt.Sprintf("%d-0", timestamp)

	// This path runs per log line and per pod-stats write. Pipeline XADD,
	// retention trims and TTL refresh into a single roundtrip instead of
	// four sequential Do() calls.
	cmds := make([]valkeyclient.Completed, 0, 4)
	cmds = append(cmds, self.valkeyClient.B().Xadd().Key(streamKey).Id(id).FieldValue().
		FieldValue("data", string(jsonData)).
		Build())
	if MAX_RETENTION_TIME > 0 {
		cutoffTime := time.Now().Add(-MAX_RETENTION_TIME)
		cutoffID := fmt.Sprintf("%d-0", cutoffTime.UnixMilli())
		cmds = append(cmds, self.valkeyClient.B().Xtrim().Key(streamKey).Minid().Threshold(cutoffID).Build())
	}
	if MAX_RETENTION_SIZE > 0 {
		cmds = append(cmds, self.valkeyClient.B().Xtrim().Key(streamKey).Maxlen().Threshold(fmt.Sprintf("%d", MAX_RETENTION_SIZE)).Build())
	}
	// Set TTL on the stream key so stale keys (e.g. for deleted pods) expire automatically.
	// Active keys get their TTL refreshed on every write.
	cmds = append(cmds, self.valkeyClient.B().Expire().Key(streamKey).Seconds(int64(MAX_RETENTION_TIME.Seconds())).Build())

	results := self.valkeyClient.DoMulti(self.ctx, cmds...)

	// The XADD result (first command) decides the overall outcome.
	if err := results[0].Error(); err != nil {
		errString := err.Error()

		if strings.Contains(errString, "The ID specified in XADD is equal or smaller than the target stream top item") {
			// This means we're trying to insert a duplicate entry
			// we dont care about duplicates (and skip the publish so
			// subscribers don't see the same entry twice)
			return nil
		}
		// Previously: on WRONGTYPE the wrapper silently DEL'd the
		// existing key and retried XADD. That converts an unexpected
		// schema collision into permanent, undetected data loss. If a
		// stream-key namespace ever collides with a string/hash/list
		// key, surface it loudly instead so the underlying cause can be
		// fixed (renamed key prefix, leftover from an older operator
		// version, manual debug write, etc.).
		if errString == "WRONGTYPE Operation against a key holding the wrong kind of value" {
			typeResult := self.valkeyClient.Do(self.ctx,
				self.valkeyClient.B().Type().Key(streamKey).Build())
			actualType, _ := typeResult.ToString()
			self.logger.Error("WRONGTYPE on stream write - refusing to overwrite existing key",
				"key", streamKey, "actualType", actualType)
			return fmt.Errorf("WRONGTYPE on stream key %q (existing type %q); refusing to delete to avoid data loss", streamKey, actualType)
		}
		return fmt.Errorf("failed to add to stream: %w, key:%s", err, streamKey)
	}

	// Trim/expire results: best-effort maintenance, log and continue.
	for _, result := range results[1:] {
		if err := result.Error(); err != nil && err.Error() != "ERR no such key" {
			self.logger.Error("failed to trim/expire stream", "key", streamKey, "error", err.Error())
		}
	}

	// notify subscribers about the new entry (reuse the already marshaled
	// payload instead of marshalling the same data a second time)
	err = self.valkeyClient.Do(self.ctx, self.valkeyClient.B().Publish().Channel(createChannel(keys...)).Message(string(jsonData)).Build()).Error()
	if err != nil {
		return err
	}

	return nil
}

func GetObjectsFromSortedListWithDuration[T any](store ValkeyClient, duration int64, keys ...string) ([]T, error) {
	end := time.Now().UTC()
	start := end.Add(-time.Duration(duration) * time.Minute).UTC()
	return GetObjectsFromSortedListWithRange[T](store, start, end, keys...)
}

func GetObjectsFromSortedListWithRange[T any](store ValkeyClient, start, end time.Time, keys ...string) ([]T, error) {
	key := createKey(keys...)
	startID := fmt.Sprintf("%d-0", start.UnixMilli())
	endID := fmt.Sprintf("%d-0", end.UnixMilli())

	// Build XRANGE command using valkey-go command builder
	cmd := store.GetValkeyClient().B().Xrange().Key(key).Start(startID).End(endID).Build()
	result := store.GetValkeyClient().Do(store.GetContext(), cmd)

	if err := result.Error(); err != nil {
		return nil, fmt.Errorf("failed to query stream: %w", err)
	}

	// Parse the stream messages
	messages, err := result.AsXRange()
	if err != nil {
		return nil, fmt.Errorf("failed to parse stream result: %w", err)
	}

	return parseStreamMessages[T](store.GetLogger(), messages)
}

func GetLastObjectsFromSortedList[T any](store ValkeyClient, count int64, keys ...string) ([]T, error) {
	key := createKey(keys...)

	// Build XREVRANGE command to get entries in reverse order (newest first)
	// Using "+" as end (latest) and "-" as start (oldest), with COUNT to limit results
	cmd := store.GetValkeyClient().B().Xrevrange().Key(key).End("+").Start("-").Count(count).Build()
	result := store.GetValkeyClient().Do(store.GetContext(), cmd)
	if err := result.Error(); err != nil {
		return nil, fmt.Errorf("failed to query stream: %w", err)
	}

	// Parse the stream messages
	messages, err := result.AsXRange()
	if err != nil {
		return nil, fmt.Errorf("failed to parse stream result: %w", err)
	}

	return parseStreamMessages[T](store.GetLogger(), messages)
}

func createChannel(parts ...string) string {
	return strings.Join(parts, ":") + ":channel"
}
