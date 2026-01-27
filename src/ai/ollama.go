package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/store"
	"time"

	"github.com/ollama/ollama/api"
)

var ollamaTools = &[]api.Tool{}

func buildTools() {
	if ollamaTools != nil {
		return
	}

	properties := api.NewToolPropertiesMap()
	properties.Set("kind", api.ToolProperty{Type: []string{"string"}})
	properties.Set("apiVersion", api.ToolProperty{Type: []string{"string"}})
	properties.Set("name", api.ToolProperty{Type: []string{"string"}})
	properties.Set("namespace", api.ToolProperty{Type: []string{"string"}})

	ollamaTools = &[]api.Tool{
		{
			Type: "function",
			Function: api.ToolFunction{
				Name:        "get_kubernetes_resources",
				Description: "Get Kubernetes resources by kind, optionally filtered by name and namespace",
				Parameters: api.ToolFunctionParameters{
					Type:       "object",
					Properties: properties,
					Required:   []string{"kind", "apiVersion"},
				},
			},
		},
	}
}

func (ai *aiManager) processPromptOllama(ctx context.Context, model, systemPrompt, prompt string) (*AiResponse, int64, int, string, error) {
	buildTools()
	startTime := time.Now()

	client, err := ai.getOllamaClient(nil)
	if err != nil {
		return nil, 0, int(time.Since(startTime).Milliseconds()), model, err
	}

	messages := []api.Message{
		{
			Role:    "system",
			Content: systemPrompt + "\n You have access to the following tool: get_kubernetes_resources. Use it to retrieve Kubernetes resources as needed to answer the user's question accurately.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	falsePtr := false
	truePtr := true
	var tokensUsed int64 = 0

	// Loop until there are no more tool calls
	for {
		req := &api.ChatRequest{
			Model:    model,
			Messages: messages,
			Stream:   &falsePtr,
			Format:   json.RawMessage(`"json"`),
			Truncate: &truePtr,
			Shift:    &truePtr,
			Tools:    *ollamaTools,
			Options: map[string]interface{}{
				"temperature": 0.1,
			},
		}

		var responseText string
		var promptEvalCount int
		var evalCount int
		var toolCalls []api.ToolCall

		err = client.Chat(ctx, req, func(resp api.ChatResponse) error {
			responseText += resp.Message.Content
			if resp.Done {
				promptEvalCount = resp.PromptEvalCount
				evalCount = resp.EvalCount
				toolCalls = resp.Message.ToolCalls
			}
			return nil
		})

		if err != nil {
			return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, err
		}

		tokensUsed += int64(promptEvalCount + evalCount)

		// Check if there are tool calls to process
		if len(toolCalls) == 0 {
			ai.logger.Info("No tool calls found, finishing AI processing")

			responseText = cleanJSONResponse(responseText)
			responseBytes, removedText, err := extractJSONRobust(responseText)
			ai.logger.Info("Extracted JSON from AI response", "removed_text", removedText)
			if err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error extracting JSON from AI response: %v\n%s", err, responseText)
			}

			var aiResponse AiResponse
			err = json.Unmarshal(responseBytes, &aiResponse)
			if err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling AI response: %v\n%s", err, responseText)
			}

			return &aiResponse, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
		}

		// Add the assistant's response to the messages
		messages = append(messages, api.Message{
			Role:      "assistant",
			Content:   responseText,
			ToolCalls: toolCalls,
		})

		// Process each tool call
		for _, toolCall := range toolCalls {
			ai.logger.Info("Processing tool call", "tool", toolCall.Function.Name)
			if toolCall.Function.Name == "get_kubernetes_resources" {
				// Extract the arguments from the function call
				var args map[string]interface{}
				argsBytes, err := json.Marshal(toolCall.Function.Arguments)
				if err != nil {
					return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error marshaling tool arguments: %v", err)
				}
				err = json.Unmarshal(argsBytes, &args)
				if err != nil {
					return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling tool arguments: %v", err)
				}

				kind := args["kind"].(string)
				apiVersion := args["apiVersion"].(string)
				name, _ := args["name"].(string)
				namespace, _ := args["namespace"].(string)

				ai.logger.Info("Retrieving Kubernetes resources", "apiVersion", apiVersion, "kind", kind, "namespace", namespace, "name", name)
				resources, err := store.GetResource(ai.valkeyClient, apiVersion, kind, namespace, name, ai.logger)
				data := ""
				if err != nil {
					data = fmt.Sprintf("Error retrieving resources: %v", err)
				} else {
					resourceBytes, err := json.MarshalIndent(resources, "", "  ")
					if err != nil {
						data = fmt.Sprintf("Error marshaling resources: %v", err)
					} else {
						data = string(resourceBytes)
					}
				}

				// Add tool result to messages
				messages = append(messages, api.Message{
					Role:    "tool",
					Content: data,
				})
			}
		}

		// Continue the loop to get the next response with tool results
	}
}
