package aisdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"
)

const anthropicDefaultMaxTokens = 8192

// AnthropicProvider wraps the Anthropic SDK behind the Provider interface.
type AnthropicProvider struct {
	client             *anthropic.Client
	contextWindowCache sync.Map // model string → int64
}

var _ Provider = (*AnthropicProvider)(nil)
var _ CacheBreakpointMover = (*AnthropicProvider)(nil)

// NewAnthropicProvider creates a provider backed by the Anthropic API.
// baseURL may be empty to use the default endpoint.
func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
	opts := []anthropic_option.RequestOption{anthropic_option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, anthropic_option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(opts...)
	return &AnthropicProvider{client: &client}
}

func (p *AnthropicProvider) ContextWindowTokens(ctx context.Context, model string) (int64, error) {
	if v, ok := p.contextWindowCache.Load(model); ok {
		return v.(int64), nil
	}
	info, err := p.client.Models.Get(ctx, model, anthropic.ModelGetParams{})
	if err != nil {
		return 0, err
	}
	p.contextWindowCache.Store(model, info.MaxInputTokens)
	return info.MaxInputTokens, nil
}

// MoveCacheBreakpoint shifts the prompt-cache marker to the last message.
// The previous marker is cleared so Anthropic's 4-breakpoint cap is not
// exhausted (the system prompt and tool block each consume one slot).
func (p *AnthropicProvider) MoveCacheBreakpoint(messages []Message, idx *int) {
	if len(messages) == 0 {
		return
	}
	last := len(messages) - 1
	if *idx == last {
		return
	}
	if *idx >= 0 && *idx < last {
		messages[*idx].CacheControl = false
	}
	messages[last].CacheControl = true
	*idx = last
}

// toolsToAnthropic converts provider-neutral tools to Anthropic's format.
// The last tool receives cache_control so the entire tool block is cached.
func toolsToAnthropic(tools []Tool) []anthropic.ToolUnionParam {
	params := make([]anthropic.ToolUnionParam, len(tools))
	for i, t := range tools {
		props := t.InputSchema
		if props == nil {
			props = map[string]any{}
		}
		tp := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:       "object",
				Properties: props,
				Required:   t.Required,
			},
		}
		if i == len(tools)-1 {
			tp.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
		params[i] = anthropic.ToolUnionParam{OfTool: &tp}
	}
	return params
}

// toAnthropicMessages splits messages into the Anthropic system block and
// conversation turns. Messages with CacheControl:true get a cache_control
// ephemeral marker on their last content block.
func toAnthropicMessages(messages []Message) (system []anthropic.TextBlockParam, msgs []anthropic.MessageParam) {
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			block := anthropic.TextBlockParam{Type: "text", Text: m.Content}
			if m.CacheControl {
				block.CacheControl = anthropic.NewCacheControlEphemeralParam()
			}
			system = append(system, block)

		case RoleUser:
			textBlock := anthropic.NewTextBlock(m.Content)
			if m.CacheControl && textBlock.OfText != nil {
				textBlock.OfText.CacheControl = anthropic.NewCacheControlEphemeralParam()
			}
			msgs = append(msgs, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{textBlock},
			})

		case RoleTool:
			resultBlock := anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false)
			if m.CacheControl && resultBlock.OfToolResult != nil {
				resultBlock.OfToolResult.CacheControl = anthropic.NewCacheControlEphemeralParam()
			}
			msgs = append(msgs, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{resultBlock},
			})

		case RoleAssistant:
			content := make([]anthropic.ContentBlockParamUnion, 0, 1+len(m.ToolCalls))
			if m.Content != "" {
				content = append(content, anthropic.NewTextBlock(m.Content))
			}
			for _, tc := range m.ToolCalls {
				var input map[string]any
				_ = json.Unmarshal([]byte(tc.Arguments), &input)
				content = append(content, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{ID: tc.ID, Name: tc.Name, Input: input},
				})
			}
			if m.CacheControl && len(content) > 0 {
				last := &content[len(content)-1]
				switch {
				case last.OfText != nil:
					last.OfText.CacheControl = anthropic.NewCacheControlEphemeralParam()
				case last.OfToolUse != nil:
					last.OfToolUse.CacheControl = anthropic.NewCacheControlEphemeralParam()
				}
			}
			msgs = append(msgs, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleAssistant,
				Content: content,
			})
		}
	}
	return
}

func (p *AnthropicProvider) Chat(ctx context.Context, model string, messages []Message, tools []Tool) (Response, error) {
	system, msgs := toAnthropicMessages(messages)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: anthropicDefaultMaxTokens,
		System:    system,
		Messages:  msgs,
	}
	if len(tools) > 0 {
		params.Tools = toolsToAnthropic(tools)
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return Response{}, err
	}

	var text strings.Builder
	var toolCalls []ToolCall
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(block.Input),
			})
		}
	}

	finishReason := string(resp.StopReason)
	// Normalize Anthropic-specific stop reasons to the common "length" signal.
	if finishReason == "max_tokens" || finishReason == "model_context_window_exceeded" {
		finishReason = "length"
	}

	return Response{
		Content:         text.String(),
		ToolCalls:       toolCalls,
		InputTokens:     resp.Usage.InputTokens + resp.Usage.CacheCreationInputTokens,
		OutputTokens:    resp.Usage.OutputTokens,
		FinishReason:    finishReason,
		CacheReadTokens: resp.Usage.CacheReadInputTokens,
	}, nil
}

func (p *AnthropicProvider) ChatStream(ctx context.Context, model string, messages []Message, tools []Tool) (<-chan StreamChunk, error) {
	system, msgs := toAnthropicMessages(messages)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: anthropicDefaultMaxTokens,
		System:    system,
		Messages:  msgs,
	}
	if len(tools) > 0 {
		params.Tools = toolsToAnthropic(tools)
	}

	stream := p.client.Messages.NewStreaming(ctx, params)

	ch := make(chan StreamChunk, 16)
	go func() {
		defer close(ch)

		var inputTokens, outputTokens int64
		var finishReason string

		type toolAccum struct {
			id   string
			name string
			args strings.Builder
		}
		toolByIndex := map[int64]*toolAccum{}
		var toolOrder []int64

		for stream.Next() {
			event := stream.Current()

			switch evt := event.AsAny().(type) {
			case anthropic.ContentBlockStartEvent:
				if evt.ContentBlock.Type == "tool_use" {
					toolByIndex[evt.Index] = &toolAccum{
						id:   evt.ContentBlock.ID,
						name: evt.ContentBlock.Name,
					}
					toolOrder = append(toolOrder, evt.Index)
					ch <- StreamChunk{ToolCallName: evt.ContentBlock.Name}
				}

			case anthropic.ContentBlockDeltaEvent:
				switch evt.Delta.Type {
				case "text_delta":
					ch <- StreamChunk{Content: evt.Delta.Text}
				case "input_json_delta":
					if accum, ok := toolByIndex[evt.Index]; ok {
						accum.args.WriteString(evt.Delta.PartialJSON)
					}
				}

			case anthropic.MessageStartEvent:
				inputTokens = evt.Message.Usage.InputTokens + evt.Message.Usage.CacheCreationInputTokens

			case anthropic.MessageDeltaEvent:
				outputTokens = evt.Usage.OutputTokens
				finishReason = string(evt.Delta.StopReason)
				if finishReason == "max_tokens" || finishReason == "model_context_window_exceeded" {
					finishReason = "length"
				}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamChunk{Err: fmt.Errorf("streaming error: %w", err), Done: true}
			return
		}

		toolCalls := make([]ToolCall, 0, len(toolOrder))
		for _, idx := range toolOrder {
			accum := toolByIndex[idx]
			toolCalls = append(toolCalls, ToolCall{ID: accum.id, Name: accum.name, Arguments: accum.args.String()})
		}

		ch <- StreamChunk{
			Done:         true,
			ToolCalls:    toolCalls,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			FinishReason: finishReason,
		}
	}()

	return ch, nil
}
