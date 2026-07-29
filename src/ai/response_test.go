package ai

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFollowUpResourceLenientUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected FollowUpResource
	}{
		{
			name:  "plain string keeps the identification as resource name",
			input: `"ReplicaSet/homepage-5f8d9f9c5d (homepage namespace, 0 replicas)"`,
			expected: func() FollowUpResource {
				var f FollowUpResource
				f.ResourceName = "ReplicaSet/homepage-5f8d9f9c5d (homepage namespace, 0 replicas)"
				return f
			}(),
		},
		{
			name:  "object with name alias maps to resourceName",
			input: `{"kind":"Job","apiVersion":"batch/v1","namespace":"harbor","name":"harbor-jobservice-init"}`,
			expected: func() FollowUpResource {
				var f FollowUpResource
				f.Kind = "Job"
				f.ApiVersion = "batch/v1"
				f.Namespace = "harbor"
				f.ResourceName = "harbor-jobservice-init"
				return f
			}(),
		},
		{
			name:  "canonical object parses unchanged",
			input: `{"kind":"Deployment","plural":"deployments","apiVersion":"apps/v1","namespaced":true,"namespace":"mogenius","resourceName":"mogenius-studio"}`,
			expected: func() FollowUpResource {
				var f FollowUpResource
				f.Kind = "Deployment"
				f.Plural = "deployments"
				f.ApiVersion = "apps/v1"
				f.Namespaced = true
				f.Namespace = "mogenius"
				f.ResourceName = "mogenius-studio"
				return f
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got FollowUpResource
			err := json.Unmarshal([]byte(tt.input), &got)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}


func TestDescribeToolCall(t *testing.T) {
	assert.Equal(t,
		"list_kubernetes_resources (kind: Pod, namespace: harbor)",
		describeToolCall("list_kubernetes_resources", map[string]any{"kind": "Pod", "namespace": "harbor", "apiVersion": "v1"}))
	assert.Equal(t, "helm_list_releases", describeToolCall("helm_list_releases", map[string]any{}))
}


func TestAgeString(t *testing.T) {
	assert.Equal(t, "", ageString(time.Time{}))
	assert.Equal(t, "2d", ageString(time.Now().Add(-49*time.Hour)))
	assert.Equal(t, "3h", ageString(time.Now().Add(-190*time.Minute)))
	assert.Equal(t, "5m", ageString(time.Now().Add(-5*time.Minute)))
}

func TestIsResourceExcluded(t *testing.T) {
	var nilCtx *ToolContext
	assert.False(t, nilCtx.IsResourceExcluded("v1", "Pod", "default", "x"))

	tc := &ToolContext{ExcludeResources: map[string]bool{
		aiResourceKey("apps/v1", "ReplicaSet", "mogenius", "cert-manager-cainjector-5cd89979d6"): true,
	}}
	assert.True(t, tc.IsResourceExcluded("apps/v1", "ReplicaSet", "mogenius", "cert-manager-cainjector-5cd89979d6"))
	assert.False(t, tc.IsResourceExcluded("apps/v1", "ReplicaSet", "mogenius", "other"))
	assert.False(t, tc.IsResourceExcluded("v1", "Pod", "mogenius", "cert-manager-cainjector-5cd89979d6"))

	empty := &ToolContext{}
	assert.False(t, empty.IsResourceExcluded("v1", "Pod", "default", "x"))
}

