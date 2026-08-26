package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"mogenius-operator/src/ai/aisdk"
)

// runAgentLoop executes the unattended tool-call loop using a provider-neutral
// aisdk.Provider. It replaces the three provider-specific processPrompt* functions.
//
// messages must start with a RoleSystem message followed by the initial user prompt.
// The caller is responsible for building the messages slice and the tools list.
func (ai *aiManager) runAgentLoop(
	ctx context.Context,
	provider aisdk.Provider,
	model string,
	messages []aisdk.Message,
	tools []aisdk.Tool,
	toolCtx *ToolContext,
	mcpSessions []string,
	maxToolCalls int,
	maxTokensPerRun int64,
	onProgress func(int64, string),
	recordStep StepRecorder,
) (tokensUsed int64, err error) {
	toolCallCount := 0
	cachedMsgIdx := -1
	var cacheReadTokens int64

	defer func() {
		if cacheReadTokens > 0 {
			ai.logger.Info("AI run cache read tokens (not counted toward budget)", "cacheReadTokens", cacheReadTokens)
		}
	}()

	exec := toolExec{
		ToolCtx:           toolCtx,
		McpSessions:       mcpSessions,
		ScopedMCP:         true,
		InterceptMutating: true,
		UnknownIsFatal:    true,
		Audit:             true,
		RecordStep:        recordStep,
	}

	// mover is non-nil for AnthropicProvider only.
	mover, _ := provider.(aisdk.CacheBreakpointMover)

	// Resolve compaction threshold: 80 % of the model's context window expressed
	// as byte length of the JSON-serialised conversation (~4 bytes/token for
	// ASCII-heavy content). When the provider cannot report the context window
	// (e.g. a custom base URL that does not expose /v1/models), compaction is
	// disabled for the run — the model's own context-limit rejection (caught as
	// FinishReason == "length") acts as the backstop.
	compactAfterBytes := math.MaxInt
	if windowTokens, err := provider.ContextWindowTokens(ctx, model); err == nil && windowTokens > 0 {
		compactAfterBytes = int(windowTokens * 4 * 80 / 100) // tokens → bytes → 80 %
	} else if err != nil {
		ai.logger.Warn("could not determine model context window; compaction disabled for this run", "error", err)
	}

	for {
		reasoningStep := recordStep.Reason("Running AI model...")
		if mover != nil {
			mover.MoveCacheBreakpoint(messages, &cachedMsgIdx)
		}

		resp, callErr := provider.Chat(ctx, model, messages, tools)
		if callErr != nil {
			reasoningStep(AiRunStepStatusErrored, "AI model call failed", callErr.Error())
			return tokensUsed, callErr
		}

		tokensUsed += resp.InputTokens + resp.OutputTokens
		cacheReadTokens += resp.CacheReadTokens
		if onProgress != nil {
			onProgress(tokensUsed, "")
		}

		// Compact conversation history when it grows too large.
		bytesBefore := estimateAiSDKMessageBytes(messages)
		if bytesBefore > compactAfterBytes {
			compactionStep := recordStep.Compaction("Running Compaction...")
			compacted, compactTokens, compactErr := ai.compactMessagesWithAI(ctx, provider, model, messages)
			if compactErr != nil {
				compactionStep(AiRunStepStatusErrored, "Conversation compaction failed", compactErr.Error())
				return tokensUsed, &CompactionError{Cause: compactErr}
			} else {
				compactionStep(AiRunStepStatusFinished, "Compaction finished successfully", "")
				messages = compacted
				tokensUsed += compactTokens
				cachedMsgIdx = -1
			}
		}

		if resp.FinishReason == "length" {
			reasoningStep(AiRunStepStatusErrored, "Context limit reached", "The model stopped because it reached its maximum context length.")
			return tokensUsed, &BudgetExhaustedError{
				Msg: "run stopped: model reached its maximum context length",
			}
		}

		if resp.Content == "" && len(resp.ToolCalls) == 0 {
			reasoningStep(AiRunStepStatusFinished, "No content or tool calls returned; run complete", "")
			return tokensUsed, fmt.Errorf("no content returned from AI model")
		}

		// Append assistant message to history.
		messages = append(messages, aisdk.Message{
			Role:      aisdk.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		var reasoningLabel string
		switch {
		case resp.Content != "" && len(resp.ToolCalls) > 0:
			reasoningLabel = "Finished Reasoning, Calling Tools"
		case len(resp.ToolCalls) > 0:
			reasoningLabel = "Calling Tools"
		default:
			reasoningLabel = "Finished Reasoning"
		}
		reasoningStep(AiRunStepStatusFinished, reasoningLabel, strings.TrimSpace(resp.Content))

		if len(resp.ToolCalls) == 0 {
			ai.logger.Info("No tool calls, run complete")
			return tokensUsed, nil
		}

		// Execute each tool call and collect results.
		dispatchedToolCalls := 0
		for _, tc := range resp.ToolCalls {
			if ctx.Err() != nil {
				return tokensUsed, ctx.Err()
			}
			ai.logger.Info("Processing tool call", "tool", tc.Name)

			var args map[string]any
			if jsonErr := json.Unmarshal([]byte(tc.Arguments), &args); jsonErr != nil {
				// Feed the parse error back to the model so it can self-correct,
				// consistent with how chat_loop.go handles this case.
				ai.logger.Warn("Failed to parse tool arguments", "tool", tc.Name, "error", jsonErr)
				messages = append(messages, aisdk.Message{
					Role:       aisdk.RoleTool,
					Content:    fmt.Sprintf("Error parsing arguments: %v", jsonErr),
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
				})
				continue
			}

			if onProgress != nil {
				onProgress(tokensUsed, describeToolCall(tc.Name, args))
			}

			out := ai.dispatchToolCall(ctx, tc.Name, args, tc.Arguments, exec)
			if out.Fatal != nil {
				return tokensUsed, out.Fatal
			}

			messages = append(messages, aisdk.Message{
				Role:       aisdk.RoleTool,
				Content:    out.Result,
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
			})
			dispatchedToolCalls++
		}

		toolCallCount += dispatchedToolCalls
		if msg := runBudgetExhausted(ai.logger, maxToolCalls, toolCallCount, maxTokensPerRun, tokensUsed); msg != "" {
			recordStep.Error(msg)
			return tokensUsed, &BudgetExhaustedError{Msg: msg}
		}
	}
}

// buildAgentMessages constructs the initial message slice for an agent run:
// [systemPrompt, userPrompt].
func buildAgentMessages(systemPrompt, userPrompt string) []aisdk.Message {
	return []aisdk.Message{
		{Role: aisdk.RoleSystem, Content: systemPrompt, CacheControl: true},
		{Role: aisdk.RoleUser, Content: userPrompt},
	}
}

// buildAgentTools assembles the tool list for an unattended agent run from
// MCP sessions and the built-in kubernetes/helm tool sets.
func buildAgentTools(mcpManager *mcpClientManager, mcpSessions []string, toolCtx *ToolContext) []aisdk.Tool {
	tools := mcpManager.GetAiSDKToolsForSessions(mcpSessions)
	if toolCtx == nil || !toolCtx.DisableKubernetes {
		tools = append(tools, kubernetesAiSDKTools...)
	}
	if toolCtx == nil || !toolCtx.DisableHelm {
		tools = append(tools, helmAiSDKTools...)
	}
	return tools
}
