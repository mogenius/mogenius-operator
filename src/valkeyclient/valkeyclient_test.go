package valkeyclient

import (
	"context"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	valkeyclient "github.com/valkey-io/valkey-go"
)

// newTestClient wires a valkeyClient against an in-memory server. Nodes() on a
// standalone client returns the single connection, which is the fan-out path
// scanKeys takes for a cluster.
func newTestClient(t *testing.T) (*valkeyClient, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	client, err := valkeyclient.NewClient(valkeyclient.ClientOption{
		InitAddress:  []string{mr.Addr()},
		DisableCache: true,
	})
	assert.NoError(t, err)
	t.Cleanup(client.Close)

	return &valkeyClient{
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		ctx:          context.Background(),
		valkeyClient: client,
	}, mr
}

func set(t *testing.T, mr *miniredis.Miniredis, key string) {
	t.Helper()
	assert.NoError(t, mr.Set(key, "{}"))
}

func TestKeysReturnsEveryMatchingKey(t *testing.T) {
	self, mr := newTestClient(t)

	// More keys than a single SCAN batch returns, so the cursor has to be
	// carried across round trips.
	for i := range 250 {
		set(t, mr, "resources:v1:Node::node-"+strconv.Itoa(i))
	}
	set(t, mr, "resources:v1:Pod:ns:some-pod")

	keys, err := self.Keys("resources:v1:Node:*")
	assert.NoError(t, err)
	assert.Len(t, keys, 250)

	all, err := self.Keys("*")
	assert.NoError(t, err)
	assert.Len(t, all, 251)
}

func TestKeysDeduplicatesAcrossNodes(t *testing.T) {
	self, mr := newTestClient(t)

	set(t, mr, "a:1")
	set(t, mr, "a:2")

	keys, err := self.scanKeys("a:*", 1)
	assert.NoError(t, err)

	sort.Strings(keys)
	assert.Equal(t, []string{"a:1", "a:2"}, keys)
}

func TestDeleteMultipleOnlyDeletesMatchingPatterns(t *testing.T) {
	self, mr := newTestClient(t)

	set(t, mr, "live-stats:cpu:node-a")
	set(t, mr, "live-stats:memory:node-a")
	set(t, mr, "resources:v1:Node::node-a")

	assert.NoError(t, self.DeleteMultiple("live-stats:*"))

	keys, err := self.Keys("*")
	assert.NoError(t, err)
	assert.Equal(t, []string{"resources:v1:Node::node-a"}, keys)
}
