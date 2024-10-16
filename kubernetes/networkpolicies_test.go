package kubernetes

import (
	"testing"
)

func TestRemoveAllNetworkPolicies(t *testing.T) {
	t.Skip("skipping this test for manual testing")

	RemoveAllNetworkPolicies("mogenius")
}
