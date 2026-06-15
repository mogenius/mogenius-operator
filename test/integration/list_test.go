//go:build e2e

package integration_test

import (
	"context"
	"testing"
	"time"

	"mogenius-operator/test/integration/harness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultipleWorkspacesList creates three workspaces, verifies all three appear
// in the list, deletes one, and verifies the count drops and the right names remain.
func TestMultipleWorkspacesList(t *testing.T) {
	h := harness.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	names := []string{"ws-list-1", "ws-list-2", "ws-list-3"}
	for _, name := range names {
		resp, err := h.APIServer.Send(ctx, "create/workspace", map[string]any{
			"name":        name,
			"displayName": name,
			"resources": []any{
				map[string]any{"id": "default", "type": "namespace"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "success", resp.PayloadStatus(), "create/workspace %q must succeed", name)
	}

	// Wait until all three propagate to the Valkey store.
	h.WaitFor(ctx, t, "get/workspaces", nil, func(d harness.Datagram) bool {
		p, _ := d.Payload.(map[string]any)
		items, _ := p["data"].([]any)
		return len(items) >= 3
	})

	listResp, err := h.APIServer.Send(ctx, "get/workspaces", nil)
	require.NoError(t, err)
	p, _ := listResp.Payload.(map[string]any)
	items, _ := p["data"].([]any)
	assert.Len(t, items, 3, "list should return exactly 3 workspaces")

	// Verify all three names are present.
	got := make([]string, 0, len(items))
	for _, item := range items {
		ws, _ := item.(map[string]any)
		got = append(got, ws["name"].(string))
	}
	assert.ElementsMatch(t, names, got)

	// Delete the middle workspace.
	delResp, err := h.APIServer.Send(ctx, "delete/workspace", map[string]any{"name": "ws-list-2"})
	require.NoError(t, err)
	assert.Equal(t, "success", delResp.PayloadStatus())

	// Wait until the deletion propagates.
	h.WaitFor(ctx, t, "get/workspaces", nil, func(d harness.Datagram) bool {
		p, _ := d.Payload.(map[string]any)
		items, _ := p["data"].([]any)
		return len(items) == 2
	})

	listResp, err = h.APIServer.Send(ctx, "get/workspaces", nil)
	require.NoError(t, err)
	p, _ = listResp.Payload.(map[string]any)
	items, _ = p["data"].([]any)
	assert.Len(t, items, 2, "list should return exactly 2 workspaces after deletion")

	remaining := make([]string, 0, len(items))
	for _, item := range items {
		ws, _ := item.(map[string]any)
		remaining = append(remaining, ws["name"].(string))
	}
	assert.ElementsMatch(t, []string{"ws-list-1", "ws-list-3"}, remaining)
}
