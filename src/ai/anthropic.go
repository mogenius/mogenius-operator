package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

func (ai *aiManager) anthropicChat(
	ctx context.Context,
	ioChannel IOChatChannel,

	systemPrompt string,
	rc *ResolvedModelConfig,
) error {
	maxToolCalls := rc.MaxToolCalls
	client := ai.newAnthropicClientFor(rc)

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
			if ai.isModelBudgetExceeded(rc) {
				ai.logger.Warn("Daily model token limit exceeded, rejecting input", "model", rc.ModelCrName)
				select {
				case ioChannel.Output <- fmt.Sprintf("\n[Error: %s]", ai.modelBudgetError(rc)):
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
			fullResponse, updatedMessages, turnStats, err := ai.anthropicChatWithTools(ctx, client, systemPrompt, rc, messages, ioChannel, allAnthropicTools, categories, maxToolCalls, &sessionInputTokens, &sessionOutputTokens)
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
	rc *ResolvedModelConfig,
	messages []anthropic.MessageParam,
	ioChannel IOChatChannel,
	allAnthropicTools []anthropic.ToolParam,
	categories *ActiveToolCategories,
	maxToolCalls int,
	sessionInputTokens *int64,
	sessionOutputTokens *int64,
) (fullResponse string, updatedMessages []anthropic.MessageParam, stats ChatTurnStats, err error) {
	model := rc.Model
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
				if addErr := ai.addTokenUsage(int(inputTokensUsed+outputTokensUsed), model, timeUsedInMs, chatKey, rc.ModelCrName); addErr != nil {
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
		exec := toolExec{ToolCtx: toolCtx, Categories: categories, Stats: &stats}
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

			out := ai.dispatchToolCall(ctx, toolUse.Name, args, "", exec)
			toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, out.Result, out.IsError))
		}

		// Add tool results to messages for next iteration
		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleUser,
			Content: toolResults,
		})

		// Continue loop to get response after tool calls
	}
}

// assistantContentParams converts response content blocks into request param
// blocks so an assistant turn can be appended back onto the conversation.
// Unknown block types are skipped instead of producing empty union values.
func assistantContentParams(blocks []anthropic.ContentBlockUnion) ([]anthropic.ContentBlockParamUnion, error) {
	params := make([]anthropic.ContentBlockParamUnion, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text":
			params = append(params, anthropic.NewTextBlock(block.Text))
		case "tool_use":
			// Unmarshal the input to a map so it is sent as a dictionary.
			var input map[string]any
			if err := json.Unmarshal(block.Input, &input); err != nil {
				return nil, fmt.Errorf("error unmarshaling tool input: %v", err)
			}
			params = append(params, anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{
					ID:    block.ID,
					Name:  block.Name,
					Input: input,
				},
			})
		}
	}
	return params, nil
}

// compactAnthropicMessagesWithAI appends a compaction request to the existing
// messages and calls the model to produce a summary. Returns a replacement list
// with only the original user prompt and the compaction summary, so all future
// calls carry minimal history. On success the caller must reset cachedMsgIdx to -1.
func (ai *aiManager) compactAnthropicMessagesWithAI(ctx context.Context, client *anthropic.Client, model string, messages []anthropic.MessageParam) ([]anthropic.MessageParam, int64, error) {
	if len(messages) < 1 {
		return nil, 0, fmt.Errorf("message list too short to compact")
	}
	compactionRequest := anthropic.MessageParam{
		Role: anthropic.MessageParamRoleUser,
		Content: []anthropic.ContentBlockParamUnion{
			anthropic.NewTextBlock("Compact this conversation into a concise first-person progress report."),
		},
	}

	compactionMessages := append(slices.Clone(messages), compactionRequest)

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(10000),
		System: []anthropic.TextBlockParam{
			{Type: "text", Text: compactionSystemPrompt},
		},
		Messages: compactionMessages,
	})
	if err != nil {
		return nil, 0, err
	}

	// Concatenate every text block: the model may emit multiple text blocks
	// (or lead with a non-text block), and taking only the first would drop
	// part of the summary.
	var sb strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(block.Text)
		}
	}
	summary := sb.String()

	tokensUsed := resp.Usage.InputTokens + resp.Usage.OutputTokens + resp.Usage.CacheCreationInputTokens

	return buildCompactedAnthropicMessages(messages[0], summary), tokensUsed, nil
}

func (ai *aiManager) processPromptAnthropic(ctx context.Context, rc *ResolvedModelConfig, systemPrompt, prompt string, toolCtx *ToolContext, onProgress func(int64, string), recordStep StepRecorder) (int64, int, string, error) {
	startTime := time.Now()

	model := rc.Model
	maxToolCalls := rc.MaxToolCalls
	maxTokensPerRun := rc.MaxTokensPerRun

	var mcpSessions []string
	if toolCtx != nil {
		mcpSessions = toolCtx.McpSessions
	}
	allTools := ai.mcpManager.GetAnthropicToolsForSessions(mcpSessions)
	if toolCtx == nil || !toolCtx.DisableKubernetes {
		allTools = append(allTools, kubernetesAnthropicTools...)
	}
	if toolCtx == nil || !toolCtx.DisableHelm {
		allTools = append(allTools, helmAnthropicTools...)
	}

	tools := make([]anthropic.ToolUnionParam, len(allTools))
	for i, toolParam := range allTools {
		if i == len(allTools)-1 {
			toolParam.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
		tools[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
	}

	client := ai.newAnthropicClientFor(rc)

	messages := []anthropic.MessageParam{
		{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock(prompt),
			},
		},
	}

	var tokensUsed int64 = 0

	// Cache reads cost ~10% of fresh input and are not budgeted — logged so
	// the full provider-reported picture stays visible.
	var cacheReadTokens int64 = 0
	defer func() {
		ai.logger.Info("AI run token usage", "tokensUsed", tokensUsed, "cacheReadTokens", cacheReadTokens)
	}()

	// Track total number of tool calls across iterations
	toolCallCount := 0

	// Index of the message currently carrying the moving cache breakpoint.
	cachedMsgIdx := -1

	exec := toolExec{
		ToolCtx:           toolCtx,
		McpSessions:       mcpSessions,
		ScopedMCP:         true,
		InterceptMutating: true,
		UnknownIsFatal:    true,
		Audit:             true,
		RecordStep:        recordStep,
	}

	// Loop until there are no more tool calls or maxToolCalls reached
	for {
		moveCacheBreakpoint(messages, &cachedMsgIdx)
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
			return tokensUsed, int(time.Since(startTime).Milliseconds()), model, err
		}

		// Compact the conversation if it exceeds the character threshold
		if estimateMessagesChars(messages) > compactHistoryAfterChars {
			charsBefore := estimateMessagesChars(messages)
			ai.logger.Info("Compacting conversation history with AI", "chars", charsBefore)
			compacted, compactTokens, compactErr := ai.compactAnthropicMessagesWithAI(ctx, client, model, messages)
			if compactErr != nil {
				ai.logger.Warn("AI compaction failed", "error", compactErr)
				if recordStep != nil {
					recordStep(AiRunStep{Kind: AI_RUN_STEP_ERROR, Label: fmt.Sprintf("Conversation compaction failed: %v", compactErr)})
				}
			} else {
				messages = compacted
				tokensUsed += compactTokens
				cachedMsgIdx = -1
				ai.logger.Info("AI compaction complete", "charsBefore", charsBefore, "charsAfter", estimateMessagesChars(messages))
			}
		}

		if message != nil {
			// Provider-reported usage, taken verbatim: fresh input, output and
			// cache writes count against the budgets; cache reads are tracked
			// separately.
			tokensUsed += message.Usage.InputTokens + message.Usage.OutputTokens + message.Usage.CacheCreationInputTokens
			cacheReadTokens += message.Usage.CacheReadInputTokens
		}
		if onProgress != nil {
			onProgress(tokensUsed, "")
		}

		if len(message.Content) == 0 {
			return tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("no content returned from AI model")
		}

		// Add the assistant's response to the messages
		assistantContent, err := assistantContentParams(message.Content)
		if err != nil {
			return tokensUsed, int(time.Since(startTime).Milliseconds()), model, err
		}
		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleAssistant,
			Content: assistantContent,
		})

		// Assistant free text between tool calls is the model's reasoning.
		if recordStep != nil {
			for _, block := range message.Content {
				if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
					recordStep(AiRunStep{Kind: AI_RUN_STEP_REASON, Label: block.Text})
				}
			}
		}

		// Check if there are tool calls to process
		hasToolUse := false
		var toolResults []anthropic.ContentBlockParamUnion
		iterationToolUses := 0

		for _, block := range message.Content {
			if block.Type == "tool_use" {
				// A canceled run must not start further tool calls — without
				// this check every remaining call of the turn runs to
				// completion before the next LLM request notices the dead
				// context.
				if ctx.Err() != nil {
					return tokensUsed, int(time.Since(startTime).Milliseconds()), model, ctx.Err()
				}
				hasToolUse = true
				iterationToolUses++
				ai.logger.Info("Processing tool call", "tool", block.Name)

				// Extract the arguments from the tool use
				var args map[string]any
				inputBytes, err := json.Marshal(block.Input)
				if err != nil {
					return tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error marshaling tool input: %v", err)
				}
				err = json.Unmarshal(inputBytes, &args)
				if err != nil {
					return tokensUsed, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("error unmarshaling tool arguments: %v", err)
				}

				if onProgress != nil {
					onProgress(tokensUsed, describeToolCall(block.Name, args))
				}

				out := ai.dispatchToolCall(ctx, block.Name, args, string(inputBytes), exec)
				if out.Fatal != nil {
					return tokensUsed, int(time.Since(startTime).Milliseconds()), model, out.Fatal
				}

				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, out.Result, false))
			}

		}

		if !hasToolUse {
			ai.logger.Info("No tool calls, run complete")
			return tokensUsed, int(time.Since(startTime).Milliseconds()), model, nil
		}

		// Increase global tool call count and check the run budgets.
		toolCallCount += iterationToolUses
		if msg := runBudgetExhausted(ai.logger, maxToolCalls, toolCallCount, maxTokensPerRun, tokensUsed); msg != "" {
			return tokensUsed, int(time.Since(startTime).Milliseconds()), model, &BudgetExhaustedError{Msg: msg}
		}

		// Add tool results to messages
		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleUser,
			Content: toolResults,
		})

		// Continue the loop to get the next response with tool results
	}
}
