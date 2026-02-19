package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
)

func (ai *aiManager) processPromptOpenAi(ctx context.Context, model, systemPrompt, prompt string, maxToolCalls int) (*AiResponse, int64, int, string, error) {
	startTime := time.Now()

	client, err := ai.getOpenAIClient(nil)
	if err != nil {
		return nil, 0, int(time.Since(startTime).Milliseconds()), model, err
	}

	allTools := append(kubernetesOpenAiTools, helmOpenAiTools...)
	if ai.mcpManager != nil {
		allTools = append(allTools, ai.mcpManager.GetOpenAITools()...)
	}

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
			openai.SystemMessage(systemPrompt + "\n You have access to the following tool: get_kubernetes_resources. Use it to retrieve Kubernetes resources as needed to answer the user's question accurately."),
		},
		Model: model,
		Tools: allTools,
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

			var data string
			if ai.mcpManager != nil && ai.mcpManager.IsMCPTool(toolCall.Function.Name) {
				mcpResult, err := ai.mcpManager.CallTool(ctx, toolCall.Function.Name, args)
				if err != nil {
					return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("MCP tool call %q failed: %v", toolCall.Function.Name, err)
				}
				data = mcpResult
			} else if tool, ok := toolDefinitions[toolCall.Function.Name]; ok {
				data = tool(args, nil, ai.valkeyClient, ai.logger)
			} else {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("unknown tool called: %s", toolCall.Function.Name)
			}

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

	systemPrompt string,
	model string,
	maxToolCalls int,
) error {
	client, err := ai.getOpenAIClient(nil)
	if err != nil {
		return fmt.Errorf("failed to get OpenAI client: %w", err)
	}

	// Add system message at the beginning
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}

	// Build tools once per session (static + MCP, filtered by role)
	chatTools := append(kubernetesOpenAiTools, helmOpenAiTools...)
	if ai.mcpManager != nil {
		chatTools = append(chatTools, ai.mcpManager.GetOpenAITools()...)
	}
	chatTools = filterOpenAiTools(chatTools, ioChannel)

	// Session-level accumulated token counters
	var sessionInputTokens, sessionOutputTokens int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case userInput, ok := <-ioChannel.Input:
			if !ok {
				// Input channel closed
				return nil
			}

			if ai.isTokenLimitExceeded() {
				ai.logger.Warn("Daily token limit exceeded, rejecting input")
				select {
				case ioChannel.Output <- "\n[Error: Daily AI token limit exceeded, cannot process further tasks. Increase limit or wait 24 hours.]":
				case <-ctx.Done():
					return ctx.Err()
				}
				select {
				case ioChannel.Output <- "[COMPLETED]":
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}

			// Add user message to conversation history
			messages = append(messages, openai.UserMessage(userInput))

			// Process with tool call loop
			fullResponse, updatedMessages, err := ai.openaiChatWithTools(ctx, client, model, messages, ioChannel, chatTools, maxToolCalls, &sessionInputTokens, &sessionOutputTokens)
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

			select {
			case ioChannel.Output <- "[COMPLETED]":
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// openaiChatWithTools handles the AI request with streaming and tool call support.
func (ai *aiManager) openaiChatWithTools(
	ctx context.Context,
	client *openai.Client,
	model string,
	messages []openai.ChatCompletionMessageParamUnion,
	ioChannel IOChatChannel,
	chatTools []openai.ChatCompletionToolUnionParam,
	maxToolCalls int,
	sessionInputTokens *int64,
	sessionOutputTokens *int64,
) (fullResponse string, updatedMessages []openai.ChatCompletionMessageParamUnion, err error) {
	toolCallCount := 0
	toolCtx := newToolContextFromIOChannel(ioChannel)

	var inputTokens int64
	var outputTokenCount int64
	var inputTokensUsed int64
	var outputTokensUsed int64
	startTime := time.Now()

	for {
		// Notify user that AI is thinking
		select {
		case ioChannel.Output <- "[AI is thinking...]\n":
		case <-ctx.Done():
			return "", messages, ctx.Err()
		}

		params := openai.ChatCompletionNewParams{
			Messages: messages,
			Model:    model,
			Tools:    chatTools,
			StreamOptions: openai.ChatCompletionStreamOptionsParam{
				IncludeUsage: openai.Bool(true),
			},
		}

		stream := client.Chat.Completions.NewStreaming(ctx, params)

		var fullText strings.Builder
		// Track tool calls being assembled from deltas
		toolCallMap := make(map[int64]*struct {
			ID        string
			Name      string
			Arguments strings.Builder
		})

		for stream.Next() {
			chunk := stream.Current()

			// Capture usage from final chunk
			if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
				inputTokens = chunk.Usage.PromptTokens
				outputTokenCount = chunk.Usage.CompletionTokens
				*sessionInputTokens += inputTokens
				*sessionOutputTokens += outputTokenCount
				inputTokensUsed += inputTokens
				outputTokensUsed += outputTokenCount
				ai.sendTokens(inputTokens, outputTokenCount, sessionInputTokens, sessionOutputTokens, ctx, ioChannel)
			}

			if len(chunk.Choices) == 0 {
				continue
			}
			delta := chunk.Choices[0].Delta

			// Stream text content to the user
			if delta.Content != "" {
				fullText.WriteString(delta.Content)
				select {
				case ioChannel.Output <- delta.Content:
				case <-ctx.Done():
					return "", messages, ctx.Err()
				}
			}

			// Accumulate tool call deltas
			for _, tc := range delta.ToolCalls {
				entry, ok := toolCallMap[tc.Index]
				if !ok {
					entry = &struct {
						ID        string
						Name      string
						Arguments strings.Builder
					}{}
					toolCallMap[tc.Index] = entry
				}
				if tc.ID != "" {
					entry.ID = tc.ID
				}
				if tc.Function.Name != "" {
					entry.Name = tc.Function.Name
					// Notify the user that a tool is being called
					select {
					case ioChannel.Output <- fmt.Sprintf("\n[Using tool: %s]\n", tc.Function.Name):
					case <-ctx.Done():
						return "", messages, ctx.Err()
					}
				}
				if tc.Function.Arguments != "" {
					entry.Arguments.WriteString(tc.Function.Arguments)
				}
			}
		}

		if err := stream.Err(); err != nil {
			return "", messages, fmt.Errorf("streaming error: %w", err)
		}

		// Record token usage for this streaming iteration
		chatKey := "chat"
		if ioChannel.User != nil && ioChannel.User.Email != "" {
			chatKey = fmt.Sprintf("chat:%s", ioChannel.User.Email)
		}
		timeUsedInMs := int(time.Since(startTime).Milliseconds())
		if addErr := ai.addTokenUsage(int(inputTokensUsed+outputTokensUsed), model, timeUsedInMs, chatKey); addErr != nil {
			ai.logger.Error("Error recording chat token usage", "error", addErr)
		}

		ioChannel.Output <- "\n\n"

		// Build the assistant message for conversation history
		if len(toolCallMap) == 0 {
			// No tool calls — just a text response
			response := fullText.String()
			messages = append(messages, openai.AssistantMessage(response))
			return response, messages, nil
		}

		// Build tool_calls slice for the assistant message
		type collectedToolCall struct {
			ID        string
			Name      string
			Arguments string
		}
		var toolCalls []collectedToolCall
		for i := int64(0); ; i++ {
			entry, ok := toolCallMap[i]
			if !ok {
				break
			}
			toolCalls = append(toolCalls, collectedToolCall{
				ID:        entry.ID,
				Name:      entry.Name,
				Arguments: entry.Arguments.String(),
			})
		}

		// Build param tool calls for conversation history
		paramToolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(toolCalls))
		for i, tc := range toolCalls {
			paramToolCalls[i] = openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID: tc.ID,
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				},
			}
		}

		// Add assistant message with tool calls to history
		messages = append(messages, openai.ChatCompletionMessageParamUnion{
			OfAssistant: &openai.ChatCompletionAssistantMessageParam{
				Content: openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(fullText.String()),
				},
				ToolCalls: paramToolCalls,
			},
		})

		// Check tool call limit
		toolCallCount += len(toolCalls)
		if maxToolCalls > 0 && toolCallCount >= maxToolCalls {
			ai.logger.Warn("Max tool calls reached", "count", toolCallCount)
			return fullText.String(), messages, nil
		}

		// Execute each tool call
		for _, tc := range toolCalls {
			ai.logger.Info("Executing tool", "tool", tc.Name)

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
				ai.logger.Error("Error parsing tool arguments", "error", err)
				messages = append(messages, openai.ToolMessage(fmt.Sprintf("Error parsing arguments: %v", err), tc.ID))
				continue
			}

			var result string
			if ai.mcpManager != nil && ai.mcpManager.IsMCPTool(tc.Name) {
				mcpResult, err := ai.mcpManager.CallTool(ctx, tc.Name, args)
				if err != nil {
					ai.logger.Error("MCP tool call failed", "tool", tc.Name, "error", err)
					messages = append(messages, openai.ToolMessage(fmt.Sprintf("Error calling MCP tool: %v", err), tc.ID))
					continue
				}
				result = mcpResult
			} else if tool, ok := toolDefinitions[tc.Name]; ok {
				result = tool(args, toolCtx, ai.valkeyClient, ai.logger)
			} else {
				ai.logger.Error("Unknown tool called", "tool", tc.Name)
				messages = append(messages, openai.ToolMessage(fmt.Sprintf("Unknown tool: %s", tc.Name), tc.ID))
				continue
			}
			messages = append(messages, openai.ToolMessage(result, tc.ID))
		}

		// Continue loop to get response after tool results
	}
}
