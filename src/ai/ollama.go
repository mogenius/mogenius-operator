package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
)

func (ai *aiManager) processPromptOllama(ctx context.Context, rc *ResolvedModelConfig, systemPrompt, prompt string, toolCtx *ToolContext, onProgress func(int64, string), recordStep StepRecorder) (int64, int, string, error) {

	startTime := time.Now()
	elapsed := func() int { return int(time.Since(startTime).Milliseconds()) }

	model := rc.Model
	maxToolCalls := rc.MaxToolCalls
	maxTokensPerRun := rc.MaxTokensPerRun

	client, err := ai.newOllamaClientFor(rc)
	if err != nil {
		return 0, elapsed(), model, err
	}

	var mcpSessions []string
	if toolCtx != nil {
		mcpSessions = toolCtx.McpSessions
	}
	allTools := ai.mcpManager.GetOllamaToolsForSessions(mcpSessions)
	if toolCtx == nil || !toolCtx.DisableKubernetes {
		allTools = append(allTools, kubernetesOllamaTools...)
	}
	if toolCtx == nil || !toolCtx.DisableHelm {
		allTools = append(allTools, helmOllamaTools...)
	}

	messages := []api.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
	}

	falsePtr := false
	truePtr := true
	var tokensUsed int64 = 0
	toolCallCount := 0

	toolResult := func(name, content string) api.Message {
		return api.Message{Role: "tool", ToolName: name, Content: content}
	}

	for {
		req := &api.ChatRequest{
			Model:    model,
			Messages: messages,
			Stream:   &falsePtr,
			Truncate: &truePtr,
			Shift:    &truePtr,
			Tools:    allTools,
			Options: map[string]any{
				"temperature": 0.1,
			},
		}

		var responseText string
		var toolCalls []api.ToolCall
		err = client.Chat(ctx, req, func(resp api.ChatResponse) error {
			responseText += resp.Message.Content
			if resp.Done {
				tokensUsed += int64(resp.PromptEvalCount + resp.EvalCount)
				toolCalls = resp.Message.ToolCalls
			}
			return nil
		})
		if err != nil {
			return tokensUsed, elapsed(), model, err
		}
		if onProgress != nil {
			onProgress(tokensUsed, "")
		}

		messages = append(messages, api.Message{
			Role:      "assistant",
			Content:   responseText,
			ToolCalls: toolCalls,
		})

		// Assistant free text between tool calls is the model's reasoning.
		if recordStep != nil && strings.TrimSpace(responseText) != "" {
			recordStep(AiRunStep{Kind: AI_RUN_STEP_REASON, Label: responseText})
		}

		if len(toolCalls) == 0 {
			ai.logger.Info("No tool calls, run complete")
			return tokensUsed, elapsed(), model, nil
		}

		// Process each tool call
		for _, toolCall := range toolCalls {
			// A canceled run must not start further tool calls — without this
			// check every remaining call of the turn runs to completion before
			// the next LLM request notices the dead context.
			if ctx.Err() != nil {
				return tokensUsed, elapsed(), model, ctx.Err()
			}
			name := toolCall.Function.Name
			ai.logger.Info("Processing tool call", "tool", name)

			argsBytes, marshalErr := json.Marshal(toolCall.Function.Arguments)
			if marshalErr != nil {
				messages = append(messages, toolResult(name, fmt.Sprintf("Error marshaling tool arguments: %v", marshalErr)))
				continue
			}

			var args map[string]any
			if unmarshalErr := json.Unmarshal(argsBytes, &args); unmarshalErr != nil {
				messages = append(messages, toolResult(name, fmt.Sprintf("Error parsing tool arguments: %v", unmarshalErr)))
				continue
			}
			if onProgress != nil {
				onProgress(tokensUsed, describeToolCall(name, args))
			}

			builtinTool, isBuiltin := toolDefinitions[name]
			switch {
			case isBuiltin:
				var data string
				execCtx := toolCtx
				if mutatingBuiltinTools[name] && toolCtx != nil && toolCtx.CreateApprovalRequest != nil {
					approverCtx, approvalErr := toolCtx.CreateApprovalRequest(ctx, name, args)
					if approvalErr != nil {
						data = fmt.Sprintf("Tool call %q was not executed: %v", name, approvalErr)
					} else {
						execCtx = approverCtx
						data = builtinTool(args, execCtx, ai.valkeyClient, ai.logger)
					}
				} else if mutatingBuiltinTools[name] {
					data = fmt.Sprintf("Tool call %q requires human approval but no approval mechanism is configured for this run. The call was blocked.", name)
				} else {
					data = builtinTool(args, execCtx, ai.valkeyClient, ai.logger)
				}
				ai.auditInsightToolCall(execCtx, name, args, data, nil)
				if recordStep != nil {
					recordStep(AiRunStep{Kind: AI_RUN_STEP_ACT, Label: describeToolCall(name, args), Tool: name, Args: string(argsBytes), Result: data})
				}
				messages = append(messages, toolResult(name, data))
			case ai.mcpManager.IsMCPToolInSessions(name, mcpSessions):
				var data string
				auditCtx := toolCtx
				if ai.mcpManager.MCPToolNeedsApproval(name, mcpSessions) {
					approverCtx, approvalErr := toolCtx.CreateApprovalRequest(ctx, name, args)
					if approvalErr != nil {
						data = fmt.Sprintf("Tool call %q was not executed: %v", name, approvalErr)
					} else {
						auditCtx = approverCtx
					}
				}
				var mcpErr error
				if data == "" {
					var mcpResult string
					mcpResult, mcpErr = ai.mcpManager.CallToolInSessions(ctx, name, args, mcpSessions)
					if mcpErr != nil {
						data = fmt.Sprintf("MCP tool error: %v", mcpErr)
					} else {
						data = mcpResult
					}
				}
				ai.auditInsightToolCall(auditCtx, name, args, data, mcpErr)
				if recordStep != nil {
					recordStep(AiRunStep{Kind: AI_RUN_STEP_ACT, Label: describeToolCall(name, args), Tool: name, Args: string(argsBytes), Result: data})
				}
				messages = append(messages, toolResult(name, data))

			default:
				messages = append(messages, toolResult(name, fmt.Sprintf("Unknown tool %q — only the tools offered in this conversation exist.", name)))
			}
		}

		// Check run budgets (tool calls and, when configured, tokens).
		toolCallCount += len(toolCalls)
		budgetExhausted := maxToolCalls > 0 && toolCallCount >= maxToolCalls
		if !budgetExhausted && maxTokensPerRun > 0 && tokensUsed >= maxTokensPerRun {
			budgetExhausted = true
			ai.logger.Info("Per-run token limit reached", "maxTokensPerRun", maxTokensPerRun, "tokensUsed", tokensUsed)
		}
		if budgetExhausted {
			ai.logger.Info("Run budget exhausted", "maxToolCalls", maxToolCalls, "toolCallCount", toolCallCount)
			return tokensUsed, elapsed(), model, nil
		}

		// Continue the loop to get the next response with tool results
	}
}

func (ai *aiManager) ollamaChat(
	ctx context.Context,
	ioChannel IOChatChannel,
	systemPrompt string,
	rc *ResolvedModelConfig,
) error {

	maxToolCalls := rc.MaxToolCalls
	client, err := ai.newOllamaClientFor(rc)
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

	// Build full tool set once per session (static + MCP, filtered by role)
	allOllamaTools := append(kubernetesOllamaTools, helmOllamaTools...)
	if !isViewerRole(ioChannel) {
		allOllamaTools = append(allOllamaTools, activateToolCategoriesOllama)
	}
	if ai.mcpManager != nil {
		allOllamaTools = append(allOllamaTools, ai.mcpManager.GetOllamaTools()...)
	}
	allOllamaTools = filterOllamaTools(allOllamaTools, ioChannel)

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

			messages = append(messages, api.Message{
				Role:    "user",
				Content: userInput,
			})

			// Pass allTools + categories so the inner loop can recompute
			// active tools after the LLM activates new categories
			fullResponse, updatedMessages, turnStats, err := ai.ollamaChatWithTools(ctx, client, rc, messages, ioChannel, allOllamaTools, categories, maxToolCalls, &sessionInputTokens, &sessionOutputTokens)
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

// ollamaChatWithTools handles the AI request with streaming and tool call support.
func (ai *aiManager) ollamaChatWithTools(
	ctx context.Context,
	client *api.Client,
	rc *ResolvedModelConfig,
	messages []api.Message,
	ioChannel IOChatChannel,
	allOllamaTools []api.Tool,
	categories *ActiveToolCategories,
	maxToolCalls int,
	sessionInputTokens *int64,
	sessionOutputTokens *int64,
) (fullResponse string, updatedMessages []api.Message, stats ChatTurnStats, err error) {
	model := rc.Model
	toolCallCount := 0
	truePtr := true
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
		ollamaTools := filterOllamaToolsByCategory(allOllamaTools, categories)

		// Notify user that AI is thinking
		select {
		case ioChannel.Output <- "[AI is thinking...]\n":
		case <-ctx.Done():
			return "", messages, stats, ctx.Err()
		}

		req := &api.ChatRequest{
			Model:    model,
			Messages: messages,
			Stream:   &truePtr,
			Truncate: &truePtr,
			Shift:    &truePtr,
			Tools:    ollamaTools,
			Options: map[string]any{
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

			// In streaming mode Ollama delivers tool calls in intermediate
			// chunks (done=false); the final done chunk has none. Accumulate
			// them from every chunk.
			if len(resp.Message.ToolCalls) > 0 {
				toolCalls = append(toolCalls, resp.Message.ToolCalls...)
				for _, tc := range resp.Message.ToolCalls {
					select {
					case ioChannel.Output <- fmt.Sprintf("\n[Using tool: %s]\n", tc.Function.Name):
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}

			if resp.Done {
				inputTokens = int64(resp.PromptEvalCount)
				outputTokenCount = int64(resp.EvalCount)
				*sessionInputTokens += inputTokens
				*sessionOutputTokens += outputTokenCount
				inputTokensUsed += inputTokens
				outputTokensUsed += outputTokenCount
				ai.logger.Info("Stream usage", "input_tokens", inputTokens, "output_tokens", outputTokenCount,
					"session_input_tokens", *sessionInputTokens, "session_output_tokens", *sessionOutputTokens)
			}

			return nil
		})

		if err != nil {
			return "", messages, stats, fmt.Errorf("streaming error: %w", err)
		}

		ai.sendTokens(inputTokens, outputTokenCount, sessionInputTokens, sessionOutputTokens, ctx, ioChannel)

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

		select {
		case ioChannel.Output <- "\n\n":
		case <-ctx.Done():
			return "", messages, stats, ctx.Err()
		}

		// No tool calls — just a text response
		if len(toolCalls) == 0 {
			response := fullText.String()
			messages = append(messages, api.Message{
				Role:    "assistant",
				Content: response,
			})
			return response, messages, stats, nil
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
			// Replace the just-appended assistant tool_calls message with a
			// text-only one so messages[-1] is always a valid assistant text
			// message and never an unmatched tool_calls entry.
			text := fullText.String()
			if text == "" {
				text = "[Tool call limit reached]"
			}
			messages = messages[:len(messages)-1]
			messages = append(messages, api.Message{
				Role:    "assistant",
				Content: text,
			})
			return text, messages, stats, nil
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
			var toolErr string
			if tc.Function.Name == activateToolCategoriesName {
				result = categories.ActivateFromToolCall(args)
			} else if ai.mcpManager != nil && ai.mcpManager.IsMCPTool(tc.Function.Name) {
				mcpResult, err := ai.mcpManager.CallTool(ctx, tc.Function.Name, args)
				if err != nil {
					ai.logger.Error("MCP tool call failed", "tool", tc.Function.Name, "error", err)
					toolErr = fmt.Sprintf("Error calling MCP tool: %v", err)
					messages = append(messages, api.Message{
						Role:    "tool",
						Content: toolErr,
					})
					stats.ToolRecords = append(stats.ToolRecords, ToolUseRecord{Tool: tc.Function.Name, Args: args, Error: toolErr})
					continue
				}
				result = mcpResult
			} else if tool, ok := toolDefinitions[tc.Function.Name]; ok {
				result = tool(args, toolCtx, ai.valkeyClient, ai.logger)
			} else {
				ai.logger.Error("Unknown tool called", "tool", tc.Function.Name)
				messages = append(messages, api.Message{
					Role:    "tool",
					Content: fmt.Sprintf("Unknown tool: %s", tc.Function.Name),
				})
				continue
			}
			stats.ToolRecords = append(stats.ToolRecords, ToolUseRecord{Tool: tc.Function.Name, Args: args, Result: truncateToolResult(result)})
			messages = append(messages, api.Message{
				Role:    "tool",
				Content: result,
			})
		}

		// Continue loop to get response after tool results
	}
}
