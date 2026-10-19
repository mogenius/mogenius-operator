package core

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"mogenius-operator/src/config"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/valkeyclient"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vgo "github.com/valkey-io/valkey-go"
)

// commandRecorder wraps a real vgo.Client and counts Do/DoMulti invocations
// (round trips), not individual Redis commands - a DoMulti pipelining N
// commands is still one round trip. Package-local: core and store each build
// their own small Valkey test fixture.
type commandRecorder struct {
	vgo.Client

	mu                 sync.Mutex
	doInvocations      int
	doMultiInvocations int
}

func (r *commandRecorder) Do(ctx context.Context, cmd vgo.Completed) vgo.ValkeyResult {
	r.mu.Lock()
	r.doInvocations++
	r.mu.Unlock()
	return r.Client.Do(ctx, cmd)
}

func (r *commandRecorder) DoMulti(ctx context.Context, multi ...vgo.Completed) []vgo.ValkeyResult {
	r.mu.Lock()
	r.doMultiInvocations++
	r.mu.Unlock()
	return r.Client.DoMulti(ctx, multi...)
}

func (r *commandRecorder) roundTrips() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.doInvocations + r.doMultiInvocations
}

func (r *commandRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.doInvocations, r.doMultiInvocations = 0, 0
}

// recordingValkeyClient wraps a real valkeyclient.ValkeyClient, routing
// GetValkeyClient() through a commandRecorder so callers that build their own
// commands against it (as the valkeyclient package's generic helpers do) get
// their round trips counted. Every other method is promoted unchanged from
// the embedded real client.
type recordingValkeyClient struct {
	valkeyclient.ValkeyClient
	recorder *commandRecorder
}

func (w *recordingValkeyClient) GetValkeyClient() vgo.Client { return w.recorder }

func newRecordingValkeyClient(t *testing.T) *recordingValkeyClient {
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
	t.Cleanup(real.Close)

	return &recordingValkeyClient{
		ValkeyClient: real,
		recorder:     &commandRecorder{Client: real.GetValkeyClient()},
	}
}

// Regression guard for: "GetMachineStatsForNodes does N sequential Valkey
// calls instead of one pipeline" (src/core/dbstats.go). The fix batches every
// node's read into one valkeyclient.GetKeyedObjectsForKeys call (a single
// DoMulti), instead of one GetMachineStatsForNode (one GET) per node.
func TestGetMachineStatsForNodesBatchesIntoOneRoundTrip(t *testing.T) {
	vc := newRecordingValkeyClient(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := NewValkeyStatsModule(logger, nil, vc, nil)

	nodeNames := []string{"node-a", "node-b", "node-c", "node-d", "node-e"}
	for _, name := range nodeNames {
		require.NoError(t, db.AddMachineStatsToDb(name, structs.MachineStats{}))
	}

	vc.recorder.reset()
	stats := db.GetMachineStatsForNodes(nodeNames)

	assert.Len(t, stats, len(nodeNames))
	assert.LessOrEqual(t, vc.recorder.roundTrips(), 1,
		"GetMachineStatsForNodes must fetch every node in one pipelined round trip, not one GET per node: issued %d Do/DoMulti calls for %d nodes",
		vc.recorder.roundTrips(), len(nodeNames))
}

// Regression guard for: "ReplaceCniData does N sequential Valkey calls
// instead of one pipeline" (src/core/dbstats.go). The fix batches every
// node's write into one valkeyclient.SetObjectsForKeys call (a single
// DoMulti), instead of one SetObject (one SET) per node - and, as a side
// effect, fixes it writing the entire input slice under every node's key
// instead of just that node's own entry.
func TestReplaceCniDataBatchesIntoOneRoundTripAndWritesPerNodeData(t *testing.T) {
	vc := newRecordingValkeyClient(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := NewValkeyStatsModule(logger, nil, vc, nil)

	data := []structs.CniData{
		{Name: "cni-a", Node: "node-a", CNIVersion: "1.0.0"},
		{Name: "cni-b", Node: "node-b", CNIVersion: "1.0.0"},
		{Name: "cni-c", Node: "node-c", CNIVersion: "1.0.0"},
	}

	vc.recorder.reset()
	db.ReplaceCniData(data)

	assert.LessOrEqual(t, vc.recorder.roundTrips(), 1,
		"ReplaceCniData must write every node's entry in one pipelined round trip, not one SET per node: issued %d Do/DoMulti calls for %d nodes",
		vc.recorder.roundTrips(), len(data))

	// Read each node's key directly (rather than through GetCniData's
	// pattern scan) to isolate this assertion to ReplaceCniData itself: each
	// node's key must unmarshal to that node's own CniData, not the entire
	// input slice duplicated under every node's key.
	for _, want := range data {
		got, err := valkeyclient.GetObjectForKey[structs.CniData](vc, DB_STATS_CNI_BUCKET_NAME, want.Node)
		require.NoError(t, err, "node %s", want.Node)
		assert.Equal(t, want, *got)
	}
}
