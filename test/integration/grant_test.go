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

// TestGrantCRUD exercises the full create → get → list → update → delete lifecycle
// of the Grant CRD through the WebSocket API.
func TestGrantCRUD(t *testing.T) {
	h := harness.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create
	createResp, err := h.APIServer.Send(ctx, "create/grant", map[string]any{
		"name":       "grant-e2e",
		"grantee":    "alice",
		"targetType": "workspace",
		"targetName": "ws-example",
		"role":       "editor",
	})
	require.NoError(t, err)
	assert.Equal(t, "success", createResp.PayloadStatus(), "create/grant must succeed")

	// Get — poll until the watcher populates Valkey.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/grant", map[string]any{"name": "grant-e2e"})
		return err == nil && r.PayloadStatus() == "success"
	}, 15*time.Second, 200*time.Millisecond, "grant should become readable via get/grant")

	getResp, err := h.APIServer.Send(ctx, "get/grant", map[string]any{"name": "grant-e2e"})
	require.NoError(t, err)
	payload, _ := getResp.Payload.(map[string]any)
	rawData := payload["data"]
	require.NotNil(t, rawData, "get/grant data must not be nil")
	grantData, ok := rawData.(map[string]any)
	require.True(t, ok, "get/grant data must be a map")
	spec, _ := grantData["spec"].(map[string]any)
	assert.Equal(t, "alice", spec["grantee"])
	assert.Equal(t, "workspace", spec["targetType"])
	assert.Equal(t, "ws-example", spec["targetName"])
	assert.Equal(t, "editor", spec["role"])

	// List — grant must appear when listing all grants.
	listResp, err := h.APIServer.Send(ctx, "get/grants", nil)
	require.NoError(t, err)
	assert.Equal(t, "success", listResp.PayloadStatus(), "get/grants must succeed")
	listPayload, _ := listResp.Payload.(map[string]any)
	items, _ := listPayload["data"].([]any)
	assert.NotEmpty(t, items, "grant list must contain at least one entry")

	// List with filter — filter by targetType and targetName.
	filteredResp, err := h.APIServer.Send(ctx, "get/grants", map[string]any{
		"targetType": "workspace",
		"targetName": "ws-example",
	})
	require.NoError(t, err)
	assert.Equal(t, "success", filteredResp.PayloadStatus(), "get/grants with filter must succeed")
	fp, _ := filteredResp.Payload.(map[string]any)
	filteredItems, _ := fp["data"].([]any)
	assert.NotEmpty(t, filteredItems, "filtered grant list must contain the matching grant")

	// Update — promote grantee to admin.
	updateResp, err := h.APIServer.Send(ctx, "update/grant", map[string]any{
		"name":       "grant-e2e",
		"grantee":    "alice",
		"targetType": "workspace",
		"targetName": "ws-example",
		"role":       "admin",
	})
	require.NoError(t, err)
	assert.Equal(t, "success", updateResp.PayloadStatus(), "update/grant must succeed")

	// Verify the update propagated to the Valkey store.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/grant", map[string]any{"name": "grant-e2e"})
		if err != nil || r.PayloadStatus() != "success" {
			return false
		}
		p, _ := r.Payload.(map[string]any)
		d, _ := p["data"].(map[string]any)
		s, _ := d["spec"].(map[string]any)
		return s["role"] == "admin"
	}, 15*time.Second, 200*time.Millisecond, "grant role update should propagate")

	// Delete
	deleteResp, err := h.APIServer.Send(ctx, "delete/grant", map[string]any{"name": "grant-e2e"})
	require.NoError(t, err)
	assert.Equal(t, "success", deleteResp.PayloadStatus(), "delete/grant must succeed")

	// Verify deletion propagated.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/grant", map[string]any{"name": "grant-e2e"})
		return err == nil && r.PayloadStatus() == "error"
	}, 15*time.Second, 200*time.Millisecond, "grant should be gone after delete")
}
