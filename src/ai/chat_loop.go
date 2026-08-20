package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mogenius-operator/src/ai/aisdk"
)

// runChatTurn executes one interactive chat turn with streaming, handling the
// inner tool-call loop. It replaces the three provider-specific *ChatWithTools
// functions.
func (ai *aiManager) runChatTurn(
	ctx context.Context,
	provider aisdk.Provider,
	rc *ResolvedModelConfig,
	messages []aisdk.Message,
	ioChannel IOChatChannel,
	allTools []aisdk.Tool,
	categories *ActiveToolCategories,
	maxToolCalls int,
	sessionIn *int64,
	sessionOut *int64,
) (fullResponse string, updatedMessages []aisdk.Message, stats ChatTurnStats, err error) {
	model := rc.Model
	toolCallCount := 0
	toolCtx := newToolContextFromIOChannel(ioChannel)
	stats.Model = model
	turnStartIn := *sessionIn
	turnStartOut := *sessionOut
	startTime := time.Now()
	defer func() {
		stats.InputTokens = *sessionIn - turnStartIn
		stats.OutputTokens = *sessionOut - turnStartOut
		stats.DurationMs = int(time.Since(startTime).Milliseconds())
	}()

	for {
		activeTools := filterAiSDKToolsByCategory(allTools, categories)

		select {
		case ioChannel.Output <- "[AI is thinking...]\n":
		case <-ctx.Done():
			return "", messages, stats, ctx.Err()
		}

		ch, streamErr := provider.ChatStream(ctx, model, messages, activeTools)
		if streamErr != nil {
			return "", messages, stats, streamErr
		}

		var fullText strings.Builder
		var toolCalls []aisdk.ToolCall
		var inputTokensThisTurn, outputTokensThisTurn int64

		for chunk := range ch {
			if chunk.Err != nil {
				return "", messages, stats, chunk.Err
			}
			if chunk.ToolCallName != "" {
				select {
				case ioChannel.Output <- fmt.Sprintf("\n[Using tool: %s]\n", chunk.ToolCallName):
				case <-ctx.Done():
					return "", messages, stats, ctx.Err()
				}
			}
			if chunk.Content != "" {
				fullText.WriteString(chunk.Content)
				select {
				case ioChannel.Output <- chunk.Content:
				case <-ctx.Done():
					return "", messages, stats, ctx.Err()
				}
			}
			if chunk.InputTokens > 0 {
				inputTokensThisTurn = chunk.InputTokens
			}
			if chunk.OutputTokens > 0 {
				outputTokensThisTurn = chunk.OutputTokens
			}
			if chunk.Done {
				toolCalls = chunk.ToolCalls
			}
		}

		*sessionIn += inputTokensThisTurn
		*sessionOut += outputTokensThisTurn
		ai.sendTokens(inputTokensThisTurn, outputTokensThisTurn, sessionIn, sessionOut, ctx, ioChannel)

		chatKey := "chat"
		if ioChannel.User != nil && ioChannel.User.Email != "" {
			chatKey = fmt.Sprintf("chat:%s", ioChannel.User.Email)
		}
		if addErr := ai.addTokenUsage(int(inputTokensThisTurn+outputTokensThisTurn), model, int(time.Since(startTime).Milliseconds()), chatKey, rc.ModelCrName); addErr != nil {
			ai.logger.Error("Error recording chat token usage", "error", addErr)
		}

		select {
		case ioChannel.Output <- "\n\n":
		case <-ctx.Done():
			return "", messages, stats, ctx.Err()
		}

		// Append assistant turn to history.
		assistantMsg := aisdk.Message{
			Role:      aisdk.RoleAssistant,
			Content:   fullText.String(),
			ToolCalls: toolCalls,
		}
		messages = append(messages, assistantMsg)

		if len(toolCalls) == 0 {
			return fullText.String(), messages, stats, nil
		}

		// Tool call limit check.
		toolCallCount += len(toolCalls)
		if maxToolCalls > 0 && toolCallCount >= maxToolCalls {
			ai.logger.Warn("Max tool calls reached", "count", toolCallCount)
			text := fullText.String()
			if text == "" {
				text = "[Tool call limit reached]"
			}
			messages[len(messages)-1] = aisdk.Message{Role: aisdk.RoleAssistant, Content: text}
			return text, messages, stats, nil
		}

		// Execute tool calls.
		exec := toolExec{ToolCtx: toolCtx, Categories: categories, Stats: &stats}
		for _, tc := range toolCalls {
			if ctx.Err() != nil {
				return "", messages, stats, ctx.Err()
			}
			ai.logger.Info("Executing tool", "tool", tc.Name)

			var args map[string]any
			if jsonErr := json.Unmarshal([]byte(tc.Arguments), &args); jsonErr != nil {
				ai.logger.Error("Error parsing tool arguments", "error", jsonErr)
				messages = append(messages, aisdk.Message{
					Role:       aisdk.RoleTool,
					Content:    fmt.Sprintf("Error parsing arguments: %v", jsonErr),
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
				})
				continue
			}

			out := ai.dispatchToolCall(ctx, tc.Name, args, "", exec)
			messages = append(messages, aisdk.Message{
				Role:       aisdk.RoleTool,
				Content:    out.Result,
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
			})
		}
	}
}

// runChatSession drives the interactive chat session loop. It replaces the
// three provider-specific *Chat functions.
func (ai *aiManager) runChatSession(
	ctx context.Context,
	provider aisdk.Provider,
	rc *ResolvedModelConfig,
	systemPrompt string,
	ioChannel IOChatChannel,
) error {
	maxToolCalls := rc.MaxToolCalls

	// Build full tool set once per session (static + MCP, filtered by role).
	allTools := append(kubernetesAiSDKTools, helmAiSDKTools...)
	if !isViewerRole(ioChannel) {
		allTools = append(allTools, activateToolCategoriesAiSDK)
	}
	if ai.mcpManager != nil {
		allTools = append(allTools, ai.mcpManager.GetAiSDKTools()...)
	}
	allTools = filterAiSDKTools(allTools, ioChannel)

	// Session-level category filter (sticky, driven by LLM via meta-tool).
	categories := NewActiveToolCategories()

	// Session-level accumulated token counters.
	var sessionIn, sessionOut int64

	// Conversation history starts with the system prompt.
	messages := []aisdk.Message{
		{Role: aisdk.RoleSystem, Content: systemPrompt, CacheControl: true},
	}

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

			messages = append(messages, aisdk.Message{Role: aisdk.RoleUser, Content: userInput})

			fullResponse, updatedMessages, turnStats, turnErr := ai.runChatTurn(
				ctx, provider, rc, messages, ioChannel, allTools, categories, maxToolCalls, &sessionIn, &sessionOut,
			)
			if turnErr != nil {
				ai.logger.Error("Error processing with tools", "error", turnErr)
				payload := map[string]any{"question": userInput, "stats": turnStats}
				emitAuditEvent(ioChannel, "ai/chat", payload, nil, turnErr.Error())
				select {
				case ioChannel.Output <- fmt.Sprintf("\n[Error: %v]", turnErr):
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}

			payload := map[string]any{"question": userInput, "response": truncateToolResult(fullResponse), "stats": turnStats}
			emitAuditEvent(ioChannel, "ai/chat", payload, nil, "")

			// Keep only the final assistant text response in history (drop intermediate
			// tool exchanges) to prevent large tool results from accumulating in context.
			messages = append(messages, updatedMessages[len(updatedMessages)-1])

			select {
			case ioChannel.Output <- "[COMPLETED]":
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
