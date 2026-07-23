package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
)

// processPromptOllama runs the unattended agent-run protocol against an
// Ollama endpoint, mirroring the Anthropic path: strictly read-only
// inspection tools plus the repeatable submit_analysis tool through which
// all findings arrive. Local models are less reliable tool callers than the
// hosted providers, so protocol violations (hallucinated tool names, schema
// misses, prose answers) are fed back as in-conversation corrections —
// bounded by the run budget — instead of failing the whole run.
func (ai *aiManager) processPromptOllama(ctx context.Context, rc *ResolvedModelConfig, systemPrompt, prompt string, toolCtx *ToolContext, onProgress func(int64, string), recordStep StepRecorder) ([]*AiResponse, int64, int, string, error) {

	startTime := time.Now()
	elapsed := func() int { return int(time.Since(startTime).Milliseconds()) }

	model := rc.Model
	maxToolCalls := rc.MaxToolCalls
	maxTokensPerRun := rc.MaxTokensPerRun

	client, err := ai.newOllamaClientFor(rc)
	if err != nil {
		return nil, 0, elapsed(), model, err
	}

	// Unattended pipeline: strictly read-only tools (defense in depth on top
	// of the ToolContext namespace scoping) plus submit_analysis, so findings
	// arrive as structured tool input instead of JSON scraped out of text.
	tools := readOnlyOllamaTools(append(kubernetesOllamaTools, helmOllamaTools...))
	tools = append(tools, submitAnalysisOllamaTool)

	messages := []api.Message{
		{Role: "system", Content: systemPrompt + submitAnalysisInstruction},
		{Role: "user", Content: prompt},
	}

	falsePtr := false
	truePtr := true
	var tokensUsed int64 = 0
	toolCallCount := 0

	// Successful read-only inspections. A whole-scope run that ends without a
	// single one produced its answer blind — a model failure, not an all-clear.
	inspectionCalls := 0

	// Bounded in-conversation repair turns for schema-violating submissions.
	repairAttempts := 0

	// Set once the run budget is exhausted; from then on inspection tools are
	// refused and only a bounded number of turns remain to submit.
	nudged := false
	turnsAfterNudge := 0

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

	toolResult := func(name, content string) api.Message {
		return api.Message{Role: "tool", ToolName: name, Content: content}
	}

	for {
		// Deliberately no Format:"json": forcing JSON from the first turn
		// suppresses tool calls — the model answers immediately instead of
		// inspecting anything. The structured result arrives through
		// submit_analysis; free-text stragglers go through parseAiResponse.
		req := &api.ChatRequest{
			Model:    model,
			Messages: messages,
			Stream:   &falsePtr,
			Truncate: &truePtr,
			Shift:    &truePtr,
			Tools:    tools,
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
			if len(collected) > 0 {
				// Salvage what the run already confirmed instead of throwing
				// the whole exploration away.
				ai.logger.Warn("LLM turn failed mid-run, keeping findings collected so far", "collected", len(collected), "error", err)
				return collected, tokensUsed, elapsed(), model, nil
			}
			return nil, tokensUsed, elapsed(), model, err
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
			aiResponse, removedText, parseErr := parseAiResponse(responseText)
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
				return nil, tokensUsed, elapsed(), model, fmt.Errorf("final answer unparsable after %d repair attempts: %v\n%s", repairAttempts, parseErr, responseText)
			}
			messages = append(messages, api.Message{
				Role:    "user",
				Content: fmt.Sprintf("Your answer could not be processed: %s. Submit your findings by calling the %s tool (or call it with an empty findings array if nothing is actionable).", parseErr.Error(), submitAnalysisToolName),
			})
			continue
		}

		// Process each tool call
		for _, toolCall := range toolCalls {
			name := toolCall.Function.Name
			ai.logger.Info("Processing tool call", "tool", name)

			argsBytes, marshalErr := json.Marshal(toolCall.Function.Arguments)
			if marshalErr != nil {
				messages = append(messages, toolResult(name, fmt.Sprintf("Error marshaling tool arguments: %v", marshalErr)))
				continue
			}

			// The final analysis arrives as tool input; a schema violation is
			// fed back as a tool result so the model repairs it
			// in-conversation instead of failing the whole run.
			if name == submitAnalysisToolName {
				findings, parseErr := parseSubmittedAnalysis(argsBytes)
				if parseErr != nil {
					repairAttempts++
					ai.logger.Warn("Submitted findings rejected", "error", parseErr, "attempt", repairAttempts)
					if repairAttempts > maxAnalysisRepairs {
						if len(collected) > 0 {
							return collected, tokensUsed, elapsed(), model, nil
						}
						return nil, tokensUsed, elapsed(), model, fmt.Errorf("analysis rejected %d times, giving up: %w", repairAttempts, parseErr)
					}
					messages = append(messages, toolResult(name, fmt.Sprintf("Submission rejected: %s. Fix the arguments to match the %s tool schema exactly and call it again.", parseErr.Error(), submitAnalysisToolName)))
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
				if recordStep != nil {
					recordStep(AiRunStep{Kind: AI_RUN_STEP_FINDINGS, Label: fmt.Sprintf("%d finding(s) submitted — %d total", len(findings), len(collected)), Tool: submitAnalysisToolName})
				}
				resultText := fmt.Sprintf("Recorded %d finding(s) — %d total so far. Continue the investigation and submit further findings, or call %s with an empty findings array when nothing else is actionable.", len(findings), len(collected), submitAnalysisToolName)
				if len(rejected) > 0 {
					resultText = fmt.Sprintf("Recorded %d finding(s) — %d total so far. Rejected %d finding(s) without an applicable proposal:\n- %s\nFix each rejected finding and resubmit it, or drop it if no safe concrete change exists.", len(findings), len(collected), len(rejected), strings.Join(rejected, "\n- "))
				}
				messages = append(messages, toolResult(name, resultText))
				continue
			}

			tool, ok := toolDefinitions[name]
			if !ok {
				messages = append(messages, toolResult(name, fmt.Sprintf("Unknown tool %q — only the tools offered in this conversation exist. Continue with those, or call %s to finish.", name, submitAnalysisToolName)))
				continue
			}
			if !viewerAllowedTools[name] {
				messages = append(messages, toolResult(name, fmt.Sprintf("Tool %q is not permitted in this unattended run — only read-only inspection tools and %s are available.", name, submitAnalysisToolName)))
				continue
			}
			if nudged {
				messages = append(messages, toolResult(name, "Budget exhausted — no more inspection tools. Call "+submitAnalysisToolName+" now with your remaining findings (or an empty findings array)."))
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
			data := tool(args, toolCtx, ai.valkeyClient, ai.logger)
			ai.auditInsightToolCall(toolCtx, name, args, data)
			if recordStep != nil {
				recordStep(AiRunStep{Kind: AI_RUN_STEP_ACT, Label: describeToolCall(name, args), Tool: name, Args: string(argsBytes), Result: data})
			}
			inspectionCalls++
			messages = append(messages, toolResult(name, data))
		}

		// Increase global tool call count and check the run budgets (tool
		// calls and, when configured, tokens). Either limit exhausted forces
		// a bounded number of final submit turns; findings collected so far
		// are kept.
		toolCallCount += len(toolCalls)
		if nudged {
			turnsAfterNudge++
			if turnsAfterNudge >= 3 {
				ai.logger.Warn("Model kept requesting tools after the final-answer nudge, ending the run", "collected", len(collected))
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
			messages = append(messages, api.Message{Role: "user", Content: finalAnswerNudge})
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

			if resp.Done {
				inputTokens = int64(resp.PromptEvalCount)
				outputTokenCount = int64(resp.EvalCount)
				*sessionInputTokens += inputTokens
				*sessionOutputTokens += outputTokenCount
				inputTokensUsed += inputTokens
				outputTokensUsed += outputTokenCount
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
