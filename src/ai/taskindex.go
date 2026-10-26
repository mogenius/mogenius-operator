package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"mogenius-operator/src/valkeyclient"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valkey-io/valkey-go"
)

// The task index is a ZSET over all AI task keys (ai_tasks:*) with score =
// the key's expiry time in unix milliseconds. Every listing of tasks used to
// run a full-keyspace SCAN — per UI poll, per workspace namespace and once a
// minute in the queue loop — which is O(keyspace) on Valkey's single thread
// and saturated it on large clusters (MOG-4518). Reading the index instead is
// O(number of tasks).
//
// Writes go through createOrUpdateAiTask, which indexes the key; deletes
// unindex it. Members whose key expired fall out by score. A periodic resync
// (one SCAN per replica every aiTaskIndexResyncInterval) backfills keys
// written before the index existed or by an older operator during a rollout,
// and drops members whose key was deleted behind the index's back.
const (
	aiTaskIndexKey            = "idx:ai-tasks"
	aiTaskIndexResyncInterval = 15 * time.Minute
)

// taskIndex lists and loads AI tasks without scanning the keyspace.
type taskIndex interface {
	add(key string)
	remove(keys ...string)
	// keys returns all task keys matching a Valkey glob pattern
	keys(pattern string) ([]string, error)
	// load fetches the tasks for keys, skipping keys that are gone or
	// unreadable
	load(keys []string) []aiTaskWithKey
}

// tasks returns the manager's task index. Managers built without
// NewAiManager (unit tests with a fake ValkeyClient) fall back to scanning.
func (ai *aiManager) tasks() taskIndex {
	if ai.taskIndex == nil {
		return scanTaskIndex{logger: ai.logger, valkeyClient: ai.valkeyClient}
	}
	return ai.taskIndex
}

// scanTaskIndex is the index-less implementation: a keyspace SCAN per
// listing and one GET per task.
type scanTaskIndex struct {
	logger       *slog.Logger
	valkeyClient valkeyclient.ValkeyClient
}

func (s scanTaskIndex) add(string)       {}
func (s scanTaskIndex) remove(...string) {}

func (s scanTaskIndex) keys(pattern string) ([]string, error) {
	return s.valkeyClient.Keys(pattern)
}

func (s scanTaskIndex) load(keys []string) []aiTaskWithKey {
	tasks := make([]aiTaskWithKey, 0, len(keys))
	for _, key := range keys {
		raw, err := s.valkeyClient.Get(key)
		if err != nil || raw == "" {
			continue
		}
		if task, ok := unmarshalTask(s.logger, key, raw); ok {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// zsetTaskIndex is the Valkey ZSET implementation described above.
type zsetTaskIndex struct {
	logger       *slog.Logger
	valkeyClient valkeyclient.ValkeyClient

	// syncedAt is the unix-nano time of the last successful resync
	// (0 = never); mu serializes resyncs.
	mu       sync.Mutex
	syncedAt atomic.Int64
}

func newZsetTaskIndex(logger *slog.Logger, valkeyClient valkeyclient.ValkeyClient) *zsetTaskIndex {
	return &zsetTaskIndex{logger: logger, valkeyClient: valkeyClient}
}

// add indexes or refreshes a task key. Called right after the task is
// written with ValkeyAiTTL, so the score mirrors the key's TTL.
func (idx *zsetTaskIndex) add(key string) {
	client := idx.valkeyClient.GetValkeyClient()
	expiresAt := float64(time.Now().Add(ValkeyAiTTL).UnixMilli())
	err := client.Do(idx.valkeyClient.GetContext(),
		client.B().Zadd().Key(aiTaskIndexKey).ScoreMember().ScoreMember(expiresAt, key).Build()).Error()
	if err != nil {
		// the next resync picks the key up again
		idx.logger.Warn("failed to index AI task", "key", key, "error", err)
	}
}

func (idx *zsetTaskIndex) remove(keys ...string) {
	if len(keys) == 0 {
		return
	}
	client := idx.valkeyClient.GetValkeyClient()
	err := client.Do(idx.valkeyClient.GetContext(),
		client.B().Zrem().Key(aiTaskIndexKey).Member(keys...).Build()).Error()
	if err != nil {
		idx.logger.Warn("failed to unindex AI tasks", "keys", keys, "error", err)
	}
}

// keys returns the live task keys matching pattern, ordered by expiry (i.e.
// oldest update first). Only '*' is supported as wildcard, see
// taskKeyMatches.
func (idx *zsetTaskIndex) keys(pattern string) ([]string, error) {
	if err := idx.ensure(); err != nil {
		return nil, err
	}
	client := idx.valkeyClient.GetValkeyClient()
	now := strconv.FormatInt(time.Now().UnixMilli(), 10)
	members, err := client.Do(idx.valkeyClient.GetContext(),
		client.B().Zrangebyscore().Key(aiTaskIndexKey).Min("("+now).Max("+inf").Build()).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("read AI task index: %w", err)
	}
	keys := make([]string, 0, len(members))
	for _, member := range members {
		if taskKeyMatches(pattern, member) {
			keys = append(keys, member)
		}
	}
	return keys, nil
}

// load fetches tasks in pipelined batches instead of one round trip each.
func (idx *zsetTaskIndex) load(keys []string) []aiTaskWithKey {
	client := idx.valkeyClient.GetValkeyClient()
	ctx := idx.valkeyClient.GetContext()
	tasks := make([]aiTaskWithKey, 0, len(keys))
	for chunk := range slices.Chunk(keys, valkeyBatchSize) {
		cmds := make([]valkey.Completed, len(chunk))
		for i, key := range chunk {
			cmds[i] = client.B().Get().Key(key).Build()
		}
		for i, resp := range client.DoMulti(ctx, cmds...) {
			raw, err := resp.ToString()
			if err != nil {
				// nil = deleted or expired since it was listed
				if !valkey.IsValkeyNil(err) {
					idx.logger.Warn("failed to get AI task", "key", chunk[i], "error", err)
				}
				continue
			}
			if task, ok := unmarshalTask(idx.logger, chunk[i], raw); ok {
				tasks = append(tasks, task)
			}
		}
	}
	return tasks
}

// ensure resyncs the index when it was never synced on this replica or the
// last sync is older than aiTaskIndexResyncInterval. The first sync blocks
// callers (an unsynced index may miss pre-existing tasks); later ones run in
// whichever caller gets there first while the rest keep reading the current
// index.
func (idx *zsetTaskIndex) ensure() error {
	fresh := func(syncedAt int64) bool {
		return syncedAt != 0 && time.Since(time.Unix(0, syncedAt)) < aiTaskIndexResyncInterval
	}
	syncedAt := idx.syncedAt.Load()
	if fresh(syncedAt) {
		return nil
	}
	if syncedAt == 0 {
		idx.mu.Lock()
	} else if !idx.mu.TryLock() {
		return nil
	}
	defer idx.mu.Unlock()

	syncedAt = idx.syncedAt.Load()
	if fresh(syncedAt) {
		return nil
	}
	if err := idx.resync(); err != nil {
		if syncedAt == 0 {
			return fmt.Errorf("sync AI task index: %w", err)
		}
		idx.logger.Warn("failed to resync AI task index, serving the current one", "error", err)
		return nil
	}
	idx.syncedAt.Store(time.Now().UnixNano())
	return nil
}

// resync reconciles the index with the keyspace: every existing task key is
// (re)added with its actual expiry, members whose key no longer exists are
// removed.
func (idx *zsetTaskIndex) resync() error {
	client := idx.valkeyClient.GetValkeyClient()
	ctx := idx.valkeyClient.GetContext()

	keys, err := idx.valkeyClient.Keys(DB_AI_BUCKET_TASKS + ":*")
	if err != nil {
		return err
	}

	now := time.Now()
	var writes []valkey.Completed
	for chunk := range slices.Chunk(keys, valkeyBatchSize) {
		pttls := make([]valkey.Completed, len(chunk))
		for i, key := range chunk {
			pttls[i] = client.B().Pttl().Key(key).Build()
		}
		zadd := client.B().Zadd().Key(aiTaskIndexKey).ScoreMember()
		added := 0
		for i, resp := range client.DoMulti(ctx, pttls...) {
			pttl, err := resp.AsInt64()
			if err != nil {
				return err
			}
			var score float64
			switch pttl {
			case -2: // gone since the scan
				continue
			case -1: // no expiry
				score = math.Inf(1)
			default:
				score = float64(now.Add(time.Duration(pttl) * time.Millisecond).UnixMilli())
			}
			zadd = zadd.ScoreMember(score, chunk[i])
			added++
		}
		if added > 0 {
			writes = append(writes, zadd.Build())
		}
	}

	// Members the scan did not see are only dropped once EXISTS confirms the
	// key is gone: a task written after the scan started is indexed but may
	// be missing from the scan result.
	members, err := client.Do(ctx, client.B().Zrange().Key(aiTaskIndexKey).Min("0").Max("-1").Build()).AsStrSlice()
	if err != nil {
		return err
	}
	scanned := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		scanned[key] = struct{}{}
	}
	var unseen []string
	for _, member := range members {
		if _, ok := scanned[member]; !ok {
			unseen = append(unseen, member)
		}
	}
	var gone []string
	for chunk := range slices.Chunk(unseen, valkeyBatchSize) {
		exists := make([]valkey.Completed, len(chunk))
		for i, key := range chunk {
			exists[i] = client.B().Exists().Key(key).Build()
		}
		for i, resp := range client.DoMulti(ctx, exists...) {
			if n, err := resp.AsInt64(); err == nil && n == 0 {
				gone = append(gone, chunk[i])
			}
		}
	}
	if len(gone) > 0 {
		writes = append(writes, client.B().Zrem().Key(aiTaskIndexKey).Member(gone...).Build())
	}
	writes = append(writes, client.B().Zremrangebyscore().Key(aiTaskIndexKey).
		Min("-inf").Max(strconv.FormatInt(now.UnixMilli(), 10)).Build())

	for _, resp := range client.DoMulti(ctx, writes...) {
		if err := resp.Error(); err != nil {
			return err
		}
	}
	idx.logger.Debug("AI task index resynced", "keys", len(keys), "removed", len(gone))
	return nil
}

func unmarshalTask(logger *slog.Logger, key, raw string) (aiTaskWithKey, bool) {
	var task AiTask
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		logger.Warn("failed to unmarshal AI task", "key", key, "error", err)
		return aiTaskWithKey{}, false
	}
	return aiTaskWithKey{key: key, task: task}, true
}

// taskKeyMatches reports whether key matches a Valkey glob pattern. Only '*'
// is supported: task key patterns are built from Kubernetes names, which
// cannot contain the other glob metacharacters ('?', '[', '\').
func taskKeyMatches(pattern, key string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == key
	}
	if !strings.HasPrefix(key, parts[0]) {
		return false
	}
	rest := key[len(parts[0]):]
	for _, part := range parts[1 : len(parts)-1] {
		i := strings.Index(rest, part)
		if i < 0 {
			return false
		}
		rest = rest[i+len(part):]
	}
	return strings.HasSuffix(rest, parts[len(parts)-1])
}
