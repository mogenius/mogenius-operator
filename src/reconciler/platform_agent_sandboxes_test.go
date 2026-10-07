package reconciler

import (
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool { return &b }

// An empty config overrides nothing: the chart / platform-defaults values stand.
func TestBuildAgentSandboxValuesEmptyReturnsNil(t *testing.T) {
	values, err := buildAgentSandboxValues(&v1alpha1.AgentSandboxesConfig{})
	require.NoError(t, err)
	assert.Nil(t, values)
}

// Only the keys the spec sets are written, nested under `sandboxes`.
func TestBuildAgentSandboxValuesTemplatesSetFieldsOnly(t *testing.T) {
	values, err := buildAgentSandboxValues(&v1alpha1.AgentSandboxesConfig{
		Namespace: "agent-sandbox",
		NetworkPolicy: &v1alpha1.AgentSandboxNetworkPolicyConfig{
			Managed:                boolPtr(true),
			AdditionalBlockedCidrs: []string{"34.118.224.0/20", "fd00::/8"},
		},
	})
	require.NoError(t, err)

	sandboxes, ok := values["sandboxes"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, map[string]any{"name": "agent-sandbox"}, sandboxes["namespace"])

	np, ok := sandboxes["networkPolicy"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, true, np["managed"])
	assert.Equal(t, []string{"34.118.224.0/20", "fd00::/8"}, np["additionalBlockedCidrs"])
}

// A networkPolicy block with every field unset contributes no key.
func TestBuildAgentSandboxValuesEmptyNetworkPolicyIsOmitted(t *testing.T) {
	values, err := buildAgentSandboxValues(&v1alpha1.AgentSandboxesConfig{
		NetworkPolicy: &v1alpha1.AgentSandboxNetworkPolicyConfig{},
	})
	require.NoError(t, err)
	assert.Nil(t, values)
}

// A malformed CIDR fails the component instead of reaching the engine as a
// NetworkPolicy it cannot apply.
func TestBuildAgentSandboxValuesRejectsInvalidCidr(t *testing.T) {
	for _, cidr := range []string{"10.0.0.0", "34.118.224.0/33", "not-a-cidr", ""} {
		t.Run(cidr, func(t *testing.T) {
			_, err := buildAgentSandboxValues(&v1alpha1.AgentSandboxesConfig{
				NetworkPolicy: &v1alpha1.AgentSandboxNetworkPolicyConfig{
					AdditionalBlockedCidrs: []string{"34.118.224.0/20", cidr},
				},
			})
			assert.ErrorContains(t, err, "invalid CIDR")
		})
	}
}
