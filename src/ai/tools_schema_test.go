package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAiSDKToolsHaveRequiredFields verifies that every tool in the built-in
// slices has a non-empty Name and Description, and that all Required field
// names are present as keys in InputSchema.
func TestAiSDKToolsHaveRequiredFields(t *testing.T) {
	groups := []struct {
		name  string
		tools interface{ Len() int }
	}{}
	_ = groups

	for _, tc := range []struct {
		groupName string
		count     int
	}{
		{"kubernetes", len(kubernetesAiSDKTools)},
		{"helm", len(helmAiSDKTools)},
	} {
		t.Run(tc.groupName, func(t *testing.T) {
			assert.Greater(t, tc.count, 0, "expected at least one tool in group")
		})
	}

	t.Run("kubernetes tools", func(t *testing.T) {
		assert.Equal(t, 8, len(kubernetesAiSDKTools))
		for _, tool := range kubernetesAiSDKTools {
			assert.NotEmpty(t, tool.Name, "tool Name must be set")
			assert.NotEmpty(t, tool.Description, "tool Description must be set for %s", tool.Name)
			for _, req := range tool.Required {
				_, ok := tool.InputSchema[req]
				assert.True(t, ok, "required field %q missing from InputSchema of %s", req, tool.Name)
			}
		}
	})

	t.Run("helm tools", func(t *testing.T) {
		assert.Equal(t, 19, len(helmAiSDKTools))
		for _, tool := range helmAiSDKTools {
			assert.NotEmpty(t, tool.Name, "tool Name must be set")
			assert.NotEmpty(t, tool.Description, "tool Description must be set for %s", tool.Name)
			for _, req := range tool.Required {
				_, ok := tool.InputSchema[req]
				assert.True(t, ok, "required field %q missing from InputSchema of %s", req, tool.Name)
			}
		}
	})
}

// TestAiSDKToolNamesUnique ensures no duplicate tool names within each group.
func TestAiSDKToolNamesUnique(t *testing.T) {
	for _, tc := range []struct {
		groupName string
		tools     []interface{ GetName() string }
	}{} {
		_ = tc
	}

	check := func(groupName string, names []string) {
		t.Run(groupName, func(t *testing.T) {
			seen := make(map[string]bool, len(names))
			for _, n := range names {
				assert.False(t, seen[n], "duplicate tool name %q in %s", n, groupName)
				seen[n] = true
			}
		})
	}

	k8sNames := make([]string, len(kubernetesAiSDKTools))
	for i, t := range kubernetesAiSDKTools {
		k8sNames[i] = t.Name
	}
	check("kubernetes", k8sNames)

	helmNames := make([]string, len(helmAiSDKTools))
	for i, t := range helmAiSDKTools {
		helmNames[i] = t.Name
	}
	check("helm", helmNames)
}

// TestGetKubernetesResourcesEnumPropagates verifies the detail enum is present.
func TestGetKubernetesResourcesEnumPropagates(t *testing.T) {
	var found bool
	for _, tool := range kubernetesAiSDKTools {
		if tool.Name != "get_kubernetes_resources" {
			continue
		}
		found = true
		detailRaw, ok := tool.InputSchema["detail"]
		assert.True(t, ok, "detail prop missing from get_kubernetes_resources")
		if !ok {
			break
		}
		detailMap, ok := detailRaw.(map[string]any)
		assert.True(t, ok)
		enumRaw, ok := detailMap["enum"]
		assert.True(t, ok, "enum missing from detail prop")
		enumSlice, ok := enumRaw.([]any)
		assert.True(t, ok)
		assert.Equal(t, []any{"summary", "full"}, enumSlice)
		break
	}
	assert.True(t, found, "get_kubernetes_resources tool not found")
}
