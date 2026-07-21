package ai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The Ollama SDK passes the findings item schema through an untyped Items
// field — a marshaling regression there would silently strip the schema the
// model relies on. Pin the serialized shape.
func TestSubmitAnalysisOllamaToolSchema(t *testing.T) {
	assert.Equal(t, submitAnalysisToolName, submitAnalysisOllamaTool.Function.Name)

	raw, err := json.Marshal(submitAnalysisOllamaTool)
	assert.NoError(t, err)

	var tool struct {
		Function struct {
			Parameters struct {
				Required   []string `json:"required"`
				Properties struct {
					Findings struct {
						// The SDK marshals a single-entry PropertyType as a
						// plain string.
						Type  string         `json:"type"`
						Items map[string]any `json:"items"`
					} `json:"findings"`
				} `json:"properties"`
			} `json:"parameters"`
		} `json:"function"`
	}
	assert.NoError(t, json.Unmarshal(raw, &tool))

	assert.Equal(t, []string{"findings"}, tool.Function.Parameters.Required)
	assert.Equal(t, "array", tool.Function.Parameters.Properties.Findings.Type)

	items := tool.Function.Parameters.Properties.Findings.Items
	assert.ElementsMatch(t, []any{"errorMessage", "analysis"}, items["required"])
	properties, ok := items["properties"].(map[string]any)
	assert.True(t, ok, "finding schema properties must survive serialization")
	assert.Contains(t, properties, "errorMessage")
	assert.Contains(t, properties, "analysis")
}
