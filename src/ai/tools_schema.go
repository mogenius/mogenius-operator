package ai

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
)

// toolProp is one provider-neutral input property of a toolSchema. Type is a
// JSON-schema scalar type ("string", "boolean", "integer"); Enum is optional.
type toolProp struct {
	Type        string
	Description string
	Enum        []string
}

// toolSchema is the single, provider-neutral definition of a tool. Each tool is
// declared once as a toolSchema and rendered into every SDK's tool type via the
// converters below, so names, parameters and descriptions no longer drift
// between providers.
type toolSchema struct {
	Name        string
	Description string
	Props       map[string]toolProp
	Required    []string
}

// mapSchemas renders a slice of schemas into a provider slice, preserving order
// (which matters for Anthropic's cache-control marker on the last tool).
func mapSchemas[T any](schemas []toolSchema, conv func(toolSchema) T) []T {
	out := make([]T, len(schemas))
	for i, s := range schemas {
		out[i] = conv(s)
	}
	return out
}

// toOpenAI renders the schema into the OpenAI tool type.
func (t toolSchema) toOpenAI() openai.ChatCompletionToolUnionParam {
	properties := make(map[string]any, len(t.Props))
	for name, p := range t.Props {
		prop := map[string]any{"type": p.Type, "description": p.Description}
		if len(p.Enum) > 0 {
			prop["enum"] = p.Enum
		}
		properties[name] = prop
	}
	params := openai.FunctionParameters{
		"type":       "object",
		"properties": properties,
	}
	if len(t.Required) > 0 {
		params["required"] = t.Required
	}
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  params,
			},
		},
	}
}

// toAnthropic renders the schema into the Anthropic tool type.
func (t toolSchema) toAnthropic() anthropic.ToolParam {
	properties := make(map[string]any, len(t.Props))
	for name, p := range t.Props {
		prop := map[string]any{"type": p.Type, "description": p.Description}
		if len(p.Enum) > 0 {
			prop["enum"] = p.Enum
		}
		properties[name] = prop
	}
	return anthropic.ToolParam{
		Name:        t.Name,
		Description: anthropic.String(t.Description),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: properties,
			Required:   t.Required,
		},
	}
}

// toOllama renders the schema into the Ollama tool type.
func (t toolSchema) toOllama() api.Tool {
	properties := api.NewToolPropertiesMap()
	for name, p := range t.Props {
		prop := api.ToolProperty{Type: []string{p.Type}, Description: p.Description}
		if len(p.Enum) > 0 {
			prop.Enum = make([]any, len(p.Enum))
			for i, e := range p.Enum {
				prop.Enum[i] = e
			}
		}
		properties.Set(name, prop)
	}
	return api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        t.Name,
			Description: t.Description,
			Parameters: api.ToolFunctionParameters{
				Type:       "object",
				Properties: properties,
				Required:   t.Required,
			},
		},
	}
}
