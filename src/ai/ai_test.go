package ai

import (
	"fmt"
	"mogenius-operator/src/assert"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetNestedStringWithJSONPath(t *testing.T) {
	deployment := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "default",
				"labels": map[string]interface{}{
					"app":     "myapp",
					"env":     "production",
					"version": "1.0.0",
				},
				"annotations": map[string]interface{}{
					"deployment.kubernetes.io/revision": "3",
				},
			},
			"spec": map[string]interface{}{
				"replicas": int64(5),
			},
			"status": map[string]interface{}{
				"replicas":          int64(3),
				"availableReplicas": int64(2),
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Available",
						"status":             "False",
						"lastTransitionTime": "2024-01-01T00:00:00Z",
						"reason":             "MinimumReplicasUnavailable",
						"message":            "Deployment does not have minimum availability.",
					},
					map[string]interface{}{
						"type":               "Progressing",
						"status":             "True",
						"lastTransitionTime": "2024-01-01T00:00:00Z",
						"reason":             "NewReplicaSetAvailable",
					},
				},
				"containerStatuses": []interface{}{
					map[string]interface{}{
						"name": "container-1",
						"state": map[string]interface{}{
							"waiting": map[string]interface{}{
								"reason":  "CrashLoopBackOff",
								"message": "Back-off restarting failed container",
							},
						},
						"restartCount": int64(5),
					},
					map[string]interface{}{
						"name": "container-2",
						"state": map[string]interface{}{
							"running": map[string]interface{}{
								"startedAt": "2024-01-01T00:00:00Z",
							},
						},
						"ready":        true,
						"restartCount": int64(0),
					},
					map[string]interface{}{
						"name": "container-3",
						"state": map[string]interface{}{
							"waiting": map[string]interface{}{
								"reason":  "ImagePullBackOff",
								"message": "Failed to pull image",
							},
						},
						"restartCount": int64(2),
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		path          string
		keyword       string
		expectFound   bool
		expectValue   string
		expectError   bool
		errorContains string
	}{
		{
			name:        "Simple string field",
			path:        ".metadata.name",
			keyword:     "",
			expectFound: true,
			expectValue: "test-deployment",
		},
		{
			name:        "Simple numeric field",
			path:        ".status.replicas",
			keyword:     "",
			expectFound: true,
			expectValue: "3",
		},
		{
			name:        "Nested field with dot notation",
			path:        `.metadata.annotations.deployment\.kubernetes\.io/revision`, // Escape dots in key name
			keyword:     "",
			expectFound: true,
			expectValue: "3",
		},
		{
			name:        "Array filter by type - Available",
			path:        ".status.conditions[?(@.type=='Available')].status",
			keyword:     "",
			expectFound: true,
			expectValue: "False",
		},
		{
			name:        "Array filter by type - Progressing",
			path:        ".status.conditions[?(@.type=='Progressing')].status",
			keyword:     "",
			expectFound: true,
			expectValue: "True",
		},
		{
			name:        "Array filter with nested field access",
			path:        ".status.conditions[?(@.type=='Available')].reason",
			keyword:     "",
			expectFound: true,
			expectValue: "MinimumReplicasUnavailable",
		},
		{
			name:        "Array index access",
			path:        ".status.containerStatuses[0].state.waiting.reason",
			keyword:     "",
			expectFound: true,
			expectValue: "CrashLoopBackOff",
		},
		{
			name:        "Array wildcard - all container names",
			path:        ".status.containerStatuses[*].name",
			keyword:     "",
			expectFound: true,
			expectValue: "container-1, container-2, container-3",
		},
		{
			name:        "Array wildcard - all restart counts",
			path:        ".status.containerStatuses[*].restartCount",
			keyword:     "",
			expectFound: true,
			expectValue: "5, 0, 2",
		},
		{
			name:        "Array wildcard with nested path - waiting reasons",
			path:        ".status.containerStatuses[*].state.waiting.reason",
			keyword:     "",
			expectFound: true,
			expectValue: "CrashLoopBackOff, ImagePullBackOff", // Only containers with waiting state
		},
		{
			name:        "Filter containers by state type",
			path:        ".status.containerStatuses[?(@.state.waiting)].name",
			keyword:     "",
			expectFound: true,
			expectValue: "container-1, container-3",
		},
		{
			name:        "Map with keyword search - label search",
			path:        ".metadata.labels",
			keyword:     "app",
			expectFound: true,
			expectValue: "app=myapp",
		},
		{
			name:        "Map with keyword search - value match",
			path:        ".metadata.labels",
			keyword:     "production",
			expectFound: true,
			expectValue: "env=production",
		},
		{
			name:        "Non-existent path",
			path:        ".status.nonexistent",
			keyword:     "",
			expectFound: false,
		},
		{
			name:        "Non-existent array filter",
			path:        ".status.conditions[?(@.type=='NonExistent')].status",
			keyword:     "",
			expectFound: false,
		},
		{
			name:        "Array index out of bounds",
			path:        ".status.containerStatuses[99].name",
			keyword:     "",
			expectFound: false,
		},
		{
			name:        "Filter with no matches",
			path:        ".status.conditions[?(@.status=='Unknown')].type",
			keyword:     "",
			expectFound: false,
		},
		{
			name:        "Boolean field",
			path:        ".status.containerStatuses[1].ready",
			keyword:     "",
			expectFound: true,
			expectValue: "true",
		},
		{
			name:        "Multiple array filters chained",
			path:        ".status.conditions[?(@.type=='Available')].message",
			keyword:     "",
			expectFound: true,
			expectValue: "Deployment does not have minimum availability.",
		},
		{
			name:        "Spec field access",
			path:        ".spec.replicas",
			keyword:     "",
			expectFound: true,
			expectValue: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Running test %s %s", tt.name, tt.path)
			value, found, err := getNestedStringWithJSONPath(&deployment, tt.path, tt.keyword)

			if tt.expectError {
				assert.AssertT(t, err != nil, "Expected error but got none")
				if tt.errorContains != "" {
					assert.AssertT(t, strings.Contains(err.Error(), tt.errorContains),
						fmt.Sprintf("Expected error to contain '%s', got: %v", tt.errorContains, err))
				}
				return
			}

			assert.AssertT(t, err == nil, fmt.Sprintf("Unexpected error: %v", err))
			assert.AssertT(t, found == tt.expectFound,
				fmt.Sprintf("Expected found=%v, got found=%v", tt.expectFound, found))

			if tt.expectFound {
				assert.AssertT(t, value == tt.expectValue,
					fmt.Sprintf("Expected value='%s', got value='%s'", tt.expectValue, value))
			}

			t.Logf("Path: %s, Value: %s, Found: %v", tt.path, value, found)
		})
	}
}
