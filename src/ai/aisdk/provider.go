package aisdk

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a provider-agnostic chat message.
type Message struct {
	Role    Role
	Content string
	// ToolCallID and ToolName are populated for RoleTool messages.
	ToolCallID string
	ToolName   string
	// ToolCalls is populated for RoleAssistant messages that request tool execution.
	ToolCalls []ToolCall
}

// ToolCall is a tool invocation requested by the model in an assistant message.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // JSON-encoded arguments
}

// Response is the result of a non-streaming chat call.
type Response struct {
	Content      string
	ToolCalls    []ToolCall
	InputTokens  int64
	OutputTokens int64
	FinishReason string
}

// StreamChunk is one event delivered during a streaming chat call.
// Done is true on the final chunk; Err carries any terminal error.
// ToolCalls is populated on the Done chunk with all tool calls requested during the turn.
type StreamChunk struct {
	Content      string
	ToolCallName string     // set on the first chunk that opens a tool call
	ToolCalls    []ToolCall // populated on the Done chunk
	InputTokens  int64
	OutputTokens int64
	Done         bool
	FinishReason string
	Err          error
}

// Provider is a provider-agnostic interface for LLM chat calls.
type Provider interface {
	// Chat sends messages and waits for the full response.
	Chat(ctx context.Context, model string, messages []Message) (Response, error)
	// ChatStream sends messages and returns a channel of chunks.
	// The channel is closed after the Done chunk is delivered.
	// The caller must drain the channel even when cancelling via ctx.
	ChatStream(ctx context.Context, model string, messages []Message) (<-chan StreamChunk, error)
}
