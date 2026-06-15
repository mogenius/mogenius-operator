//go:build e2e

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"mogenius-operator/test/integration/harness"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configMapDescriptor returns the ResourceDescriptor fields for v1/ConfigMap.
// Inlined into each request map because WorkloadSingleRequest/WorkloadChangeRequest
// embed ResourceDescriptor directly.
func configMapDescriptor() map[string]any {
	return map[string]any{
		"kind":       "ConfigMap",
		"plural":     "configmaps",
		"apiVersion": "v1",
		"namespaced": true,
		"namespace":  "default",
	}
}

// TestGenericResourceCRUD exercises the generic workload WebSocket endpoints
// (create/new-workload, get/workload, get/workload-list, update/workload,
// delete/workload) using a plain ConfigMap as the test resource.
//
// These endpoints work with any K8s resource type; ConfigMap is used because
// it needs no schema validation beyond the core API, exists in every envtest
// cluster, and has a simple "data" payload that is easy to assert.
func TestGenericResourceCRUD(t *testing.T) {
	h := harness.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const cmName = "e2e-cm"
	const cmNS = "default"

	createYAML := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  env: test
  version: "1"
`, cmName, cmNS)

	// ── Create ──────────────────────────────────────────────────────────────
	desc := configMapDescriptor()
	desc["yamlData"] = createYAML

	createResp, err := h.APIServer.Send(ctx, "create/new-workload", desc)
	require.NoError(t, err)
	assert.Equal(t, "success", createResp.PayloadStatus(), "create/new-workload must succeed")

	// The created resource is returned directly in data.
	createData, ok := createResp.PayloadData()
	require.True(t, ok, "create/new-workload must return a data map")
	meta, _ := createData["metadata"].(map[string]any)
	assert.Equal(t, cmName, meta["name"])

	// ── List ─────────────────────────────────────────────────────────────────
	// Poll until the watcher populates Valkey and the CM appears in the list.
	listPayload := map[string]any{
		"kind":       "ConfigMap",
		"plural":     "configmaps",
		"apiVersion": "v1",
		"namespace":  cmNS,
	}
	h.WaitFor(ctx, t, "get/workload-list", listPayload, func(d harness.Datagram) bool {
		data, ok := d.PayloadData()
		if !ok {
			return false
		}
		items, _ := data["items"].([]any)
		for _, item := range items {
			obj, _ := item.(map[string]any)
			m, _ := obj["metadata"].(map[string]any)
			if m["name"] == cmName {
				return true
			}
		}
		return false
	})

	// ── Get ──────────────────────────────────────────────────────────────────
	getPayload := configMapDescriptor()
	getPayload["resourceName"] = cmName

	getResp := h.WaitFor(ctx, t, "get/workload", getPayload, func(d harness.Datagram) bool {
		return d.PayloadStatus() == "success"
	})

	getData, ok := getResp.PayloadData()
	require.True(t, ok, "get/workload must return a data map")
	cmData, _ := getData["data"].(map[string]any)
	assert.Equal(t, "test", cmData["env"])
	assert.Equal(t, "1", cmData["version"])

	// Extract resourceVersion for the optimistic-concurrency PUT in update.
	getMeta, _ := getData["metadata"].(map[string]any)
	rv, _ := getMeta["resourceVersion"].(string)
	require.NotEmpty(t, rv, "resourceVersion must be present in get/workload response")

	// ── Update ───────────────────────────────────────────────────────────────
	updateYAML := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
  resourceVersion: "%s"
data:
  env: test
  version: "2"
`, cmName, cmNS, rv)

	updateDesc := configMapDescriptor()
	updateDesc["yamlData"] = updateYAML

	updateResp, err := h.APIServer.Send(ctx, "update/workload", updateDesc)
	require.NoError(t, err)
	assert.Equal(t, "success", updateResp.PayloadStatus(), "update/workload must succeed")

	// Verify the new data propagates to the store.
	h.WaitFor(ctx, t, "get/workload", getPayload, func(d harness.Datagram) bool {
		if d.PayloadStatus() != "success" {
			return false
		}
		gd, ok := d.PayloadData()
		if !ok {
			return false
		}
		cd, _ := gd["data"].(map[string]any)
		return cd["version"] == "2"
	})

	// ── Delete ───────────────────────────────────────────────────────────────
	deletePayload := configMapDescriptor()
	deletePayload["resourceName"] = cmName

	deleteResp, err := h.APIServer.Send(ctx, "delete/workload", deletePayload)
	require.NoError(t, err)
	assert.Equal(t, "success", deleteResp.PayloadStatus(), "delete/workload must succeed")

	// Verify the CM is gone from the store.
	h.WaitFor(ctx, t, "get/workload", getPayload, func(d harness.Datagram) bool {
		// get/workload returns error when the resource is not in the Valkey store.
		return d.PayloadStatus() == "error"
	})
}
