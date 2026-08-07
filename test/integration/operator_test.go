package integration_test

import (
	"context"
	"testing"
	"time"

	"mogenius-operator/test/integration/harness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDescribe verifies the operator responds to the "describe" pattern.
// This is the simplest possible smoke test: no K8s writes, no Helm, no Valkey reads.
// If this passes, the operator started successfully and the WebSocket dispatch works.
func TestDescribe(t *testing.T) {
	h := harness.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.APIServer.Send(ctx, "describe", nil)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.PayloadStatus())

	// data should be a map with a "patterns" field listing all registered handlers
	data, ok := resp.PayloadData()
	require.True(t, ok, "describe response data should be a map")
	patterns, ok := data["patterns"].(map[string]any)
	require.True(t, ok, "describe response should contain patterns map")
	assert.Greater(t, len(patterns), 0, "at least one pattern must be registered")
}
