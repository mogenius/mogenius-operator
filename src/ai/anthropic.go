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
			fullResponse, updatedMessages, turnStats, err := ai.anthropicChatWithTools(ctx, client, systemPrompt, model, messages, ioChannel, allAnthropicTools, categories, maxToolCalls, &sessionInputTokens, &sessionOutputTokens)
			if err != nil {
				ai.logger.Error("Error processing with tools", "error", err)
				payload := map[string]any{"question": userInput, "stats": turnStats}
				emitAuditEvent(ioChannel, "ai/chat", payload, nil, err.Error())
				select {
				case ioChannel.Output <- fmt.Sprintf("\n[Error: %v]", err):
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}

			payload := map[string]any{"question": userInput, "response": truncateToolResult(fullResponse), "stats": turnStats}
			emitAuditEvent(ioChannel, "ai/chat", payload, nil, "")

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
) (fullResponse string, updatedMessages []anthropic.MessageParam, stats ChatTurnStats, err error) {
	toolCallCount := 0
	toolCtx := newToolContextFromIOChannel(ioChannel)
	stats.Model = model
	turnStartInput := *sessionInputTokens
	turnStartOutput := *sessionOutputTokens
	startTime := time.Now()
	defer func() {
		stats.InputTokens = *sessionInputTokens - turnStartInput
		stats.OutputTokens = *sessionOutputTokens - turnStartOutput
		stats.DurationMs = int(time.Since(startTime).Milliseconds())
	}()

	var inputTokens int64
	var outputTokenCount int64
	inputTokensUsed := int64(0)
	outputTokensUsed := int64(0)

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
			return "", messages, stats, ctx.Err()
		}

		stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: int64(4096),
			System: []anthropic.TextBlockParam{
				{Type: "text", Text: systemPrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()},
			},
			Messages: messages,
			Tools:    tools,
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
						return "", messages, stats, ctx.Err()
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
						return "", messages, stats, ctx.Err()
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
			return "", messages, stats, fmt.Errorf("streaming error: %w", err)
		}

		select {
		case ioChannel.Output <- "\n\n":
		case <-ctx.Done():
			return "", messages, stats, ctx.Err()
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
				var input map[string]any
				if err := json.Unmarshal(block.Input, &input); err != nil {
					return "", messages, stats, fmt.Errorf("error unmarshaling tool input: %w", err)
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
			return fullText.String(), messages, stats, nil
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
			return text, messages, stats, nil
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
			var toolErr string
			if toolUse.Name == activateToolCategoriesName {
				result = categories.ActivateFromToolCall(args)
			} else if ai.mcpManager != nil && ai.mcpManager.IsMCPTool(toolUse.Name) {
				mcpResult, err := ai.mcpManager.CallTool(ctx, toolUse.Name, args)
				if err != nil {
					ai.logger.Error("MCP tool call failed", "tool", toolUse.Name, "error", err)
					toolErr = fmt.Sprintf("Error calling MCP tool: %v", err)
					toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, toolErr, true))
					stats.ToolRecords = append(stats.ToolRecords, ToolUseRecord{Tool: toolUse.Name, Args: args, Error: toolErr})
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
			stats.ToolRecords = append(stats.ToolRecords, ToolUseRecord{Tool: toolUse.Name, Args: args, Result: truncateToolResult(result)})
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

func (ai *aiManager) processPromptAnthropic(ctx context.Context, model, systemPrompt, prompt string, maxToolCalls int, maxTokensPerRun int64, toolCtx *ToolContext, onProgress func(int64, string)) ([]*AiResponse, int64, int, string, error) {
	startTime := time.Now()

	// Unattended pipeline: strictly read-only tools, no external MCP tools.
	// The ToolContext additionally scopes reads to the agent's namespaces —
	// the read-only filter stays as defense in depth.
	// The final analysis is collected through the schema-carrying
	// submit_analysis tool (appended last so the cache boundary covers it)
	// instead of being scraped out of free text.
	allTools := readOnlyAnthropicTools(append(kubernetesAnthropicTools, helmAnthropicTools...))
	allTools = append(allTools, submitAnalysisAnthropicTool)
	tools := make([]anthropic.ToolUnionParam, len(allTools))
	for i, toolParam := range allTools {
		if i == len(allTools)-1 {
			toolParam.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
		tools[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
	}
	systemPrompt += submitAnalysisInstruction

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

	// Bounded in-conversation repair turns for schema-violating final answers.
	repairAttempts := 0

	// Findings accumulated across repeated submit_analysis calls. The tool is
	// repeatable so the number of findings is not limited by a single
	// response's output budget.
	collected := []*AiResponse{}

	// Loop until there are no more tool calls or maxToolCalls reached
	for {
		message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: int64(10000),
			System: []anthropic.TextBlockParam{
				{Type: "text", Text: systemPrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()},
			},
			Messages: messages,
			Tools:    tools,
		})

		if err != nil {
			if len(collected) > 0 {
				// Salvage what the run already confirmed instead of throwing
				// the whole exploration away.
				ai.logger.Warn("LLM turn failed mid-run, keeping findings collected so far", "collected", len(collected), "error", err)
				return collected, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
			}
			return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, err
		}

		// Everything in messages has now been seen by the model — compact the
		// tool results it just processed so they stop burning tokens on the
		// following turns. Compacting BEFORE the call would blind the model:
		// results are appended at the end of an iteration and must survive
		// exactly one request (the regression that shipped in 7782b65b).
		compactAnthropicToolResults(messages)

		if message != nil {
			tokensUsed += message.Usage.InputTokens + message.Usage.OutputTokens
		}
		if onProgress != nil {
			onProgress(tokensUsed, "")
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
				var input map[string]any
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

				// The final analysis arrives as tool input; a schema violation
				// is fed back as an is_error tool result so the model repairs
				// it in-conversation instead of failing the whole run.
				if block.Name == submitAnalysisToolName {
					findings, parseErr := parseSubmittedAnalysis(block.Input)
					if parseErr != nil {
						repairAttempts++
						ai.logger.Warn("Submitted findings rejected", "error", parseErr, "attempt", repairAttempts)
						if repairAttempts > maxAnalysisRepairs {
							if len(collected) > 0 {
								return collected, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
							}
							return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("analysis rejected %d times, giving up: %w", repairAttempts, parseErr)
						}
						toolResults = append(toolResults, analysisRejectionResult(block.ID, parseErr))
						continue
					}
					if len(findings) == 0 {
						// Empty submission: the model declares the run finished.
						return collected, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
					}
					collected = append(collected, findings...)
					ai.logger.Info("Findings submitted", "new", len(findings), "total", len(collected))
					if onProgress != nil {
						onProgress(tokensUsed, fmt.Sprintf("%d finding(s) submitted", len(collected)))
					}
					toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID,
						fmt.Sprintf("Recorded %d finding(s) — %d total so far. Continue the investigation and submit further findings, or call %s with an empty findings array when nothing else is actionable.", len(findings), len(collected), submitAnalysisToolName),
						false))
					continue
				}

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

				if onProgress != nil {
					onProgress(tokensUsed, describeToolCall(block.Name, args))
				}

				var data string
				if tool, ok := toolDefinitions[block.Name]; ok {
					if !viewerAllowedTools[block.Name] {
						return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("tool %q is not permitted in the unattended insight pipeline", block.Name)
					}
					data = tool(args, toolCtx, ai.valkeyClient, ai.logger)
					ai.auditInsightToolCall(toolCtx, block.Name, args, data)
				} else {
					return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("unknown tool called: %s", block.Name)
				}

				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, data, false))
			}

		}

		if !hasToolUse {
			ai.logger.Info("No tool calls found, finishing AI processing")

			// The model stopped calling tools: with submitted findings on
			// record that simply ends the run.
			if len(collected) > 0 {
				return collected, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
			}

			// Fallback for models that answer in text despite the
			// submit_analysis instruction: parse the JSON out of the text and,
			// when that fails, spend a bounded repair turn pointing the model
			// at the tool instead of discarding the whole exploration.
			var responseText string
			for _, block := range message.Content {
				if block.Type == "text" {
					responseText += block.Text
				}
			}
			aiResponse, removedText, err := parseAiResponse(responseText)
			if err == nil {
				ai.logger.Info("Extracted JSON from AI response", "removed_text", removedText)
				return aiResponse, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
			}
			repairAttempts++
			ai.logger.Warn("Final answer unparsable, requesting repair", "error", err, "attempt", repairAttempts)
			if repairAttempts > maxAnalysisRepairs {
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("final answer unparsable after %d repair attempts: %v\n%s", repairAttempts, err, responseText)
			}
			messages = append(messages, anthropic.MessageParam{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewTextBlock(fmt.Sprintf("Your answer could not be processed: %s. Submit your complete final analysis by calling the %s tool.", err.Error(), submitAnalysisToolName)),
				},
			})
			continue
		}

		// Increase global tool call count and check the run budgets (tool
		// calls and, when configured, tokens). Either limit exhausted forces
		// one final submit turn; findings collected so far are kept.
		toolCallCount += iterationToolUses
		budgetExhausted := maxToolCalls > 0 && toolCallCount >= maxToolCalls
		if !budgetExhausted && maxTokensPerRun > 0 && tokensUsed >= maxTokensPerRun {
			budgetExhausted = true
			ai.logger.Info("Per-run token limit reached, forcing final answer", "maxTokensPerRun", maxTokensPerRun, "tokensUsed", tokensUsed)
		}
		if budgetExhausted {
			ai.logger.Info("Run budget exhausted, forcing final answer", "maxToolCalls", maxToolCalls, "toolCallCount", toolCallCount, "tokensUsed", tokensUsed)

			// Hand back the pending tool results plus a nudge and request one
			// last turn with tool choice forced to submit_analysis so the
			// model must deliver the schema-validated final answer. No
			// compaction here: the pending results were never sent, the model
			// needs them for its final verdict (older ones are already
			// compacted after each turn).
			messages = append(messages, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleUser,
				Content: append(toolResults, anthropic.NewTextBlock(finalAnswerNudge)),
			})
			submitChoice := anthropic.ToolChoiceToolParam{Name: submitAnalysisToolName}
			finalMessage, err := client.Messages.New(ctx, anthropic.MessageNewParams{
				Model:     anthropic.Model(model),
				MaxTokens: int64(10000),
				System: []anthropic.TextBlockParam{
					{Type: "text", Text: systemPrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()},
				},
				Messages:   messages,
				Tools:      tools,
				ToolChoice: anthropic.ToolChoiceUnionParam{OfTool: &submitChoice},
			})
			if err != nil {
				if len(collected) > 0 {
					return collected, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
				}
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("max tool calls reached (%d) and final answer request failed: %w", maxToolCalls, err)
			}
			tokensUsed += finalMessage.Usage.InputTokens + finalMessage.Usage.OutputTokens
			if onProgress != nil {
				onProgress(tokensUsed, "submitting final analysis")
			}

			var responseText string
			for _, block := range finalMessage.Content {
				switch block.Type {
				case "tool_use":
					if block.Name == submitAnalysisToolName {
						findings, parseErr := parseSubmittedAnalysis(block.Input)
						if parseErr != nil {
							if len(collected) > 0 {
								return collected, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
							}
							return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("max tool calls reached (%d) and final analysis invalid: %w", maxToolCalls, parseErr)
						}
						return append(collected, findings...), tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
					}
				case "text":
					responseText += block.Text
				}
			}

			// Defensive fallback: the model answered in text despite the
			// forced tool choice.
			aiResponses, removedText, err := parseAiResponse(responseText)
			ai.logger.Info("Extracted JSON after max tool calls", "removed_text", removedText)
			if err != nil {
				if len(collected) > 0 {
					return collected, tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
				}
				return nil, tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("max tool calls reached (%d) without parsable final answer: %v\n%s", maxToolCalls, err, responseText)
			}
			return append(collected, aiResponses...), tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
		}

		// Add tool results to messages
		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleUser,
			Content: toolResults,
		})

		// Continue the loop to get the next response with tool results
	}
}
