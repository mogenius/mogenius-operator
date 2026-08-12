package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	ollamaapi "github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
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
	assert.NotContains(t, result, "abc-123")         // uid
	assert.NotContains(t, result, "resourceVersion") // stripped field key
	assert.NotContains(t, result, "\"999\"")         // resourceVersion value

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
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "test-cm",
			"namespace": "default",
		},
		"data": map[string]any{
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
	obj := map[string]any{
		"metadata": map[string]any{
			"name":          "test",
			"uid":           "should-be-removed",
			"managedFields": []any{"noise"},
			"generation":    int64(5),
			"selfLink":      "/apis/v1/test",
			"labels":        map[string]any{"keep": "me"},
			"annotations": map[string]any{
				"kubectl.kubernetes.io/last-applied-configuration": "huge blob",
				"useful-annotation": "keep",
			},
		},
	}

	stripVerboseFields(obj)

	meta := obj["metadata"].(map[string]any)
	assert.Nil(t, meta["uid"])
	assert.Nil(t, meta["managedFields"])
	assert.Nil(t, meta["generation"])
	assert.Nil(t, meta["selfLink"])
	assert.Equal(t, map[string]any{"keep": "me"}, meta["labels"])
	anns := meta["annotations"].(map[string]any)
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

func TestMoveCacheBreakpoint(t *testing.T) {
	messages := []anthropic.MessageParam{
		{Role: anthropic.MessageParamRoleUser, Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock("prompt")}},
	}
	cachedIdx := -1

	moveCacheBreakpoint(messages, &cachedIdx)
	assert.Equal(t, 0, cachedIdx)
	assert.Equal(t, "ephemeral", string(messages[0].Content[0].OfText.CacheControl.Type))

	// Appending a tool-result message moves the marker and clears the old one.
	messages = append(messages, anthropic.MessageParam{
		Role:    anthropic.MessageParamRoleUser,
		Content: []anthropic.ContentBlockParamUnion{anthropic.NewToolResultBlock("t1", "result", false)},
	})
	moveCacheBreakpoint(messages, &cachedIdx)
	assert.Equal(t, 1, cachedIdx)
	assert.Empty(t, string(messages[0].Content[0].OfText.CacheControl.Type))
	assert.Equal(t, "ephemeral", string(messages[1].Content[0].OfToolResult.CacheControl.Type))

	// Idempotent when nothing new was appended.
	moveCacheBreakpoint(messages, &cachedIdx)
	assert.Equal(t, 1, cachedIdx)
	assert.Equal(t, "ephemeral", string(messages[1].Content[0].OfToolResult.CacheControl.Type))
}

func TestEstimateMessagesChars(t *testing.T) {
	assert.Equal(t, 0, estimateMessagesChars(nil))
	messages := []anthropic.MessageParam{
		{Role: anthropic.MessageParamRoleUser, Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(strings.Repeat("x", 1000))}},
	}
	assert.Greater(t, estimateMessagesChars(messages), 1000)
}

func TestSummaryResourceText(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]any{
			"name":      "github-backup-1",
			"namespace": "github-backup",
			"labels":    map[string]any{"app": "backup"},
			"ownerReferences": []any{
				map[string]any{"kind": "CronJob", "name": "github-backup"},
			},
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{map[string]any{"name": "backup", "image": "backup:v1"}},
				},
			},
		},
		"status": map[string]any{
			"failed": int64(1),
			"conditions": []any{
				map[string]any{"type": "Failed", "status": "True"},
			},
		},
	}}

	out := summaryResourceText(res)
	assert.Contains(t, out, "Job/github-backup-1 ns=github-backup")
	assert.Contains(t, out, "labels: app=backup")
	assert.Contains(t, out, "ownedBy: CronJob/github-backup")
	assert.Contains(t, out, "images: backup:v1")
	assert.Contains(t, out, "status.failed: 1")
	assert.Contains(t, out, "conditions: Failed=True")
	// The summary must stay a fraction of the full manifest.
	full, _ := json.Marshal(res.Object)
	assert.Less(t, len(out), len(full))
}

// TestBuildCompactedAnthropicMessages asserts the compacted Anthropic
// conversation is structurally valid: it must be a strict user→assistant
// alternation, since the Messages API rejects consecutive same-role turns.
// A regression breaking this alternation would previously have been swallowed
// by the removed fallback path.
func TestBuildCompactedAnthropicMessages(t *testing.T) {
	original := anthropic.MessageParam{
		Role:    anthropic.MessageParamRoleUser,
		Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock("original task")},
	}

	out := buildCompactedAnthropicMessages(original, "my progress so far")

	assert.Len(t, out, 2)
	assert.Equal(t, anthropic.MessageParamRoleUser, out[0].Role)
	assert.Equal(t, anthropic.MessageParamRoleAssistant, out[1].Role)
	// Roles must strictly alternate.
	for i := 1; i < len(out); i++ {
		assert.NotEqual(t, out[i-1].Role, out[i].Role, "consecutive messages must not share a role")
	}
	// The summary is carried on the assistant turn.
	assert.NotEmpty(t, out[1].Content)
	assert.Contains(t, out[1].Content[len(out[1].Content)-1].OfText.Text, "my progress so far")
}

// TestBuildCompactedOpenAIMessages asserts the compacted OpenAI conversation is
// structurally valid: [system, user, assistant] with the summary on the final
// assistant turn and no leftover tool messages.
func TestBuildCompactedOpenAIMessages(t *testing.T) {
	system := openai.SystemMessage("you are an agent")
	user := openai.UserMessage("original task")

	out := buildCompactedOpenAIMessages(system, user, "my progress so far")

	assert.Len(t, out, 3)
	assert.NotNil(t, out[0].OfSystem, "first message must be the system prompt")
	assert.NotNil(t, out[1].OfUser, "second message must be the user prompt")
	assert.NotNil(t, out[2].OfAssistant, "third message must be the assistant summary")
	for _, m := range out {
		assert.Nil(t, m.OfTool, "compacted history must not contain dangling tool messages")
	}
}

// TestBuildCompactedOllamaMessages asserts the compacted Ollama conversation is
// structurally valid: [system, user, assistant] with the summary on the final
// assistant turn.
func TestBuildCompactedOllamaMessages(t *testing.T) {
	system := ollamaapi.Message{Role: "system", Content: "you are an agent"}
	user := ollamaapi.Message{Role: "user", Content: "original task"}

	out := buildCompactedOllamaMessages(system, user, "my progress so far")

	assert.Len(t, out, 3)
	assert.Equal(t, []string{"system", "user", "assistant"}, []string{out[0].Role, out[1].Role, out[2].Role})
	assert.Contains(t, out[2].Content, "my progress so far")
}
