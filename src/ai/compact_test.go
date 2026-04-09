package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCompactResourceText(t *testing.T) {
	deploymentJSON := `{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "nginx",
			"namespace": "default",
			"uid": "abc-123",
			"resourceVersion": "999",
			"generation": 3,
			"creationTimestamp": "2024-01-15T10:00:00Z",
			"labels": {"app": "nginx", "version": "v1"},
			"annotations": {
				"kubectl.kubernetes.io/last-applied-configuration": "{\"very\":\"long json blob that wastes tokens\"}",
				"deployment.kubernetes.io/revision": "2"
			},
			"managedFields": [{"manager": "kubectl", "operation": "Apply", "fieldsV1": {"f:spec": {}}}]
		},
		"spec": {
			"replicas": 3,
			"selector": {"matchLabels": {"app": "nginx"}},
			"template": {
				"spec": {
					"containers": [
						{
							"name": "nginx",
							"image": "nginx:1.21",
							"ports": [{"containerPort": 80, "protocol": "TCP"}]
						}
					]
				}
			}
		},
		"status": {
			"replicas": 3,
			"readyReplicas": 3,
			"availableReplicas": 3,
			"conditions": [
				{"type": "Available", "status": "True", "lastTransitionTime": "2024-01-15T10:01:00Z"},
				{"type": "Progressing", "status": "True", "lastTransitionTime": "2024-01-15T10:00:30Z"}
			]
		}
	}`

	var obj unstructured.Unstructured
	err := json.Unmarshal([]byte(deploymentJSON), &obj.Object)
	assert.NoError(t, err)

	result := compactResourceText(&obj)

	// Header should have kind, name, namespace
	assert.Contains(t, result, "Deployment/nginx")
	assert.Contains(t, result, "ns=default")

	// Labels should be flat
	assert.Contains(t, result, "app=nginx")

	// Stripped fields should NOT appear
	assert.NotContains(t, result, "managedFields")
	assert.NotContains(t, result, "last-applied-configuration")
	assert.NotContains(t, result, "abc-123")          // uid
	assert.NotContains(t, result, "resourceVersion")   // stripped field key
	assert.NotContains(t, result, "\"999\"")           // resourceVersion value

	// Important spec/status should appear
	assert.Contains(t, result, "replicas")
	assert.Contains(t, result, "nginx:1.21")
	assert.Contains(t, result, "readyReplicas")

	// Should be MUCH smaller than JSON
	fullJSON, _ := json.MarshalIndent(obj.Object, "", "  ")
	ratio := float64(len(result)) / float64(len(fullJSON))
	t.Logf("Compact: %d chars, Full JSON: %d chars, Ratio: %.1f%%", len(result), len(fullJSON), ratio*100)
	assert.Less(t, ratio, 0.5, "compact should be less than 50%% of full JSON size")

	t.Logf("Compact output:\n%s", result)
}

func TestCompactResourceText_LongStrings(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "test-cm",
			"namespace": "default",
		},
		"data": map[string]interface{}{
			"short": "hello",
			"long":  strings.Repeat("x", 500),
		},
	}}

	result := compactResourceText(obj)
	// Long string should be truncated
	assert.Contains(t, result, "...(500 chars)")
	assert.Less(t, len(result), 400, "long strings should be truncated")
}

func TestStripVerboseFields(t *testing.T) {
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":           "test",
			"uid":            "should-be-removed",
			"managedFields":  []interface{}{"noise"},
			"generation":     int64(5),
			"selfLink":       "/apis/v1/test",
			"labels":         map[string]interface{}{"keep": "me"},
			"annotations": map[string]interface{}{
				"kubectl.kubernetes.io/last-applied-configuration": "huge blob",
				"useful-annotation": "keep",
			},
		},
	}

	stripVerboseFields(obj)

	meta := obj["metadata"].(map[string]interface{})
	assert.Nil(t, meta["uid"])
	assert.Nil(t, meta["managedFields"])
	assert.Nil(t, meta["generation"])
	assert.Nil(t, meta["selfLink"])
	assert.Equal(t, map[string]interface{}{"keep": "me"}, meta["labels"])
	anns := meta["annotations"].(map[string]interface{})
	assert.Nil(t, anns["kubectl.kubernetes.io/last-applied-configuration"])
	assert.Equal(t, "keep", anns["useful-annotation"])
}

func TestTruncateResult(t *testing.T) {
	short := "hello"
	assert.Equal(t, short, truncateResult(short, 100))

	long := strings.Repeat("a", 200)
	result := truncateResult(long, 50)
	assert.Equal(t, 50, len(strings.Split(result, "\n")[0]))
	assert.Contains(t, result, "truncated")
}
