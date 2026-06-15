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

// TestUserCRUD exercises the full create → get → list → update → delete lifecycle
// of the User CRD through the WebSocket API.
func TestUserCRUD(t *testing.T) {
	h := harness.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create
	createResp, err := h.APIServer.Send(ctx, "create/user", map[string]any{
		"name":      "user-e2e",
		"firstName": "Alice",
		"lastName":  "Test",
		"email":     "alice@example.com",
		"subject": map[string]any{
			"kind":     "User",
			"name":     "alice",
			"apiGroup": "rbac.authorization.k8s.io",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "success", createResp.PayloadStatus(), "create/user must succeed")

	// Get — poll until the watcher populates Valkey.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/user", map[string]any{"name": "user-e2e"})
		return err == nil && r.PayloadStatus() == "success"
	}, 15*time.Second, 200*time.Millisecond, "user should become readable via get/user")

	getResp, err := h.APIServer.Send(ctx, "get/user", map[string]any{"name": "user-e2e"})
	require.NoError(t, err)
	// get/user returns the full User CRD; data is the spec/metadata map
	payload, _ := getResp.Payload.(map[string]any)
	rawData := payload["data"]
	require.NotNil(t, rawData, "get/user data must not be nil")
	userData, ok := rawData.(map[string]any)
	require.True(t, ok, "get/user data must be a map")
	spec, _ := userData["spec"].(map[string]any)
	assert.Equal(t, "alice@example.com", spec["email"])
	assert.Equal(t, "Alice", spec["firstName"])

	// List — user must appear when listing all users.
	listResp, err := h.APIServer.Send(ctx, "get/users", nil)
	require.NoError(t, err)
	assert.Equal(t, "success", listResp.PayloadStatus(), "get/users must succeed")
	listPayload, _ := listResp.Payload.(map[string]any)
	items, _ := listPayload["data"].([]any)
	assert.NotEmpty(t, items, "user list must contain at least one entry")

	// Update — change the name fields.
	updateResp, err := h.APIServer.Send(ctx, "update/user", map[string]any{
		"name":      "user-e2e",
		"firstName": "Alicia",
		"lastName":  "Test (updated)",
		"email":     "alice@example.com",
		"subject": map[string]any{
			"kind":     "User",
			"name":     "alice",
			"apiGroup": "rbac.authorization.k8s.io",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "success", updateResp.PayloadStatus(), "update/user must succeed")

	// Verify the update propagated to the Valkey store.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/user", map[string]any{"name": "user-e2e"})
		if err != nil || r.PayloadStatus() != "success" {
			return false
		}
		p, _ := r.Payload.(map[string]any)
		d, _ := p["data"].(map[string]any)
		s, _ := d["spec"].(map[string]any)
		return s["firstName"] == "Alicia"
	}, 15*time.Second, 200*time.Millisecond, "user update should propagate")

	// Delete
	deleteResp, err := h.APIServer.Send(ctx, "delete/user", map[string]any{"name": "user-e2e"})
	require.NoError(t, err)
	assert.Equal(t, "success", deleteResp.PayloadStatus(), "delete/user must succeed")

	// Verify deletion propagated.
	require.Eventually(t, func() bool {
		r, err := h.APIServer.Send(ctx, "get/user", map[string]any{"name": "user-e2e"})
		return err == nil && r.PayloadStatus() == "error"
	}, 15*time.Second, 200*time.Millisecond, "user should be gone after delete")
}
