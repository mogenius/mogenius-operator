package store

import (
	"errors"
	"strings"
	"testing"
	"time"

	"mogenius-operator/src/structs"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func secretObj(data map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": "my-secret", "namespace": "prod"},
		"data":       data,
	}}
}

func TestRedactSecretDataReplacesValuesAndKeepsKeys(t *testing.T) {
	original := secretObj(map[string]any{"apiKey": "c3VwZXItc2VjcmV0", "other": "dmFsdWU="})

	redacted := redactSecretData(original)

	data, _, _ := unstructured.NestedMap(redacted.Object, "data")
	assert.Len(t, data, 2)
	for key, value := range data {
		str, ok := value.(string)
		assert.True(t, ok)
		assert.Contains(t, str, "REDACTED", "value of %q must be redacted", key)
		assert.NotContains(t, str, "c3VwZXItc2VjcmV0")
	}

	// Same input value must produce the same placeholder, different values different ones.
	assert.NotEqual(t, data["apiKey"], data["other"])

	// The caller's object must not be mutated.
	originalData, _, _ := unstructured.NestedMap(original.Object, "data")
	assert.Equal(t, "c3VwZXItc2VjcmV0", originalData["apiKey"])
}

func TestRedactSecretDataPassesThroughNonSecrets(t *testing.T) {
	deployment := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": "app"},
	}}
	assert.Same(t, deployment, redactSecretData(deployment))
	assert.Nil(t, redactSecretData(nil))
}

func TestSanitizeAuditLogEntryRedactsSecretYamlAndSensitiveKeys(t *testing.T) {
	secretYaml := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: db\n  namespace: prod\ndata:\n  password: cGFzc3dvcmQ=\n"
	entry := AuditLogEntry{
		Pattern: "update/workload",
		Payload: map[string]any{
			"yamlData": secretYaml,
			"password": "repo-password",
			"nested":   map[string]any{"token": "tok-123", "name": "keep-me"},
		},
	}

	sanitizeAuditLogEntry(&entry)

	payload := entry.Payload.(map[string]any)
	yamlData := payload["yamlData"].(string)
	assert.NotContains(t, yamlData, "cGFzc3dvcmQ=")
	assert.Contains(t, yamlData, "REDACTED")
	assert.Contains(t, yamlData, "password:", "the key name must stay visible")

	assert.NotContains(t, payload["password"], "repo-password")
	nested := payload["nested"].(map[string]any)
	assert.NotContains(t, nested["token"], "tok-123")
	assert.Equal(t, "keep-me", nested["name"])
}

func TestSanitizeAuditLogEntryRedactsUnstructuredSecretResult(t *testing.T) {
	entry := AuditLogEntry{
		Pattern: "update/workload",
		Result:  secretObj(map[string]any{"apiKey": "c3VwZXItc2VjcmV0"}),
	}

	sanitizeAuditLogEntry(&entry)

	result := entry.Result.(*unstructured.Unstructured)
	data, _, _ := unstructured.NestedMap(result.Object, "data")
	assert.NotContains(t, data["apiKey"], "c3VwZXItc2VjcmV0")
}

func TestSanitizeAuditLogEntryRedactsAiModelApiKey(t *testing.T) {
	entry := AuditLogEntry{
		Pattern: "create/aimodel",
		Payload: map[string]any{
			"name":   "claude",
			"spec":   map[string]any{"sdk": "anthropic", "model": "claude-sonnet-5"},
			"apiKey": "sk-ant-live-abc123",
		},
	}

	sanitizeAuditLogEntry(&entry)

	payload := entry.Payload.(map[string]any)
	assert.NotContains(t, payload["apiKey"], "sk-ant-live-abc123")
	assert.Contains(t, payload["apiKey"], "REDACTED")
	assert.Equal(t, "claude", payload["name"])
}

func TestSanitizeAuditLogEntryLeavesNonSecretPayloadUntouched(t *testing.T) {
	payload := map[string]any{"namespace": "prod", "name": "app"}
	entry := AuditLogEntry{Pattern: "delete/workload", Payload: payload}

	sanitizeAuditLogEntry(&entry)

	assert.Equal(t, "prod", entry.Payload.(map[string]any)["namespace"])
	assert.Equal(t, "app", entry.Payload.(map[string]any)["name"])
}

func TestAuditLogFromDatagramSetsSuccessRequestIdAndCreatedAtFallback(t *testing.T) {
	datagram := structs.Datagram{Id: "req-1", Pattern: "delete/workload"}

	entry := auditLogFromDatagram(datagram, "ok", nil)
	assert.True(t, entry.Success)
	assert.Equal(t, "req-1", entry.RequestId)
	assert.WithinDuration(t, time.Now(), entry.CreatedAt, time.Minute)

	failed := auditLogFromDatagram(datagram, "", errors.New("boom"))
	assert.False(t, failed.Success)
	assert.Equal(t, "boom", failed.Error)
}

func TestAuditLogEntryMatchesSearchTopLevelFields(t *testing.T) {
	entry := AuditLogEntry{
		Pattern:   "update/workload",
		Kind:      "Deployment",
		Namespace: "prod",
		Name:      "checkout-api",
	}

	for _, term := range []string{"deployment", "prod", "checkout"} {
		assert.True(t, auditLogEntryMatchesSearch(entry, strings.ToLower(term)), "term %q", term)
	}
	assert.False(t, auditLogEntryMatchesSearch(entry, "unrelated"))
}
