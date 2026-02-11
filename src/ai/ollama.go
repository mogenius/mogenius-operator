package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
)

func (ai *aiManager) processPromptOllama(ctx context.Context, model, systemPrompt, prompt string, maxToolCalls int) (*AiResponse, int64, int, string, error) {

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
			Tools:    append(kubernetesOllamaTools, helmOllamaTools...),
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

func (ai *aiManager) ollamaChat(
	ctx context.Context,
	ioChannel IOChatChannel,
	systemPrompt string,
	model string,
	maxToolCalls int,
) error {

	client, err := ai.getOllamaClient(nil)
	if err != nil {
		return fmt.Errorf("failed to get Ollama client: %w", err)
	}

	// Start with system message
	messages := []api.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// Session-level accumulated token counters
	var sessionInputTokens, sessionOutputTokens int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case userInput, ok := <-ioChannel.Input:
			if !ok {
				return nil
			}

			messages = append(messages, api.Message{
				Role:    "user",
				Content: userInput,
			})

			fullResponse, updatedMessages, err := ai.ollamaChatWithTools(ctx, client, model, messages, ioChannel, maxToolCalls, &sessionInputTokens, &sessionOutputTokens)
			if err != nil {
				ai.logger.Error("Error processing with tools", "error", err)
				select {
				case ioChannel.Output <- fmt.Sprintf("\n[Error: %v]", err):
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}

			messages = updatedMessages
			messages = append(messages, api.Message{
				Role:    "assistant",
				Content: fullResponse,
			})

			select {
			case ioChannel.Output <- "[COMPLETED]":
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// ollamaChatWithTools handles the AI request with streaming and tool call support.
func (ai *aiManager) ollamaChatWithTools(
	ctx context.Context,
	client *api.Client,
	model string,
	messages []api.Message,
	ioChannel IOChatChannel,
	maxToolCalls int,
	sessionInputTokens *int64,
	sessionOutputTokens *int64,
) (fullResponse string, updatedMessages []api.Message, err error) {
	toolCallCount := 0
	truePtr := true

	var inputTokens int64
	var outputTokenCount int64

	for {
		// Notify user that AI is thinking
		select {
		case ioChannel.Output <- "[AI is thinking...]\n":
		case <-ctx.Done():
			return "", messages, ctx.Err()
		}

		req := &api.ChatRequest{
			Model:    model,
			Messages: messages,
			Stream:   &truePtr,
			Truncate: &truePtr,
			Shift:    &truePtr,
			Tools:    append(kubernetesOllamaTools, helmOllamaTools...),
			Options: map[string]interface{}{
				"temperature": 0.7,
			},
		}

		var fullText strings.Builder
		var toolCalls []api.ToolCall

		err = client.Chat(ctx, req, func(resp api.ChatResponse) error {
			if resp.Message.Content != "" {
				fullText.WriteString(resp.Message.Content)
				select {
				case ioChannel.Output <- resp.Message.Content:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			if resp.Done {
				inputTokens = int64(resp.PromptEvalCount)
				outputTokenCount = int64(resp.EvalCount)
				*sessionInputTokens += inputTokens
				*sessionOutputTokens += outputTokenCount
				ai.logger.Info("Stream usage", "input_tokens", inputTokens, "output_tokens", outputTokenCount,
					"session_input_tokens", *sessionInputTokens, "session_output_tokens", *sessionOutputTokens)

				if len(resp.Message.ToolCalls) > 0 {
					toolCalls = resp.Message.ToolCalls
					for _, tc := range toolCalls {
						select {
						case ioChannel.Output <- fmt.Sprintf("\n[Using tool: %s]\n", tc.Function.Name):
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				}
			}

			return nil
		})

		if err != nil {
			return "", messages, fmt.Errorf("streaming error: %w", err)
		}

		ai.sendTokens(inputTokens, outputTokenCount, sessionInputTokens, sessionOutputTokens, ctx, ioChannel)

		ioChannel.Output <- "\n\n"

		// No tool calls — just a text response
		if len(toolCalls) == 0 {
			response := fullText.String()
			messages = append(messages, api.Message{
				Role:    "assistant",
				Content: response,
			})
			return response, messages, nil
		}

		// Add assistant message with tool calls to history
		messages = append(messages, api.Message{
			Role:      "assistant",
			Content:   fullText.String(),
			ToolCalls: toolCalls,
		})

		// Check tool call limit
		toolCallCount += len(toolCalls)
		if maxToolCalls > 0 && toolCallCount >= maxToolCalls {
			ai.logger.Warn("Max tool calls reached", "count", toolCallCount)
			return fullText.String(), messages, nil
		}

		// Execute each tool call
		for _, tc := range toolCalls {
			ai.logger.Info("Executing tool", "tool", tc.Function.Name)

			var args map[string]any
			argsBytes, err := json.Marshal(tc.Function.Arguments)
			if err != nil {
				ai.logger.Error("Error marshaling tool arguments", "error", err)
				messages = append(messages, api.Message{
					Role:    "tool",
					Content: fmt.Sprintf("Error marshaling arguments: %v", err),
				})
				continue
			}
			if err := json.Unmarshal(argsBytes, &args); err != nil {
				ai.logger.Error("Error parsing tool arguments", "error", err)
				messages = append(messages, api.Message{
					Role:    "tool",
					Content: fmt.Sprintf("Error parsing arguments: %v", err),
				})
				continue
			}

			var result string
			if ai.mcpManager != nil && ai.mcpManager.IsMCPTool(tc.Function.Name) {
				mcpResult, err := ai.mcpManager.CallTool(ctx, tc.Function.Name, args)
				if err != nil {
					ai.logger.Error("MCP tool call failed", "tool", tc.Function.Name, "error", err)
					messages = append(messages, api.Message{
						Role:    "tool",
						Content: fmt.Sprintf("Error calling MCP tool: %v", err),
					})
					continue
				}
				result = mcpResult
			} else if tool, ok := toolDefinitions[tc.Function.Name]; ok {
				result = tool(args, ai.valkeyClient, ai.logger)
			} else {
				ai.logger.Error("Unknown tool called", "tool", tc.Function.Name)
				messages = append(messages, api.Message{
					Role:    "tool",
					Content: fmt.Sprintf("Unknown tool: %s", tc.Function.Name),
				})
				continue
			}
			messages = append(messages, api.Message{
				Role:    "tool",
				Content: result,
			})
		}

		// Continue loop to get response after tool results
	}
}
