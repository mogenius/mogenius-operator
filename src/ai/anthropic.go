package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

func (ai *aiManager) anthropicChat(
	ctx context.Context,
	ioChannel IOChatChannel,

	systemPrompt string,
	model string,
	maxToolCalls int,
) error {
	client, err := ai.getAnthropicClient(nil)
	if err != nil {
		return fmt.Errorf("failed to get Anthropic client: %w", err)
	}

	// Maintain conversation history
	messages := []anthropic.MessageParam{}

	// Build full tool set once per session (static + MCP, filtered by role)
	allAnthropicTools := append(kubernetesAnthropicTools, helmAnthropicTools...)
	if !isViewerRole(ioChannel) {
		allAnthropicTools = append(allAnthropicTools, activateToolCategoriesAnthropic)
	}
	if ai.mcpManager != nil {
		allAnthropicTools = append(allAnthropicTools, ai.mcpManager.GetAnthropicTools()...)
	}
	allAnthropicTools = filterAnthropicTools(allAnthropicTools, ioChannel)

	// Session-level category filter (sticky, driven by LLM via meta-tool)
	categories := NewActiveToolCategories()

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
				case ioChannel.Output <- fmt.Sprintf("\n[Error: %s]", ai.tokenLimitErrorMessage()):
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
			messages = append(messages, anthropic.MessageParam{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewTextBlock(userInput),
				},
			})

			// Process with tool call loop (categories + allTools passed so
			// the inner loop can recompute active tools after activation)
			_, updatedMessages, err := ai.anthropicChatWithTools(ctx, client, systemPrompt, model, messages, ioChannel, allAnthropicTools, categories, maxToolCalls, &sessionInputTokens, &sessionOutputTokens)
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

			// Discard intermediate tool_use/tool_result exchanges from history.
			// updatedMessages = [..., user_input, tool_exchanges..., assistant_final]
			// messages already contains user_input, so we only append the final
			// assistant text response. This prevents tool results (often large
			// JSON blobs) from accumulating in the context on every turn.
			messages = append(messages, updatedMessages[len(updatedMessages)-1])

			select {
			case ioChannel.Output <- "[COMPLETED]":
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// anthropicChatWithTools handles the AI request with potential tool calls and streaming
func (ai *aiManager) anthropicChatWithTools(
	ctx context.Context,
	client *anthropic.Client,
	systemPrompt string,
	model string,
	messages []anthropic.MessageParam,
	ioChannel IOChatChannel,
	allAnthropicTools []anthropic.ToolParam,
	categories *ActiveToolCategories,
	maxToolCalls int,
	sessionInputTokens *int64,
	sessionOutputTokens *int64,
) (fullResponse string, updatedMessages []anthropic.MessageParam, err error) {
	toolCallCount := 0
	toolCtx := newToolContextFromIOChannel(ioChannel)

	var inputTokens int64
	var outputTokenCount int64
	inputTokensUsed := int64(0)
	outputTokensUsed := int64(0)
	startTime := time.Now()

	for {
		// Recompute active tools each iteration (categories may have changed)
		activeTools := filterAnthropicToolsByCategory(allAnthropicTools, categories)
		tools := make([]anthropic.ToolUnionParam, len(activeTools))
		for i, toolParam := range activeTools {
			// Mark the last tool with cache_control so Anthropic caches the
			// entire tool block server-side (cached tokens cost ~10% of normal).
			if i == len(activeTools)-1 {
				toolParam.CacheControl = anthropic.NewCacheControlEphemeralParam()
			}
			tools[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
		}

		// Notify user that AI is thinking
		select {
		case ioChannel.Output <- "[AI is thinking...]\n":
		case <-ctx.Done():
			return "", messages, ctx.Err()
		}

		stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: int64(4096),
			System: []anthropic.TextBlockParam{
				{Type: "text", Text: systemPrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()},
			},
			Messages:    messages,
			Tools:       tools,
			Temperature: anthropic.Float(0.7),
		})

		// Accumulator for the full message
		var accumulatedMessage anthropic.Message
		var fullText strings.Builder
		var toolUseBlocks []struct {
			ID    string
			Name  string
			Input json.RawMessage
		}
		inputTokens = 0
		outputTokenCount = 0

		// Process streaming events
		for stream.Next() {
			event := stream.Current()

			// Accumulate into the message. Partial tool_use blocks may
			// produce transient marshal errors until all deltas arrive,
			// so we only log at debug level here.
			if err := accumulatedMessage.Accumulate(event); err != nil {
				ai.logger.Debug("Transient accumulate error (expected during tool_use streaming)", "error", err)
			}

			// Handle different event types for real-time streaming
			switch evt := event.AsAny().(type) {
			case anthropic.ContentBlockStartEvent:
				if evt.ContentBlock.Type == "tool_use" {
					ai.logger.Info("Tool use starting", "tool", evt.ContentBlock.Name)
					select {
					case ioChannel.Output <- fmt.Sprintf("\n[Using tool: %s]\n", evt.ContentBlock.Name):
					case <-ctx.Done():
						return "", messages, ctx.Err()
					}
				}

			case anthropic.ContentBlockDeltaEvent:
				// Check if it's a text delta
				if evt.Delta.Type == "text_delta" {
					text := evt.Delta.Text
					fullText.WriteString(text)
					select {
					case ioChannel.Output <- text:
					case <-ctx.Done():
						return "", messages, ctx.Err()
					}
				}

			case anthropic.MessageStartEvent:
				inputTokens = evt.Message.Usage.InputTokens
				*sessionInputTokens += inputTokens
				inputTokensUsed += inputTokens
				ai.sendTokens(inputTokens, outputTokenCount, sessionInputTokens, sessionOutputTokens, ctx, ioChannel)

			case anthropic.MessageDeltaEvent:
				outputTokenCount = evt.Usage.OutputTokens
				*sessionOutputTokens += outputTokenCount
				outputTokensUsed += outputTokenCount
				ai.sendTokens(inputTokens, outputTokenCount, sessionInputTokens, sessionOutputTokens, ctx, ioChannel)

			case anthropic.MessageStopEvent:
				// Record token usage for this streaming iteration
				chatKey := "chat"
				if ioChannel.User != nil && ioChannel.User.Email != "" {
					chatKey = fmt.Sprintf("chat:%s", ioChannel.User.Email)
				}
				timeUsedInMs := int(time.Since(startTime).Milliseconds())
				if addErr := ai.addTokenUsage(int(inputTokensUsed+outputTokensUsed), model, timeUsedInMs, chatKey); addErr != nil {
					ai.logger.Error("Error recording chat token usage", "error", addErr)
				}
				inputTokensUsed = 0
				outputTokensUsed = 0
				ai.sendTokens(inputTokens, outputTokenCount, sessionInputTokens, sessionOutputTokens, ctx, ioChannel)
			}
		}

		// Check for streaming errors
		if err := stream.Err(); err != nil {
			return "", messages, fmt.Errorf("streaming error: %w", err)
		}

		select {
		case ioChannel.Output <- "\n\n":
		case <-ctx.Done():
			return "", messages, ctx.Err()
		}

		// Use the accumulated message
		finalMessage := accumulatedMessage

		// Add assistant message to history
		assistantContent := make([]anthropic.ContentBlockParamUnion, len(finalMessage.Content))
		for i, block := range finalMessage.Content {
			switch block.Type {
			case "text":
				assistantContent[i] = anthropic.NewTextBlock(block.Text)
			case "tool_use":
				var input map[string]interface{}
				if err := json.Unmarshal(block.Input, &input); err != nil {
					return "", messages, fmt.Errorf("error unmarshaling tool input: %w", err)
				}
				assistantContent[i] = anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    block.ID,
						Name:  block.Name,
						Input: input,
					},
				}
				// Collect tool use for execution
				toolUseBlocks = append(toolUseBlocks, struct {
					ID    string
					Name  string
					Input json.RawMessage
				}{
					ID:    block.ID,
					Name:  block.Name,
					Input: block.Input,
				})
			}
		}
		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleAssistant,
			Content: assistantContent,
		})

		// If no tool calls, we're done
		if len(toolUseBlocks) == 0 {
			return fullText.String(), messages, nil
		}

		// Check tool call limit
		toolCallCount += len(toolUseBlocks)
		if maxToolCalls > 0 && toolCallCount >= maxToolCalls {
			ai.logger.Warn("Max tool calls reached", "count", toolCallCount)
			// Replace the just-appended assistant tool_use message with a
			// text-only one. Without this, messages[-1] would contain
			// tool_use blocks with no corresponding tool_result, causing a
			// 400 on the next API request.
			text := fullText.String()
			if text == "" {
				text = "[Tool call limit reached]"
			}
			messages = messages[:len(messages)-1]
			messages = append(messages, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleAssistant,
				Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(text)},
			})
			return text, messages, nil
		}

		// Execute tool calls and collect results
		var toolResults []anthropic.ContentBlockParamUnion
		for _, toolUse := range toolUseBlocks {
			ai.logger.Info("Executing tool", "tool", toolUse.Name)

			// Parse arguments
			var args map[string]any
			if err := json.Unmarshal(toolUse.Input, &args); err != nil {
				ai.logger.Error("Error parsing tool arguments", "error", err)
				toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("Error parsing arguments: %v", err), true))
				continue
			}

			// Execute tool
			var result string
			if toolUse.Name == activateToolCategoriesName {
				result = categories.ActivateFromToolCall(args)
			} else if ai.mcpManager != nil && ai.mcpManager.IsMCPTool(toolUse.Name) {
				mcpResult, err := ai.mcpManager.CallTool(ctx, toolUse.Name, args)
				if err != nil {
					ai.logger.Error("MCP tool call failed", "tool", toolUse.Name, "error", err)
					toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("Error calling MCP tool: %v", err), true))
					continue
				}
				result = mcpResult
			} else if tool, ok := toolDefinitions[toolUse.Name]; ok {
				result = tool(args, toolCtx, ai.valkeyClient, ai.logger)
			} else {
				ai.logger.Error("Unknown tool called", "tool", toolUse.Name)
				toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("Unknown tool: %s", toolUse.Name), true))
				continue
			}
			toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, result, false))
		}

		// Add tool results to messages for next iteration
		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleUser,
			Content: toolResults,
		})

		// Continue loop to get response after tool calls
	}
}

func (ai *aiManager) processPromptAnthropic(ctx context.Context, model, systemPrompt, prompt string, maxToolCalls int) (*AiResponse, int64, int, string, error) {
	startTime := time.Now()

	allTools := append(kubernetesAnthropicTools, helmAnthropicTools...)
	if ai.mcpManager != nil {
		allTools = append(allTools, ai.mcpManager.GetAnthropicTools()...)
	}
	tools := make([]anthropic.ToolUnionParam, len(allTools))
	for i, toolParam := range allTools {
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

				// Extract the arguments from the tool use
				var args map[string]any
				inputBytes, err := json.Marshal(block.Input)
				if err != nil {
					return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error marshaling tool input: %v", err)
				}
				err = json.Unmarshal(inputBytes, &args)
				if err != nil {
					return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling tool arguments: %v", err)
				}

				var data string
				if ai.mcpManager != nil && ai.mcpManager.IsMCPTool(block.Name) {
					mcpResult, err := ai.mcpManager.CallTool(ctx, block.Name, args)
					if err != nil {
						return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("MCP tool call %q failed: %v", block.Name, err)
					}
					data = mcpResult
				} else if tool, ok := toolDefinitions[block.Name]; ok {
					data = tool(args, nil, ai.valkeyClient, ai.logger)
				} else {
					return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("unknown tool called: %s", block.Name)
				}

				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, data, false))
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
