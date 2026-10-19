package valkeyclient

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	valkeyclient "github.com/valkey-io/valkey-go"
)

// commandLog is the shared, concurrency-safe backing store a commandRecorder
// and every per-node commandRecorder derived from it (see Nodes) record into.
type commandLog struct {
	mu       sync.Mutex
	commands [][]string
}

// record copies the command's argument slice immediately: Completed.Commands
// documents that its backing array is recycled once the command is passed
// into the client, so it must not be retained past that point.
func (l *commandLog) record(cmd []string) {
	cp := make([]string, len(cmd))
	copy(cp, cmd)

	l.mu.Lock()
	defer l.mu.Unlock()
	l.commands = append(l.commands, cp)
}

func (l *commandLog) snapshot() [][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([][]string, len(l.commands))
	copy(out, l.commands)
	return out
}

// commandRecorder wraps a real valkeyclient.Client and records the raw
// command (as returned by Completed.Commands()) of every Do/DoMulti call, so
// a test can assert on the exact shape of what was sent to Valkey - not just
// on the end result, which stays correct regardless of how expensively it
// was computed. scanKeys fans out over Nodes() and calls Do directly on each
// node's client rather than on the top-level client, so Nodes is overridden
// to wrap every node with a recorder sharing the same log - otherwise a scan
// would silently bypass the recorder entirely.
type commandRecorder struct {
	valkeyclient.Client
	log *commandLog
}

func newCommandRecorder(real valkeyclient.Client) *commandRecorder {
	return &commandRecorder{Client: real, log: &commandLog{}}
}

func (r *commandRecorder) Do(ctx context.Context, cmd valkeyclient.Completed) valkeyclient.ValkeyResult {
	r.log.record(cmd.Commands())
	return r.Client.Do(ctx, cmd)
}

func (r *commandRecorder) DoMulti(ctx context.Context, multi ...valkeyclient.Completed) []valkeyclient.ValkeyResult {
	for _, cmd := range multi {
		r.log.record(cmd.Commands())
	}
	return r.Client.DoMulti(ctx, multi...)
}

func (r *commandRecorder) Nodes() map[string]valkeyclient.Client {
	real := r.Client.Nodes()
	wrapped := make(map[string]valkeyclient.Client, len(real))
	for addr, c := range real {
		wrapped[addr] = &commandRecorder{Client: c, log: r.log}
	}
	return wrapped
}

// matchArg returns the value following a MATCH token in a raw command, e.g.
// ["SCAN", "0", "MATCH", "live-stats:*", "COUNT", "5000"] -> "live-stats:*".
func matchArg(cmd []string) (string, bool) {
	for i, tok := range cmd {
		if tok == "MATCH" && i+1 < len(cmd) {
			return cmd[i+1], true
		}
	}
	return "", false
}

// Regression guard for: "DeleteMultiple SCANs the full keyspace and filters
// client-side" (src/valkeyclient/valkeyclient.go, DeleteMultiple). The fix
// scans with the real pattern per loop iteration (server-side MATCH), the
// same approach ClearNonEssentialKeys already used, instead of scanning "*"
// and filtering with filepath.Match in Go. This fails against the unfixed
// implementation (which always scans with MATCH *) and passes once
// DeleteMultiple is changed to loop scanKeys per pattern.
func TestDeleteMultipleScansWithTheRealPatternNotWildcard(t *testing.T) {
	self, mr := newTestClient(t)

	// A busy-keyspace-sized pile of unrelated keys: DeleteMultiple must never
	// need to inspect these just to find the few "live-stats:*" keys.
	for i := range 500 {
		set(t, mr, fmt.Sprintf("resources:v1:Pod:ns:pod-%d", i))
	}
	set(t, mr, "live-stats:cpu:node-a")
	set(t, mr, "live-stats:memory:node-a")

	recorder := newCommandRecorder(self.valkeyClient)
	self.valkeyClient = recorder

	assert.NoError(t, self.DeleteMultiple("live-stats:*"))

	foundScan := false
	for _, cmd := range recorder.log.snapshot() {
		if len(cmd) == 0 || cmd[0] != "SCAN" {
			continue
		}
		foundScan = true
		match, ok := matchArg(cmd)
		if !assert.True(t, ok, "DeleteMultiple must SCAN with a MATCH clause, not scan everything and filter client-side: %v", cmd) {
			continue
		}
		assert.Equal(t, "live-stats:*", match,
			"DeleteMultiple must let Valkey filter by the real pattern server-side (MATCH live-stats:*), not scan the whole keyspace (MATCH *) and filter in Go afterward: %v", cmd)
	}
	assert.True(t, foundScan, "expected DeleteMultiple to issue at least one SCAN command")
}
