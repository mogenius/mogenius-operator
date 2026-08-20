package aisdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"
)

const anthropicDefaultMaxTokens = 8192

// AnthropicProvider wraps the Anthropic SDK behind the Provider interface.
type AnthropicProvider struct {
	client *anthropic.Client
}

var _ Provider = (*AnthropicProvider)(nil)

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

// toAnthropicMessages splits messages into the Anthropic system block and
// conversation turns. System messages are collected into the System field;
// tool result messages are wrapped in a user turn as Anthropic requires.
func toAnthropicMessages(messages []Message) (system []anthropic.TextBlockParam, msgs []anthropic.MessageParam) {
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			system = append(system, anthropic.TextBlockParam{Type: "text", Text: m.Content})

		case RoleUser:
			msgs = append(msgs, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(m.Content)},
			})

		case RoleTool:
			// Tool results go in a user-role turn.
			msgs = append(msgs, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false)},
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
			msgs = append(msgs, anthropic.MessageParam{
				Role:    anthropic.MessageParamRoleAssistant,
				Content: content,
			})
		}
	}
	return
}

func (p *AnthropicProvider) Chat(ctx context.Context, model string, messages []Message) (Response, error) {
	system, msgs := toAnthropicMessages(messages)

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: anthropicDefaultMaxTokens,
		System:    system,
		Messages:  msgs,
	})
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

	return Response{
		Content:      text.String(),
		ToolCalls:    toolCalls,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		FinishReason: string(resp.StopReason),
	}, nil
}

func (p *AnthropicProvider) ChatStream(ctx context.Context, model string, messages []Message) (<-chan StreamChunk, error) {
	system, msgs := toAnthropicMessages(messages)

	stream := p.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: anthropicDefaultMaxTokens,
		System:    system,
		Messages:  msgs,
	})

	ch := make(chan StreamChunk, 16)
	go func() {
		defer close(ch)

		var inputTokens, outputTokens int64
		var finishReason string

		// tool_use blocks arrive across multiple events identified by Index.
		type toolAccum struct {
			id   string
			name string
			args strings.Builder
		}
		toolByIndex := map[int64]*toolAccum{}
		var toolOrder []int64 // insertion order of indices

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
				inputTokens = evt.Message.Usage.InputTokens

			case anthropic.MessageDeltaEvent:
				outputTokens = evt.Usage.OutputTokens
				finishReason = string(evt.Delta.StopReason)
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
