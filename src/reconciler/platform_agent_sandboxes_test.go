package reconciler

import (
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func boolPtr(b bool) *bool { return &b }

// An empty config overrides nothing: the chart / platform-defaults values stand.
func TestBuildAgentSandboxValuesEmptyReturnsNil(t *testing.T) {
	assert.Nil(t, buildAgentSandboxValues(&v1alpha1.AgentSandboxesConfig{}))
}

// Only the keys the spec sets are written, nested under `sandboxes`.
func TestBuildAgentSandboxValuesTemplatesSetFieldsOnly(t *testing.T) {
	values := buildAgentSandboxValues(&v1alpha1.AgentSandboxesConfig{
		Namespace: "agent-sandbox",
		NetworkPolicy: &v1alpha1.AgentSandboxNetworkPolicyConfig{
			Managed:                boolPtr(true),
			AdditionalBlockedCidrs: []string{"34.118.224.0/20"},
		},
	})

	sandboxes, ok := values["sandboxes"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, map[string]any{"name": "agent-sandbox"}, sandboxes["namespace"])

	np, ok := sandboxes["networkPolicy"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, true, np["managed"])
	assert.Equal(t, []string{"34.118.224.0/20"}, np["additionalBlockedCidrs"])
}

// A networkPolicy block with every field unset contributes no key.
func TestBuildAgentSandboxValuesEmptyNetworkPolicyIsOmitted(t *testing.T) {
	values := buildAgentSandboxValues(&v1alpha1.AgentSandboxesConfig{
		NetworkPolicy: &v1alpha1.AgentSandboxNetworkPolicyConfig{},
	})
	assert.Nil(t, values)
}
