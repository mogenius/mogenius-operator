package ai

import (
	"context"
	"fmt"
	"log/slog"
)

type toolExec struct {
	ToolCtx           *ToolContext
	Categories        *ActiveToolCategories
	McpSessions       []string
	ScopedMCP         bool
	InterceptMutating bool
	UnknownIsFatal    bool
	Audit             bool
	RecordStep        StepRecorder
	Stats             *ChatTurnStats
}

type toolOutcome struct {
	Result  string
	IsError bool
	Fatal   error
}

// dispatchToolCall resolves and executes a single tool call by name, handling
// the built-in / MCP / approval
func (ai *aiManager) dispatchToolCall(ctx context.Context, name string, args map[string]any, rawArgs string, e toolExec) toolOutcome {
	// Meta-tool: activate tool categories (chat only).
	if e.Categories != nil && name == activateToolCategoriesName {
		result := e.Categories.ActivateFromToolCall(args)
		e.recordStat(name, args, result, "")
		return toolOutcome{Result: result}
	}

	// Built-in Kubernetes/Helm tools.
	if tool, ok := toolDefinitions[name]; ok {
		execCtx := e.ToolCtx
		var result string
		switch {
		case e.InterceptMutating && mutatingBuiltinTools[name] && e.ToolCtx != nil && e.ToolCtx.CreateApprovalRequest != nil:
			approverCtx, approvalErr := e.ToolCtx.CreateApprovalRequest(ctx, name, args)
			if approvalErr != nil {
				result = fmt.Sprintf("Tool call %q was not executed: %v", name, approvalErr)
			} else {
				execCtx = approverCtx
				result = tool(args, execCtx, ai.valkeyClient, ai.logger)
			}
		case e.InterceptMutating && mutatingBuiltinTools[name]:
			result = fmt.Sprintf("Tool call %q requires human approval but no approval mechanism is configured for this run. The call was blocked.", name)
		default:
			result = tool(args, execCtx, ai.valkeyClient, ai.logger)
		}
		if e.Audit {
			ai.auditInsightToolCall(execCtx, name, args, result, nil)
		}
		e.recordStep(name, args, rawArgs, result)
		e.recordStat(name, args, result, "")
		return toolOutcome{Result: result}
	}

	// MCP tools.
	if e.ScopedMCP {
		if ai.mcpManager != nil && ai.mcpManager.IsMCPToolInSessions(name, e.McpSessions) {
			auditCtx := e.ToolCtx
			var data string
			if ai.mcpManager.MCPToolNeedsApproval(name, e.McpSessions) {
				approverCtx, approvalErr := e.ToolCtx.CreateApprovalRequest(ctx, name, args)
				if approvalErr != nil {
					data = fmt.Sprintf("Tool call %q was not executed: %v", name, approvalErr)
				} else {
					auditCtx = approverCtx
				}
			}
			var mcpErr error
			if data == "" {
				var mcpResult string
				mcpResult, mcpErr = ai.mcpManager.CallToolInSessions(ctx, name, args, e.McpSessions)
				if mcpErr != nil {
					data = fmt.Sprintf("MCP tool error: %v", mcpErr)
				} else {
					data = mcpResult
				}
			}
			if e.Audit {
				ai.auditInsightToolCall(auditCtx, name, args, data, mcpErr)
			}
			e.recordStep(name, args, rawArgs, data)
			return toolOutcome{Result: data}
		}
	} else if ai.mcpManager != nil && ai.mcpManager.IsMCPTool(name) {
		mcpResult, err := ai.mcpManager.CallTool(ctx, name, args)
		if err != nil {
			ai.logger.Error("MCP tool call failed", "tool", name, "error", err)
			toolErr := fmt.Sprintf("Error calling MCP tool: %v", err)
			e.recordStat(name, args, "", toolErr)
			return toolOutcome{Result: toolErr, IsError: true}
		}
		e.recordStat(name, args, mcpResult, "")
		return toolOutcome{Result: mcpResult}
	}

	// Unknown tool.
	if e.UnknownIsFatal {
		return toolOutcome{Fatal: fmt.Errorf("unknown tool called: %s", name)}
	}
	ai.logger.Error("Unknown tool called", "tool", name)
	return toolOutcome{Result: fmt.Sprintf("Unknown tool: %s", name), IsError: true}
}

// recordStep records a ReAct ACT step when a recorder is configured (agent).
func (e toolExec) recordStep(name string, args map[string]any, rawArgs, result string) {
	if e.RecordStep == nil {
		return
	}
	e.RecordStep(AiRunStep{
		Kind:   AI_RUN_STEP_ACT,
		Label:  describeToolCall(name, args),
		Tool:   name,
		Args:   rawArgs,
		Result: result,
	})
}

// recordStat appends a ToolUseRecord when a stats sink is configured (chat).
func (e toolExec) recordStat(name string, args map[string]any, result, errStr string) {
	if e.Stats == nil {
		return
	}
	rec := ToolUseRecord{Tool: name, Args: args}
	if errStr != "" {
		rec.Error = errStr
	} else {
		rec.Result = truncateToolResult(result)
	}
	e.Stats.ToolRecords = append(e.Stats.ToolRecords, rec)
}

// BudgetExhaustedError is returned by processPrompt* when a run hits its
// per-run tool-call or token budget. It is not a failure — callers treat it
// as a soft completion with a user-visible note explaining why the run stopped.
type BudgetExhaustedError struct {
	Msg string
}

func (e *BudgetExhaustedError) Error() string { return e.Msg }

// runBudgetExhausted returns a non-empty reason string when the run has hit
// its tool-call or per-run token budget. Empty string means the run may
// continue. Callers pass the already-incremented tool-call count.
func runBudgetExhausted(logger *slog.Logger, maxToolCalls, toolCallCount int, maxTokensPerRun, tokensUsed int64) string {
	if maxToolCalls > 0 && toolCallCount >= maxToolCalls {
		msg := fmt.Sprintf("run stopped after %d tool calls (per-run tool call limit: %d)", toolCallCount, maxToolCalls)
		logger.Info("Run budget exhausted: tool call limit", "maxToolCalls", maxToolCalls, "toolCallCount", toolCallCount)
		return msg
	}
	if maxTokensPerRun > 0 && tokensUsed >= maxTokensPerRun {
		msg := fmt.Sprintf("run stopped after %d tokens (per-run token limit: %d)", tokensUsed, maxTokensPerRun)
		logger.Info("Run budget exhausted: token limit", "maxTokensPerRun", maxTokensPerRun, "tokensUsed", tokensUsed)
		return msg
	}
	return ""
}
