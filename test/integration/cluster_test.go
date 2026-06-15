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

// TestClusterResourceInfo verifies the cluster/resource-info pattern returns a
// well-formed response.  In envtest there are no real nodes or load-balancers,
// so the test only checks structural correctness (expected fields present, no
// fatal error) rather than specific values.
func TestClusterResourceInfo(t *testing.T) {
	h := harness.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := h.APIServer.Send(ctx, "cluster/resource-info", nil)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.PayloadStatus(), "cluster/resource-info must succeed")

	data, ok := resp.PayloadData()
	require.True(t, ok, "cluster/resource-info must return a data map")

	// These keys must always be present; values may be empty in envtest.
	assert.Contains(t, data, "loadBalancerExternalIps", "response must contain loadBalancerExternalIps")
	assert.Contains(t, data, "nodeStats", "response must contain nodeStats")
	assert.Contains(t, data, "provider", "response must contain provider")
	assert.Contains(t, data, "cniConfig", "response must contain cniConfig")

	// Partial errors are surfaced via the "error" field; in envtest the cluster
	// is synthetic so some providers may fail but the response must still arrive.
	// We do NOT assert the field is absent — just that it is not crashing.
	_ = data["error"]
}
