package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
)

// processPromptOpenAi runs the unattended agent-run protocol against an
// OpenAI-compatible endpoint, mirroring the Anthropic path: strictly
// read-only inspection tools plus the repeatable submit_analysis tool through
// which all findings arrive. Unlike Anthropic, changing tool_choice carries
// no prompt-cache penalty here, so the budget-exhausted final turn forces the
// submit tool directly instead of relying on nudge-and-refuse rounds.
func (ai *aiManager) processPromptOpenAi(ctx context.Context, rc *ResolvedModelConfig, systemPrompt, prompt string, toolCtx *ToolContext, onProgress func(int64, string)) ([]*AiResponse, int64, int, string, error) {
	startTime := time.Now()
	elapsed := func() int { return int(time.Since(startTime).Milliseconds()) }

	model := rc.Model
	maxToolCalls := rc.MaxToolCalls
	maxTokensPerRun := rc.MaxTokensPerRun

	client := ai.newOpenAIClientFor(rc)

	// Unattended pipeline: strictly read-only tools (defense in depth on top
	// of the ToolContext namespace scoping) plus submit_analysis, so findings
	// arrive as structured tool input instead of JSON scraped out of text.
	allTools := readOnlyOpenAiTools(append(kubernetesOpenAiTools, helmOpenAiTools...))
	allTools = append(allTools, submitAnalysisOpenAiTool)

	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt + submitAnalysisInstruction),
			openai.UserMessage(prompt),
		},
		Model: model,
		Tools: allTools,
	}

	var tokensUsed int64 = 0
	toolCallCount := 0

	// Successful read-only inspections. A whole-scope run that ends without a
	// single one produced its answer blind — a model failure, not an all-clear.
	inspectionCalls := 0

	// Bounded in-conversation repair turns for schema-violating submissions.
	repairAttempts := 0

	// Set once the run budget is exhausted; from then on tool_choice forces
	// submit_analysis and only a bounded number of turns remain.
	nudged := false
	turnsAfterNudge := 0

	// One repair round for a post-nudge submission with rejected findings.
	repairedOnce := false

	// Findings accumulated across repeated submit_analysis calls. The tool is
	// repeatable so the number of findings is not limited by a single
	// response's output budget.
	collected := []*AiResponse{}

	// blindRunError reports a whole-scope run that ended empty-handed without
	// ever inspecting the cluster. Surfacing this as an error keeps it apart
	// from a genuine all-clear (which is silently discarded upstream).
	blindRunError := func() error {
		if toolCtx != nil && toolCtx.RequireActionableFindings && inspectionCalls == 0 && len(collected) == 0 {
			return fmt.Errorf("model %q ended the run without a single successful inspection tool call and without findings — not treating this as an all-clear; verify that the model handles tool calling reliably", model)
		}
		return nil
	}

	for {
		chatCompletion, err := client.Chat.Completions.New(ctx, params)
		if err != nil {
			if len(collected) > 0 {
				// Salvage what the run already confirmed instead of throwing
				// the whole exploration away.
				ai.logger.Warn("LLM turn failed mid-run, keeping findings collected so far", "collected", len(collected), "error", err)
				return collected, tokensUsed, elapsed(), model, nil
			}
			return nil, tokensUsed, elapsed(), model, err
		}

		// Everything in params.Messages has now been seen by the model —
		// compact the tool results it just processed so they stop burning
		// tokens on the following turns. Compacting BEFORE the call would
		// blind the model: results must survive exactly one request.
		compactOpenAiToolMessages(params.Messages)

		tokensUsed += chatCompletion.Usage.TotalTokens
		if onProgress != nil {
			onProgress(tokensUsed, "")
		}

		if len(chatCompletion.Choices) == 0 {
			if len(collected) > 0 {
				return collected, tokensUsed, elapsed(), model, nil
			}
			return nil, tokensUsed, elapsed(), model, fmt.Errorf("no choices returned from AI model")
		}

		message := chatCompletion.Choices[0].Message
		params.Messages = append(params.Messages, message.ToParam())

		if len(message.ToolCalls) == 0 {
			ai.logger.Info("No tool calls found, finishing AI processing")

			// The model stopped calling tools: with submitted findings on
			// record that simply ends the run.
			if len(collected) > 0 {
				return collected, tokensUsed, elapsed(), model, nil
			}

			// Fallback for models that answer in text despite the
			// submit_analysis instruction: parse the JSON out of the text and,
			// when that fails, spend a bounded repair turn pointing the model
			// at the tool instead of discarding the whole exploration.
			aiResponse, removedText, parseErr := parseAiResponse(message.Content)
			if parseErr == nil {
				ai.logger.Info("Extracted JSON from AI response", "removed_text", removedText)
				if len(aiResponse) == 0 {
					if blindErr := blindRunError(); blindErr != nil {
						return nil, tokensUsed, elapsed(), model, blindErr
					}
				}
				return aiResponse, tokensUsed, elapsed(), model, nil
			}
			repairAttempts++
			ai.logger.Warn("Final answer unparsable, requesting repair", "error", parseErr, "attempt", repairAttempts)
			if repairAttempts > maxAnalysisRepairs {
				return nil, tokensUsed, elapsed(), model, fmt.Errorf("final answer unparsable after %d repair attempts: %v\n%s", repairAttempts, parseErr, message.Content)
			}
			params.Messages = append(params.Messages, openai.UserMessage(fmt.Sprintf("Your answer could not be processed: %s. Submit your findings by calling the %s tool (or call it with an empty findings array if nothing is actionable).", parseErr.Error(), submitAnalysisToolName)))
			continue
		}

		// Process each tool call
		for _, toolCall := range message.ToolCalls {
			name := toolCall.Function.Name
			ai.logger.Info("Processing tool call", "tool", name)

			// The final analysis arrives as tool input; a schema violation is
			// fed back as a tool result so the model repairs it
			// in-conversation instead of failing the whole run.
			if name == submitAnalysisToolName {
				findings, parseErr := parseSubmittedAnalysis(json.RawMessage(toolCall.Function.Arguments))
				if parseErr != nil {
					repairAttempts++
					ai.logger.Warn("Submitted findings rejected", "error", parseErr, "attempt", repairAttempts)
					if repairAttempts > maxAnalysisRepairs {
						if len(collected) > 0 {
							return collected, tokensUsed, elapsed(), model, nil
						}
						return nil, tokensUsed, elapsed(), model, fmt.Errorf("analysis rejected %d times, giving up: %w", repairAttempts, parseErr)
					}
					params.Messages = append(params.Messages, openai.ToolMessage(fmt.Sprintf("Submission rejected: %s. Fix the arguments to match the %s tool schema exactly and call it again.", parseErr.Error(), submitAnalysisToolName), toolCall.ID))
					continue
				}
				if len(findings) == 0 {
					// Empty submission: the model declares the run finished.
					if blindErr := blindRunError(); blindErr != nil {
						return nil, tokensUsed, elapsed(), model, blindErr
					}
					return collected, tokensUsed, elapsed(), model, nil
				}

				// Whole-scope runs discard advice-only findings after the run
				// — validate at submission time instead, so the model can
				// repair them while the conversation is still alive.
				var rejected []string
				if toolCtx != nil && toolCtx.RequireActionableFindings {
					kept := make([]*AiResponse, 0, len(findings))
					for _, finding := range findings {
						if reason := ai.findingRejectionReason(finding); reason != "" {
							rejected = append(rejected, fmt.Sprintf("%s — %s", finding.ErrorMessage, reason))
						} else if !hasFindingHeadline(collected, finding.ErrorMessage) {
							kept = append(kept, finding)
						}
					}
					findings = kept
				}
				collected = append(collected, findings...)
				ai.logger.Info("Findings submitted", "new", len(findings), "rejected", len(rejected), "total", len(collected))
				if onProgress != nil {
					onProgress(tokensUsed, fmt.Sprintf("%d finding(s) submitted", len(collected)))
				}

				// After the nudge the investigation is over: a clean
				// submission ends the run, rejected findings get exactly one
				// repair round.
				if nudged && len(rejected) == 0 {
					return collected, tokensUsed, elapsed(), model, nil
				}
				if nudged && repairedOnce {
					return collected, tokensUsed, elapsed(), model, nil
				}
				if nudged {
					repairedOnce = true
					params.Messages = append(params.Messages, openai.ToolMessage(fmt.Sprintf("Recorded %d finding(s). Rejected %d finding(s) without an applicable proposal:\n- %s\nCall %s once more and resubmit ONLY the rejected findings, fixed (set proposedOperation and the exact live targetResource) — or an empty findings array to drop them.", len(findings), len(rejected), strings.Join(rejected, "\n- "), submitAnalysisToolName), toolCall.ID))
					continue
				}

				resultText := fmt.Sprintf("Recorded %d finding(s) — %d total so far. Continue the investigation and submit further findings, or call %s with an empty findings array when nothing else is actionable.", len(findings), len(collected), submitAnalysisToolName)
				if len(rejected) > 0 {
					resultText = fmt.Sprintf("Recorded %d finding(s) — %d total so far. Rejected %d finding(s) without an applicable proposal:\n- %s\nFix each rejected finding and resubmit it, or drop it if no safe concrete change exists.", len(findings), len(collected), len(rejected), strings.Join(rejected, "\n- "))
				}
				params.Messages = append(params.Messages, openai.ToolMessage(resultText, toolCall.ID))
				continue
			}

			var args map[string]any
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return nil, tokensUsed, elapsed(), model, fmt.Errorf("error unmarshaling tool arguments: %v", err)
			}

			if onProgress != nil {
				onProgress(tokensUsed, describeToolCall(name, args))
			}

			tool, ok := toolDefinitions[name]
			if !ok {
				return nil, tokensUsed, elapsed(), model, fmt.Errorf("unknown tool called: %s", name)
			}
			if !viewerAllowedTools[name] {
				return nil, tokensUsed, elapsed(), model, fmt.Errorf("tool %q is not permitted in the unattended insight pipeline", name)
			}
			data := tool(args, toolCtx, ai.valkeyClient, ai.logger)
			ai.auditInsightToolCall(toolCtx, name, args, data)
			inspectionCalls++
			params.Messages = append(params.Messages, openai.ToolMessage(data, toolCall.ID))
		}

		// Increase global tool call count and check the run budgets (tool
		// calls and, when configured, tokens). Either limit exhausted forces
		// a bounded number of final submit turns; findings collected so far
		// are kept.
		toolCallCount += len(message.ToolCalls)
		if nudged {
			turnsAfterNudge++
			if turnsAfterNudge >= 3 {
				ai.logger.Warn("Model kept going after the final-answer nudge, ending the run", "collected", len(collected))
				if blindErr := blindRunError(); blindErr != nil {
					return nil, tokensUsed, elapsed(), model, blindErr
				}
				return collected, tokensUsed, elapsed(), model, nil
			}
			continue
		}
		budgetExhausted := maxToolCalls > 0 && toolCallCount >= maxToolCalls
		if !budgetExhausted && maxTokensPerRun > 0 && tokensUsed >= maxTokensPerRun {
			budgetExhausted = true
			ai.logger.Info("Per-run token limit reached, forcing final answer", "maxTokensPerRun", maxTokensPerRun, "tokensUsed", tokensUsed)
		}
		if budgetExhausted {
			ai.logger.Info("Run budget exhausted, forcing final answer", "maxToolCalls", maxToolCalls, "toolCallCount", toolCallCount, "tokensUsed", tokensUsed)
			nudged = true
			params.Messages = append(params.Messages, openai.UserMessage(finalAnswerNudge))
			// Force the submit tool so the final turns cannot wander off into
			// further inspection (no Anthropic-style prompt-cache penalty for
			// changing tool_choice here).
			params.ToolChoice = openai.ToolChoiceOptionFunctionToolChoice(openai.ChatCompletionNamedToolChoiceFunctionParam{Name: submitAnalysisToolName})
		}

		// Continue the loop to get the next response with tool results
	}
}

func (ai *aiManager) openaiChat(
	ctx context.Context,
	ioChannel IOChatChannel,

	systemPrompt string,
	rc *ResolvedModelConfig,
) error {
	maxToolCalls := rc.MaxToolCalls
	client := ai.newOpenAIClientFor(rc)

	// Add system message at the beginning
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}

	// Build full tool set once per session (static + MCP, filtered by role)
	allChatTools := append(kubernetesOpenAiTools, helmOpenAiTools...)
	if !isViewerRole(ioChannel) {
		allChatTools = append(allChatTools, activateToolCategoriesOpenAi)
	}
	if ai.mcpManager != nil {
		allChatTools = append(allChatTools, ai.mcpManager.GetOpenAITools()...)
	}
	allChatTools = filterOpenAiTools(allChatTools, ioChannel)

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
			messages = append(messages, openai.UserMessage(userInput))

			// Process with tool call loop (categories + allChatTools passed so
			// the inner loop can recompute active tools after activation)
			fullResponse, updatedMessages, turnStats, err := ai.openaiChatWithTools(ctx, client, rc, messages, ioChannel, allChatTools, categories, maxToolCalls, &sessionInputTokens, &sessionOutputTokens)
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

// openaiChatWithTools handles the AI request with streaming and tool call support.
func (ai *aiManager) openaiChatWithTools(
	ctx context.Context,
	client *openai.Client,
	rc *ResolvedModelConfig,
	messages []openai.ChatCompletionMessageParamUnion,
	ioChannel IOChatChannel,
	allChatTools []openai.ChatCompletionToolUnionParam,
	categories *ActiveToolCategories,
	maxToolCalls int,
	sessionInputTokens *int64,
	sessionOutputTokens *int64,
) (fullResponse string, updatedMessages []openai.ChatCompletionMessageParamUnion, stats ChatTurnStats, err error) {
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
		chatTools := filterOpenAiToolsByCategory(allChatTools, categories)

		// Notify user that AI is thinking
		select {
		case ioChannel.Output <- "[AI is thinking...]\n":
		case <-ctx.Done():
			return "", messages, stats, ctx.Err()
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
					return "", messages, stats, ctx.Err()
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
						return "", messages, stats, ctx.Err()
					}
				}
				if tc.Function.Arguments != "" {
					entry.Arguments.WriteString(tc.Function.Arguments)
				}
			}
		}

		if err := stream.Err(); err != nil {
			return "", messages, stats, fmt.Errorf("streaming error: %w", err)
		}

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

		// Build the assistant message for conversation history
		if len(toolCallMap) == 0 {
			// No tool calls — just a text response
			response := fullText.String()
			messages = append(messages, openai.AssistantMessage(response))
			return response, messages, stats, nil
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
			// Replace the just-appended assistant tool_calls message with a
			// text-only one so messages[-1] is always a valid assistant text
			// message and never an unmatched tool_calls entry.
			text := fullText.String()
			if text == "" {
				text = "[Tool call limit reached]"
			}
			messages = messages[:len(messages)-1]
			messages = append(messages, openai.AssistantMessage(text))
			return text, messages, stats, nil
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
			var toolErr string
			if tc.Name == activateToolCategoriesName {
				result = categories.ActivateFromToolCall(args)
			} else if ai.mcpManager != nil && ai.mcpManager.IsMCPTool(tc.Name) {
				mcpResult, err := ai.mcpManager.CallTool(ctx, tc.Name, args)
				if err != nil {
					ai.logger.Error("MCP tool call failed", "tool", tc.Name, "error", err)
					toolErr = fmt.Sprintf("Error calling MCP tool: %v", err)
					messages = append(messages, openai.ToolMessage(toolErr, tc.ID))
					stats.ToolRecords = append(stats.ToolRecords, ToolUseRecord{Tool: tc.Name, Args: args, Error: toolErr})
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
			stats.ToolRecords = append(stats.ToolRecords, ToolUseRecord{Tool: tc.Name, Args: args, Result: truncateToolResult(result)})
			messages = append(messages, openai.ToolMessage(result, tc.ID))
		}

		// Continue loop to get response after tool results
	}
}
