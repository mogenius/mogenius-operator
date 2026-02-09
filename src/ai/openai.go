package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go/v3"
)

var openAiTools = []openai.ChatCompletionToolUnionParam{
	{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "get_kubernetes_resources",
				Description: openai.String("Get a specific Kubernetes resource by kind, name and namespace"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"apiVersion": map[string]string{
							"type":        "string",
							"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
						},
						"kind": map[string]string{
							"type":        "string",
							"description": "Kind of the resource (e.g. 'Pod', 'Deployment', 'Service')",
						},
						"name": map[string]string{
							"type":        "string",
							"description": "Name of the specific resource",
						},
						"namespace": map[string]string{
							"type":        "string",
							"description": "Namespace of the resource (optional for cluster-scoped resources)",
						},
					},
					"required": []string{"kind", "apiVersion", "name"},
				},
			},
		},
	},
	{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "list_kubernetes_resources",
				Description: openai.String("List all Kubernetes resources of a specific kind, optionally filtered by namespace"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"apiVersion": map[string]string{
							"type":        "string",
							"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
						},
						"kind": map[string]string{
							"type":        "string",
							"description": "Kind of the resource (e.g. 'Pod', 'Deployment', 'Service')",
						},
						"namespace": map[string]string{
							"type":        "string",
							"description": "Namespace to filter by (optional, leave empty for all namespaces)",
						},
					},
					"required": []string{"kind", "apiVersion"},
				},
			},
		},
	},
	{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "update_kubernetes_resource",
				Description: openai.String("Update an existing Kubernetes resource with new YAML configuration"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"apiVersion": map[string]string{
							"type":        "string",
							"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
						},
						"plural": map[string]string{
							"type":        "string",
							"description": "Plural name of the resource (e.g. 'pods', 'deployments', 'services')",
						},
						"namespaced": map[string]string{
							"type":        "boolean",
							"description": "Whether the resource is namespaced (true) or cluster-scoped (false)",
						},
						"yamlData": map[string]string{
							"type":        "string",
							"description": "Complete YAML definition of the resource to update",
						},
					},
					"required": []string{"apiVersion", "plural", "namespaced", "yamlData"},
				},
			},
		},
	},
	{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "delete_kubernetes_resource",
				Description: openai.String("Delete a Kubernetes resource by name and namespace"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"apiVersion": map[string]string{
							"type":        "string",
							"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
						},
						"plural": map[string]string{
							"type":        "string",
							"description": "Plural name of the resource (e.g. 'pods', 'deployments', 'services')",
						},
						"namespace": map[string]string{
							"type":        "string",
							"description": "Namespace of the resource (empty for cluster-scoped resources)",
						},
						"name": map[string]string{
							"type":        "string",
							"description": "Name of the resource to delete",
						},
					},
					"required": []string{"apiVersion", "plural", "name"},
				},
			},
		},
	},
	{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        "create_kubernetes_resource",
				Description: openai.String("Create a new Kubernetes resource from YAML configuration"),
				Parameters: openai.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"apiVersion": map[string]string{
							"type":        "string",
							"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
						},
						"plural": map[string]string{
							"type":        "string",
							"description": "Plural name of the resource (e.g. 'pods', 'deployments', 'services')",
						},
						"namespaced": map[string]string{
							"type":        "boolean",
							"description": "Whether the resource is namespaced (true) or cluster-scoped (false)",
						},
						"yamlData": map[string]string{
							"type":        "string",
							"description": "Complete YAML definition of the resource to create",
						},
					},
					"required": []string{"apiVersion", "plural", "namespaced", "yamlData"},
				},
			},
		},
	},
}

func (ai *aiManager) processPromptOpenAi(ctx context.Context, model, systemPrompt, prompt string, maxToolCalls int) (*AiResponse, int64, int, string, error) {
	startTime := time.Now()

	client, err := ai.getOpenAIClient(nil)
	if err != nil {
		return nil, 0, int(time.Since(startTime).Milliseconds()), model, err
	}

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
			openai.SystemMessage(systemPrompt + "\n You have access to the following tool: get_kubernetes_resources. Use it to retrieve Kubernetes resources as needed to answer the user's question accurately."),
		},
		Model:       model,
		Tools:       openAiTools,
		Temperature: openai.Float(0.1),
	}

	var tokensUsed int64 = 0
	var chatCompletion *openai.ChatCompletion
	toolCallCount := 0
	for {
		chatCompletion, err = client.Chat.Completions.New(ctx, params)
		if err != nil {
			return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, err
		}

		if chatCompletion != nil {
			tokensUsed += chatCompletion.Usage.TotalTokens
		}

		if len(chatCompletion.Choices) == 0 {
			return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("no choices returned from AI model")
		}

		// Add the assistant's response to the messages
		params.Messages = append(params.Messages, chatCompletion.Choices[0].Message.ToParam())

		// Check if there are tool calls to process
		if len(chatCompletion.Choices[0].Message.ToolCalls) == 0 {
			ai.logger.Info("No tool calls found, finishing AI processing")
			break
		}

		// Process each tool call
		for _, toolCall := range chatCompletion.Choices[0].Message.ToolCalls {
			ai.logger.Info("Processing tool call", "tool", toolCall.Function.Name)
			// Extract the location from the function call arguments
			var args map[string]any
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling tool arguments: %v", err)
			}

			tool, ok := toolDefinitions[toolCall.Function.Name]
			if !ok {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("unknown tool called: %s", toolCall.Function.Name)
			}

			data := tool(args, ai.valkeyClient, ai.logger)

			params.Messages = append(params.Messages, openai.ToolMessage(data, toolCall.ID))
		}

		// Increase global tool call count and check limit
		toolCallCount += len(chatCompletion.Choices[0].Message.ToolCalls)
		if maxToolCalls > 0 && toolCallCount >= maxToolCalls {
			ai.logger.Info("Max tool call limit reached, exiting loop", "maxToolCalls", maxToolCalls, "toolCallCount", toolCallCount)

			// Try to finalize using any text presently returned
			responseText := cleanJSONResponse(chatCompletion.Choices[0].Message.Content)
			responseBytes, removedText, err := extractJSONRobust(responseText)
			ai.logger.Info("Extracted JSON after max tool calls", "removed_text", removedText)
			if err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("max tool calls reached (%d) without final text: %v", maxToolCalls, err)
			}

			var aiResponse AiResponse
			if err := json.Unmarshal(responseBytes, &aiResponse); err != nil {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling AI response after max tool calls: %v\n%s", err, chatCompletion.Choices[0].Message.Content)
			}

			return &aiResponse, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
		}
		// Continue the loop to get the next response with tool results
	}

	responseText := cleanJSONResponse(chatCompletion.Choices[0].Message.Content)
	responseBytes, removedText, err := extractJSONRobust(responseText)
	ai.logger.Info("Extracted JSON from AI response", "removed_text", removedText)
	if err != nil {
		return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error extracting JSON from AI response: %v\n%s", err, responseText)
	}

	var aiResponse AiResponse
	err = json.Unmarshal(responseBytes, &aiResponse)
	if err != nil {
		return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling AI response: %v\n%s", err, chatCompletion.Choices[0].Message.Content)
	}

	// also return tokens used
	return &aiResponse, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
}

func (ai *aiManager) openaiChat(
	ctx context.Context,
	ioChannel IOChatChannel,

	model string,
	messages []openai.ChatCompletionMessageParamUnion,
	maxToolCalls int,
) error {
	client, err := ai.getOpenAIClient(nil)
	if err != nil {
		return fmt.Errorf("failed to get OpenAI client: %w", err)
	}

	// Build system prompt with user info
	systemPrompt := "You are a helpful Kubernetes assistant. You can help users manage and understand their Kubernetes resources." + mogeniusCRDsPrompt
	if ioChannel.User != nil {
		userInfo := ""
		if ioChannel.User.FirstName != "" {
			userInfo = ioChannel.User.FirstName
			if ioChannel.User.LastName != "" {
				userInfo += " " + ioChannel.User.LastName
			}
		}
		if userInfo != "" {
			systemPrompt += fmt.Sprintf("\n\nYou are chatting with %s.", userInfo)
		}
		if ioChannel.User.Email != "" {
			systemPrompt += fmt.Sprintf(" Their email is %s.", ioChannel.User.Email)
		}
	}

	// Add system message at the beginning
	messages = append([]openai.ChatCompletionMessageParamUnion{openai.SystemMessage(systemPrompt)}, messages...)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case userInput, ok := <-ioChannel.Input:
			if !ok {
				// Input channel closed
				return nil
			}

			// Add user message to conversation history
			messages = append(messages, openai.UserMessage(userInput))

			// Process with tool call loop
			fullResponse, updatedMessages, err := ai.openaiChatWithTools(ctx, client, model, messages, ioChannel, maxToolCalls)
			if err != nil {
				ai.logger.Error("Error processing with tools", "error", err)
				// Send error to output
				select {
				case ioChannel.Output <- fmt.Sprintf("\n[Error: %v]", err):
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}

			// Update messages with full conversation including tool calls
			messages = updatedMessages
			// Add assistant response to conversation history
			messages = append(messages, openai.AssistantMessage(fullResponse))
		}
	}
}

// processChatWithTools handles the AI request with potential tool calls (non-streaming for tool support)
func (ai *aiManager) openaiChatWithTools(
	ctx context.Context,
	client *openai.Client,
	model string,
	messages []openai.ChatCompletionMessageParamUnion, ioChannel IOChatChannel,
	maxToolCalls int,
) (string, []openai.ChatCompletionMessageParamUnion, error) {
	toolCallCount := 0

	for {
		params := openai.ChatCompletionNewParams{
			Messages:    messages,
			Model:       model,
			Tools:       openAiTools,
			Temperature: openai.Float(0.7),
		}

		// Use non-streaming API for tool call support
		chatCompletion, err := client.Chat.Completions.New(ctx, params)
		if err != nil {
			return "", messages, fmt.Errorf("chat completion error: %w", err)
		}

		if len(chatCompletion.Choices) == 0 {
			return "", messages, fmt.Errorf("no choices returned from AI model")
		}

		choice := chatCompletion.Choices[0]

		// Add assistant message to history
		messages = append(messages, choice.Message.ToParam())

		// If no tool calls, send response and we're done
		if len(choice.Message.ToolCalls) == 0 {
			// Send response to output channel
			response := choice.Message.Content
			select {
			case ioChannel.Output <- response:
			case <-ctx.Done():
				return "", messages, ctx.Err()
			}
			return response, messages, nil
		}

		// Check tool call limit
		toolCallCount += len(choice.Message.ToolCalls)
		if toolCallCount >= maxToolCalls {
			ai.logger.Warn("Max tool calls reached", "count", toolCallCount)
			return choice.Message.Content, messages, nil
		}

		// Process each tool call
		for _, toolCall := range choice.Message.ToolCalls {
			ai.logger.Info("Processing tool call", "tool", toolCall.Function.Name)

			// Notify user that tool is being used
			select {
			case ioChannel.Output <- fmt.Sprintf("[Using tool: %s]\n", toolCall.Function.Name):
			case <-ctx.Done():
				return "", messages, ctx.Err()
			}

			// Parse arguments
			var args map[string]any
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				ai.logger.Error("Error parsing tool arguments", "error", err)
				messages = append(messages, openai.ToolMessage(fmt.Sprintf("Error parsing arguments: %v", err), toolCall.ID))
				continue
			}

			// Execute tool
			tool, ok := toolDefinitions[toolCall.Function.Name]
			if !ok {
				ai.logger.Error("Unknown tool called", "tool", toolCall.Function.Name)
				messages = append(messages, openai.ToolMessage(fmt.Sprintf("Unknown tool: %s", toolCall.Function.Name), toolCall.ID))
				continue
			}

			result := tool(args, ai.valkeyClient, ai.logger)
			messages = append(messages, openai.ToolMessage(result, toolCall.ID))
		}

		// Continue loop to get response after tool calls
	}
}
