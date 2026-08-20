package aisdk

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Tool is a provider-neutral tool definition.
type Tool struct {
	Name        string
	Description string
	// InputSchema is the properties map (JSON-schema format): {"propName": {"type": "string", "description": "..."}}
	InputSchema map[string]any
	Required    []string
}

// Message is a provider-agnostic chat message.
type Message struct {
	Role    Role
	Content string
	// ToolCallID and ToolName are populated for RoleTool messages.
	ToolCallID string
	ToolName   string
	// ToolCalls is populated for RoleAssistant messages that request tool execution.
	ToolCalls []ToolCall
	// CacheControl marks this message for prompt caching. Used by the Anthropic
	// provider only; ignored by other providers.
	CacheControl bool
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
	// CacheReadTokens is populated by the Anthropic provider for prompt-cache
	// reads; these do not count against the regular token budget.
	CacheReadTokens int64
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

// CacheBreakpointMover is an optional interface for providers that support
// moving prompt-cache breakpoints on individual conversation messages.
// Currently implemented by AnthropicProvider only.
type CacheBreakpointMover interface {
	MoveCacheBreakpoint(messages []Message, idx *int)
}

// Provider is a provider-agnostic interface for LLM chat calls.
type Provider interface {
	// Chat sends messages (and optional tools) and returns the full response.
	Chat(ctx context.Context, model string, messages []Message, tools []Tool) (Response, error)
	// ChatStream sends messages (and optional tools) and returns a channel of chunks.
	// The channel is closed after the Done chunk. The caller must drain it even when cancelling via ctx.
	ChatStream(ctx context.Context, model string, messages []Message, tools []Tool) (<-chan StreamChunk, error)
}
