package ai

import (
	"context"
	"log/slog"
	"testing"

	"mogenius-operator/src/valkeyclient"

	"github.com/stretchr/testify/assert"
)

// TestDispatchToolCall_NilRecordStep_NoPanic guards against the MOG-4739
// regression: the chat path (runChatTurn) builds its toolExec without a
// StepRecorder, so e.RecordStep is a nil interface. dispatchToolCall calls
// RecordStep.ToolCall for every built-in tool, which crashed the operator with
// a nil-pointer panic on every chat tool call. dispatchToolCall must normalize
// a nil recorder to the no-op recorder (honoring the StepRecorder contract).
func TestDispatchToolCall_NilRecordStep_NoPanic(t *testing.T) {
	const toolName = "test_fake_tool_mog4739"

	// Register a minimal built-in tool that needs no cluster/valkey infra.
	toolDefinitions[toolName] = func(map[string]any, *ToolContext, valkeyclient.ValkeyClient, *slog.Logger) string {
		return "fake-result"
	}
	defer delete(toolDefinitions, toolName)

	ai := &aiManager{}

	// Exactly how runChatTurn builds it: no RecordStep set -> nil interface.
	exec := toolExec{}
	assert.Nil(t, exec.RecordStep, "precondition: chat toolExec has a nil recorder")

	assert.NotPanics(t, func() {
		out := ai.dispatchToolCall(context.Background(), toolName, map[string]any{}, "", exec)
		assert.Equal(t, "fake-result", out.Result)
		assert.False(t, out.IsError)
	})
}
