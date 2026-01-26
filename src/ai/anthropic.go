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
		Name:        "get_kubernetes_resources",
		Description: anthropic.String("Get Kubernetes resources by kind, optionally filtered by name and namespace"),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type: "object",
			Properties: map[string]interface{}{
				"kind": map[string]interface{}{
					"type":        "string",
					"description": "The kind of the Kubernetes resource.",
				},
				"apiVersion": map[string]interface{}{
					"type":        "string",
					"description": "The API version of the resource.",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the resource (optional).",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "The namespace of the resource (optional).",
				},
			},
			Required: []string{"kind", "apiVersion"},
		},
	},
}

func (ai *aiManager) processPromptAnthropic(ctx context.Context, model, systemPrompt, prompt string) (*AiResponse, int64, int, string, error) {
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

	// Loop until there are no more tool calls
	for {
		message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: int64(10000),
			System: []anthropic.TextBlockParam{
				{
					Type: "text",
					Text: systemPrompt + "\n You have access to the following tool: get_kubernetes_resources. Use it to retrieve Kubernetes resources as needed to answer the user's question accurately.",
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

		for _, block := range message.Content {
			if block.Type == "tool_use" {
				hasToolUse = true
				ai.logger.Info("Processing tool call", "tool", block.Name)

				if block.Name == "get_kubernetes_resources" {
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

			var aiResponse AiResponse
			err = json.Unmarshal([]byte(responseText), &aiResponse)
			if err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling AI response: %v\n%s", err, responseText)
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
