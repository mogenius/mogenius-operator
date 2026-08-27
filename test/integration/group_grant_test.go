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

// TestGroupGrantCRUD exercises the full create → get → list → update → delete
// lifecycle of the GroupGrant CRD through the WebSocket API.
func TestGroupGrantCRUD(t *testing.T) {
	h := harness.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create
	createResp, err := h.APIServer.Send(ctx, "create/group-grant", map[string]any{
		"name":       "group-grant-e2e",
		"claimValue": "platform-team",
		"targetType": "workspace",
		"targetName": "ws-example",
		"role":       "editor",
	})
	require.NoError(t, err)
	assert.Equal(t, "success", createResp.PayloadStatus(), "create/group-grant must succeed")

	// Get — poll until the watcher populates Valkey.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/group-grant", map[string]any{"name": "group-grant-e2e"})
		return err == nil && r.PayloadStatus() == "success"
	}, 15*time.Second, 200*time.Millisecond, "group grant should become readable via get/group-grant")

	getResp, err := h.APIServer.Send(ctx, "get/group-grant", map[string]any{"name": "group-grant-e2e"})
	require.NoError(t, err)
	payload, _ := getResp.Payload.(map[string]any)
	rawData := payload["data"]
	require.NotNil(t, rawData, "get/group-grant data must not be nil")
	groupGrantData, ok := rawData.(map[string]any)
	require.True(t, ok, "get/group-grant data must be a map")
	spec, _ := groupGrantData["spec"].(map[string]any)
	assert.Equal(t, "platform-team", spec["claimValue"])
	assert.Equal(t, "workspace", spec["targetType"])
	assert.Equal(t, "ws-example", spec["targetName"])
	assert.Equal(t, "editor", spec["role"])

	// List — group grant must appear when listing all group grants.
	listResp, err := h.APIServer.Send(ctx, "get/group-grants", nil)
	require.NoError(t, err)
	assert.Equal(t, "success", listResp.PayloadStatus(), "get/group-grants must succeed")
	listPayload, _ := listResp.Payload.(map[string]any)
	items, _ := listPayload["data"].([]any)
	assert.NotEmpty(t, items, "group grant list must contain at least one entry")

	// Update — switch target to the whole cluster (targetName may be empty).
	updateResp, err := h.APIServer.Send(ctx, "update/group-grant", map[string]any{
		"name":       "group-grant-e2e",
		"claimValue": "platform-team",
		"targetType": "cluster",
		"targetName": "",
		"role":       "admin",
	})
	require.NoError(t, err)
	assert.Equal(t, "success", updateResp.PayloadStatus(), "update/group-grant must succeed")

	// Verify the update propagated to the Valkey store.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/group-grant", map[string]any{"name": "group-grant-e2e"})
		if err != nil || r.PayloadStatus() != "success" {
			return false
		}
		p, _ := r.Payload.(map[string]any)
		d, _ := p["data"].(map[string]any)
		s, _ := d["spec"].(map[string]any)
		return s["role"] == "admin" && s["targetType"] == "cluster"
	}, 15*time.Second, 200*time.Millisecond, "group grant update should propagate")

	// Delete
	deleteResp, err := h.APIServer.Send(ctx, "delete/group-grant", map[string]any{"name": "group-grant-e2e"})
	require.NoError(t, err)
	assert.Equal(t, "success", deleteResp.PayloadStatus(), "delete/group-grant must succeed")

	// Verify deletion propagated.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/group-grant", map[string]any{"name": "group-grant-e2e"})
		return err == nil && r.PayloadStatus() == "error"
	}, 15*time.Second, 200*time.Millisecond, "group grant should be gone after delete")
}
