package store

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"mogenius-operator/src/config"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vgo "github.com/valkey-io/valkey-go"
)

// commandRecorder wraps a real vgo.Client and records the raw command (as
// returned by Completed.Commands()) of every Do/DoMulti call, so a test can
// assert on the exact shape of what was sent to Valkey (e.g. a bounded vs.
// unbounded ZREVRANGE), not just on the end result. Package-local: each
// package with these regression tests builds its own small fixture.
type commandRecorder struct {
	vgo.Client

	mu                 sync.Mutex
	commands           [][]string
	doInvocations      int
	doMultiInvocations int
}

func (r *commandRecorder) Do(ctx context.Context, cmd vgo.Completed) vgo.ValkeyResult {
	r.mu.Lock()
	r.doInvocations++
	r.mu.Unlock()
	r.record(cmd.Commands())
	return r.Client.Do(ctx, cmd)
}

func (r *commandRecorder) DoMulti(ctx context.Context, multi ...vgo.Completed) []vgo.ValkeyResult {
	r.mu.Lock()
	r.doMultiInvocations++
	r.mu.Unlock()
	for _, cmd := range multi {
		r.record(cmd.Commands())
	}
	return r.Client.DoMulti(ctx, multi...)
}

// record copies the command's argument slice immediately: Completed.Commands
// documents that its backing array is recycled once the command is passed
// into the client, so it must not be retained past that point.
func (r *commandRecorder) record(cmd []string) {
	cp := make([]string, len(cmd))
	copy(cp, cmd)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands = append(r.commands, cp)
}

func (r *commandRecorder) snapshot() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]string, len(r.commands))
	copy(out, r.commands)
	return out
}

// roundTrips is the number of Do/DoMulti calls (network round trips), not
// the number of individual commands - a DoMulti pipelining 20 commands is
// still one round trip.
func (r *commandRecorder) roundTrips() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.doInvocations + r.doMultiInvocations
}

func (r *commandRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands = nil
	r.doInvocations, r.doMultiInvocations = 0, 0
}

// recordingValkeyClient wraps a real valkeyclient.ValkeyClient, routing
// GetValkeyClient() through a commandRecorder. Every other method is
// promoted unchanged from the embedded real client.
type recordingValkeyClient struct {
	valkeyclient.ValkeyClient
	recorder *commandRecorder
}

func (w *recordingValkeyClient) GetValkeyClient() vgo.Client { return w.recorder }

func wrapRecording(real valkeyclient.ValkeyClient) *recordingValkeyClient {
	return &recordingValkeyClient{ValkeyClient: real, recorder: &commandRecorder{Client: real.GetValkeyClient()}}
}

// newRecordingIndexTestStore is newIndexTestStore (see resourceindex_test.go)
// but points the package-level valkeyClient at a recordingValkeyClient, for
// tests of functions (like the audit log list path) that read the
// package-level client directly instead of taking one as a parameter.
func newRecordingIndexTestStore(t *testing.T) *commandRecorder {
	t.Helper()

	mr := miniredis.RunT(t)

	cfg := config.NewConfig()
	for key, value := range map[string]string{
		"MO_VALKEY_ADDR":                 mr.Addr(),
		"MO_VALKEY_USERNAME":             "",
		"MO_VALKEY_PASSWORD":             "",
		"MO_STATS_RETENTION_MAX_ENTRIES": "",
		"MO_STATS_RETENTION_HOURS":       "",
	} {
		cfg.Declare(config.ConfigDeclaration{Key: key, DefaultValue: &value})
	}

	real := valkeyclient.NewValkeyClient(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	require.NoError(t, real.Connect())

	wrapped := wrapRecording(real)
	previous := valkeyClient
	valkeyClient = wrapped
	auditLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	auditLogIndexEnsured.Store(false)

	t.Cleanup(func() {
		real.Close()
		valkeyClient = previous
	})

	return wrapped.recorder
}

// Regression guard for: "GetResourcesByWhitelistPaginated does sequential
// per-shard round trips" (src/store/store.go). The fix batches every shard's
// ZCARD into one DoMulti and every shard's ZRANGE/ZRANGEBYLEX into a second,
// instead of readShardTopMembers' one ZCARD + one ZRANGE per shard (2 x shard
// count round trips).
func TestGetResourcesByWhitelistPaginatedBatchesShardReads(t *testing.T) {
	newIndexTestStore(t)

	const shardCount = 20
	for i := range shardCount {
		ns := fmt.Sprintf("ns-%d", i)
		writeResource(t, utils.PodResource, ns, fmt.Sprintf("pod-%d", i))
	}

	wrapped := wrapRecording(valkeyClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	page, err := GetResourcesByWhitelistPaginated(
		wrapped, []*utils.ResourceDescriptor{&utils.PodResource}, nil, nil, 0, 5, "name", "asc", nil, logger)
	require.NoError(t, err)
	assert.Equal(t, shardCount, page.TotalCount)

	// Namespace discovery (one SMEMBERS) + the ZCARD batch + the ZRANGE batch
	// + the final page GET batch: a small constant, nowhere near the
	// 2*shardCount round trips the unbatched version needs.
	assert.LessOrEqual(t, wrapped.recorder.roundTrips(), 6,
		"must batch every shard's ZCARD into one DoMulti and every shard's ZRANGE into a second, not 2 round trips per shard: issued %d round trips for %d shards",
		wrapped.recorder.roundTrips(), shardCount)
}

// Regression guard for: "ListAuditLog pulls the entire audit index on every
// call" (src/store/store.go, listAuditLogIndexed). The cluster-wide,
// non-search fix reads a bounded ZREVRANGE(offset, offset+limit-1) plus one
// ZCARD instead of ZREVRANGE(0, -1) - the whole index - regardless of the
// requested page size.
func TestListAuditLogClusterWidePageIsBounded(t *testing.T) {
	recorder := newRecordingIndexTestStore(t)

	base := time.Now().Add(-time.Hour)
	for i := range 50 {
		writeTestAuditEntry(t, "prod", fmt.Sprintf("app-%d", i), "update/workload", base.Add(time.Duration(i)*time.Second))
	}

	recorder.reset()

	entries, total, err := ListAuditLog(5, 0, nil, true, "", "")
	require.NoError(t, err)
	assert.Equal(t, 50, total)
	assert.Len(t, entries, 5)

	for _, cmd := range recorder.snapshot() {
		if len(cmd) < 4 || cmd[0] != "ZREVRANGE" {
			continue
		}
		assert.NotEqual(t, "-1", cmd[3],
			"a page-1 limit=5 request must not ZREVRANGE the whole index (stop=-1) regardless of the requested limit: %v", cmd)
	}
}
