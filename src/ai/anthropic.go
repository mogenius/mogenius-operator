package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-operator/src/store"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

var anthropicTools = []anthropic.ToolParam{
	{
		Name:        "get_kubernetes_resource",
		Description: anthropic.String("Get a specific Kubernetes resource by name. Use this when you know the exact name of the resource you want to retrieve."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"kind": map[string]interface{}{
					"type":        "string",
					"description": "The kind of the Kubernetes resource (e.g., Pod, Deployment, Service).",
				},
				"apiVersion": map[string]interface{}{
					"type":        "string",
					"description": "The API version of the resource (e.g., v1, apps/v1).",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the resource.",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "The namespace of the resource (optional for cluster-scoped resources).",
				},
			},
			Required: []string{"kind", "apiVersion", "name"},
		},
	},
	{
		Name:        "list_kubernetes_resources",
		Description: anthropic.String("List all Kubernetes resources of a given kind, optionally filtered by namespace. Use this when you want to see multiple resources or don't know the exact name."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"kind": map[string]interface{}{
					"type":        "string",
					"description": "The kind of the Kubernetes resource (e.g., Pod, Deployment, Service).",
				},
				"apiVersion": map[string]interface{}{
					"type":        "string",
					"description": "The API version of the resource (e.g., v1, apps/v1).",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "The namespace to list resources from (optional; omit to list from all namespaces or cluster-scoped resources).",
				},
			},
			Required: []string{"kind", "apiVersion"},
		},
	},
}

func (ai *aiManager) processPromptAnthropic(ctx context.Context, model, systemPrompt, prompt string, maxToolCalls int) (*AiResponse, int64, int, string, error) {
	startTime := time.Now()

	tools := make([]anthropic.ToolUnionParam, len(anthropicTools))
	for i, toolParam := range anthropicTools {
		tools[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
	}

	client, err := ai.getAnthropicClient(nil)
	if err != nil {
		return nil, 0, int(time.Since(startTime).Milliseconds()), model, err
	}

	messages := []anthropic.MessageParam{
		{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock(prompt),
			},
		},
	}

	var tokensUsed int64 = 0

	// Track total number of tool calls across iterations
	toolCallCount := 0

	// Loop until there are no more tool calls or maxToolCalls reached
	for {
		message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: int64(10000),
			System: []anthropic.TextBlockParam{
				{
					Type: "text",
					Text: systemPrompt + "\n IMPORTANT: You MUST use the provided tools to retrieve current Kubernetes resource information. Never make assumptions about what resources exist or their current state. Available tools: - get_kubernetes_resource: Fetch a specific resource when you know its exact name - list_kubernetes_resources: List all resources of a type, optionally filtered by namespace",
				},
			},
			Messages:    messages,
			Tools:       tools,
			Temperature: anthropic.Float(0.1),
		})

		if err != nil {
			return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, err
		}

		if message != nil {
			tokensUsed += message.Usage.InputTokens + message.Usage.OutputTokens
		}

		if len(message.Content) == 0 {
			return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("no content returned from AI model")
		}

		// Add the assistant's response to the messages
		// Convert ContentBlockUnion to ContentBlockParamUnion
		assistantContent := make([]anthropic.ContentBlockParamUnion, len(message.Content))
		for i, block := range message.Content {
			switch block.Type {
			case "text":
				assistantContent[i] = anthropic.NewTextBlock(block.Text)
			case "tool_use":
				// Unmarshal the Input from JSON to a map so it's sent as a dictionary
				var input map[string]interface{}
				if err := json.Unmarshal(block.Input, &input); err != nil {
					return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling tool input: %v", err)
				}
				assistantContent[i] = anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    block.ID,
						Name:  block.Name,
						Input: input,
					},
				}
			}
		}
		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleAssistant,
			Content: assistantContent,
		})

		// Check if there are tool calls to process
		hasToolUse := false
		var toolResults []anthropic.ContentBlockParamUnion
		iterationToolUses := 0

		for _, block := range message.Content {
			if block.Type == "tool_use" {
				hasToolUse = true
				iterationToolUses++
				ai.logger.Info("Processing tool call", "tool", block.Name)

				if block.Name == "get_kubernetes_resource" || block.Name == "list_kubernetes_resources" {
					// Extract the arguments from the tool use
					var args map[string]interface{}
					inputBytes, err := json.Marshal(block.Input)
					if err != nil {
						return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error marshaling tool input: %v", err)
					}
					err = json.Unmarshal(inputBytes, &args)
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

					toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, data, false))
				}
			}
		}

		if !hasToolUse {
			ai.logger.Info("No tool calls found, finishing AI processing")

			// Extract text from content blocks
			var responseText string
			for _, block := range message.Content {
				if block.Type == "text" {
					responseText += block.Text
				}
			}
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

		// Increase global tool call count and check limit
		toolCallCount += iterationToolUses
		if maxToolCalls > 0 && toolCallCount >= maxToolCalls {
			ai.logger.Info("Max tool call limit reached, exiting loop", "maxToolCalls", maxToolCalls, "toolCallCount", toolCallCount)

			// Try to finalize using any text presently returned
			var responseText string
			for _, block := range message.Content {
				if block.Type == "text" {
					responseText += block.Text
				}
			}
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

		// Add tool results to messages
		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleUser,
			Content: toolResults,
		})

		// Continue the loop to get the next response with tool results
	}
}
