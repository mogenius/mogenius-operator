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

// TestWorkspaceCRUD exercises the full create → get → update → delete lifecycle
// of the Workspace CRD through the WebSocket API.
func TestWorkspaceCRUD(t *testing.T) {
	h := harness.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create
	createResp, err := h.APIServer.Send(ctx, "create/workspace", map[string]any{
		"name":        "ws-e2e",
		"displayName": "E2E Workspace",
		"resources": []any{
			map[string]any{"id": "default", "type": "namespace"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "success", createResp.PayloadStatus(), "create/workspace must succeed")

	// Get — poll until the watcher populates Valkey from the informer event.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/workspace", map[string]any{"name": "ws-e2e"})
		return err == nil && r.PayloadStatus() == "success"
	}, 15*time.Second, 200*time.Millisecond, "workspace should become readable via get/workspace")

	getResp, err := h.APIServer.Send(ctx, "get/workspace", map[string]any{"name": "ws-e2e"})
	require.NoError(t, err)
	data, ok := getResp.PayloadData()
	require.True(t, ok, "get/workspace data must be a map")
	assert.Equal(t, "ws-e2e", data["name"])
	resources, _ := data["resources"].([]any)
	require.Len(t, resources, 1, "workspace must have one resource")
	res := resources[0].(map[string]any)
	assert.Equal(t, "default", res["id"])
	assert.Equal(t, "namespace", res["type"])

	// List — workspace must appear in the list.
	listResp, err := h.APIServer.Send(ctx, "get/workspaces", nil)
	require.NoError(t, err)
	assert.Equal(t, "success", listResp.PayloadStatus(), "get/workspaces must succeed")
	listPayload, _ := listResp.Payload.(map[string]any)
	items, _ := listPayload["data"].([]any)
	assert.NotEmpty(t, items, "workspace list must contain at least one entry")

	// Update — change the display name and swap the namespace resource.
	// Passing a non-empty resources slice avoids the replace-read-from-Valkey path.
	updateResp, err := h.APIServer.Send(ctx, "update/workspace", map[string]any{
		"name":        "ws-e2e",
		"displayName": "E2E Workspace (updated)",
		"resources": []any{
			map[string]any{"id": "kube-system", "type": "namespace"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "success", updateResp.PayloadStatus(), "update/workspace must succeed")

	// Verify the update propagated to the Valkey store.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/workspace", map[string]any{"name": "ws-e2e"})
		if err != nil || r.PayloadStatus() != "success" {
			return false
		}
		d, ok := r.PayloadData()
		if !ok {
			return false
		}
		res, _ := d["resources"].([]any)
		if len(res) == 0 {
			return false
		}
		first, _ := res[0].(map[string]any)
		return first["id"] == "kube-system"
	}, 15*time.Second, 200*time.Millisecond, "workspace update should propagate")

	// Delete
	deleteResp, err := h.APIServer.Send(ctx, "delete/workspace", map[string]any{"name": "ws-e2e"})
	require.NoError(t, err)
	assert.Equal(t, "success", deleteResp.PayloadStatus(), "delete/workspace must succeed")

	// Verify deletion propagated — get must return error once the watcher removes the key.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/workspace", map[string]any{"name": "ws-e2e"})
		return err == nil && r.PayloadStatus() == "error"
	}, 15*time.Second, 200*time.Millisecond, "workspace should be gone after delete")
}
