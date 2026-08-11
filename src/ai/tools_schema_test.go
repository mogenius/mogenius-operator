package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestToolSchemaConvertersAgree locks the three per-SDK converters against
// regressions: every generated slice must have one entry per schema, in order,
// with matching names and required sets. This guards the neutral-schema
// refactor — a converter dropping a param or reordering tools fails here.
func TestToolSchemaConvertersAgree(t *testing.T) {
	groups := []struct {
		name    string
		schemas []toolSchema
	}{
		{"kubernetes", kubernetesToolSchemas},
		{"helm", helmToolSchemas},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			var openaiTools = mapSchemas(g.schemas, toolSchema.toOpenAI)
			var anthropicTools = mapSchemas(g.schemas, toolSchema.toAnthropic)
			var ollamaTools = mapSchemas(g.schemas, toolSchema.toOllama)

			assert.Len(t, openaiTools, len(g.schemas))
			assert.Len(t, anthropicTools, len(g.schemas))
			assert.Len(t, ollamaTools, len(g.schemas))

			for i, s := range g.schemas {
				// Names align across providers and preserve order.
				assert.Equal(t, s.Name, openaiTools[i].OfFunction.Function.Name, "openai name at %d", i)
				assert.Equal(t, s.Name, anthropicTools[i].Name, "anthropic name at %d", i)
				assert.Equal(t, s.Name, ollamaTools[i].Function.Name, "ollama name at %d", i)

				// Required sets are carried through verbatim.
				assert.Equal(t, s.Required, anthropicTools[i].InputSchema.Required, "anthropic required for %s", s.Name)
				assert.Equal(t, s.Required, ollamaTools[i].Function.Parameters.Required, "ollama required for %s", s.Name)

				openaiParams := map[string]any(openaiTools[i].OfFunction.Function.Parameters)
				if len(s.Required) > 0 {
					assert.Equal(t, s.Required, openaiParams["required"], "openai required for %s", s.Name)
				} else {
					_, ok := openaiParams["required"]
					assert.False(t, ok, "openai should omit empty required for %s", s.Name)
				}
			}
		})
	}
}

// TestToolSchemaEnumPropagates verifies the optional Enum field survives every
// converter, using get_kubernetes_resources' detail property.
func TestToolSchemaEnumPropagates(t *testing.T) {
	var schema toolSchema
	for _, s := range kubernetesToolSchemas {
		if s.Name == "get_kubernetes_resources" {
			schema = s
			break
		}
	}
	assert.Equal(t, []string{"summary", "full"}, schema.Props["detail"].Enum)

	anthropicProps := schema.toAnthropic().InputSchema.Properties.(map[string]any)
	detail := anthropicProps["detail"].(map[string]any)
	assert.Equal(t, []string{"summary", "full"}, detail["enum"])

	ollamaProps := schema.toOllama().Function.Parameters.Properties
	ollamaDetail, ok := ollamaProps.Get("detail")
	assert.True(t, ok)
	assert.Equal(t, []any{"summary", "full"}, ollamaDetail.Enum)
}
