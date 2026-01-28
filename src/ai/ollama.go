package ai

import (
	"context"
	"encoding/json"
	"fmt"
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

func (ai *aiManager) processPromptOllama(ctx context.Context, model, systemPrompt, prompt string, maxToolCalls int) (*AiResponse, int64, int, string, error) {
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
	toolCallCount := 0

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

			// Extract the arguments from the function call
			var args map[string]any
			argsBytes, err := json.Marshal(toolCall.Function.Arguments)
			if err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error marshaling tool arguments: %v", err)
			}
			err = json.Unmarshal(argsBytes, &args)
			if err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling tool arguments: %v", err)
			}

			tool, ok := toolDefinitions[toolCall.Function.Name]
			if !ok {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("unknown tool called: %s", toolCall.Function.Name)
			}
			data := tool(args, ai.valkeyClient, ai.logger)

			// Add tool result to messages
			messages = append(messages, api.Message{
				Role:    "tool",
				Content: data,
			})

		}

		// Increase global tool call count and check limit
		toolCallCount += len(toolCalls)
		if maxToolCalls > 0 && toolCallCount >= maxToolCalls {
			ai.logger.Info("Max tool call limit reached, exiting loop", "maxToolCalls", maxToolCalls, "toolCallCount", toolCallCount)

			// Try to finalize using any text presently returned
			responseText = cleanJSONResponse(responseText)
			responseBytes, removedText, err := extractJSONRobust(responseText)
			ai.logger.Info("Extracted JSON after max tool calls", "removed_text", removedText)
			if err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("max tool calls reached (%d) without final text: %v", maxToolCalls, err)
			}

			var aiResponse AiResponse
			if err := json.Unmarshal(responseBytes, &aiResponse); err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling AI response after max tool calls: %v\n%s", err, responseText)
			}

			return &aiResponse, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
		}

		// Continue the loop to get the next response with tool results
	}
}
