package aisdk

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIProvider wraps the OpenAI SDK behind the Provider interface.
type OpenAIProvider struct {
	client *openai.Client
}

var _ Provider = (*OpenAIProvider)(nil)

// NewOpenAIProvider creates a provider backed by the OpenAI API.
// baseURL may be empty to use the default endpoint.
func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	return &OpenAIProvider{client: &client}
}

func toolsToOpenAI(tools []Tool) []openai.ChatCompletionToolUnionParam {
	params := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, t := range tools {
		fnParams := openai.FunctionParameters{
			"type":       "object",
			"properties": t.InputSchema,
		}
		if len(t.Required) > 0 {
			fnParams["required"] = t.Required
		}
		params[i] = openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: openai.FunctionDefinitionParam{
					Name:        t.Name,
					Description: openai.String(t.Description),
					Parameters:  fnParams,
				},
			},
		}
	}
	return params
}

func toOpenAIMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			result = append(result, openai.SystemMessage(m.Content))

		case RoleUser:
			result = append(result, openai.UserMessage(m.Content))

		case RoleTool:
			result = append(result, openai.ToolMessage(m.Content, m.ToolCallID))

		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				paramCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					paramCalls[i] = openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: tc.Arguments,
							},
						},
					}
				}
				result = append(result, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &openai.ChatCompletionAssistantMessageParam{
						Content: openai.ChatCompletionAssistantMessageParamContentUnion{
							OfString: openai.String(m.Content),
						},
						ToolCalls: paramCalls,
					},
				})
			} else {
				result = append(result, openai.AssistantMessage(m.Content))
			}
		}
	}
	return result
}

func (p *OpenAIProvider) Chat(ctx context.Context, model string, messages []Message, tools []Tool) (Response, error) {
	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: toOpenAIMessages(messages),
	}
	if len(tools) > 0 {
		params.Tools = toolsToOpenAI(tools)
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return Response{}, err
	}
	if len(resp.Choices) == 0 {
		return Response{}, fmt.Errorf("no choices returned from model")
	}

	msg := resp.Choices[0].Message
	var toolCalls []ToolCall
	for _, tc := range msg.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	return Response{
		Content:      msg.Content,
		ToolCalls:    toolCalls,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		FinishReason: string(resp.Choices[0].FinishReason),
	}, nil
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, model string, messages []Message, tools []Tool) (<-chan StreamChunk, error) {
	params := openai.ChatCompletionNewParams{
		Model:    model,
		Messages: toOpenAIMessages(messages),
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		},
	}
	if len(tools) > 0 {
		params.Tools = toolsToOpenAI(tools)
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)

	ch := make(chan StreamChunk, 16)
	go func() {
		defer close(ch)

		var inputTokens, outputTokens int64
		var finishReason string
		type accumEntry struct {
			id   string
			name string
			args strings.Builder
		}
		toolMap := map[int64]*accumEntry{}

		for stream.Next() {
			chunk := stream.Current()

			if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
				inputTokens = chunk.Usage.PromptTokens
				outputTokens = chunk.Usage.CompletionTokens
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta
			if fr := string(chunk.Choices[0].FinishReason); fr != "" {
				finishReason = fr
			}

			if delta.Content != "" {
				ch <- StreamChunk{Content: delta.Content}
			}

			for _, tc := range delta.ToolCalls {
				entry, ok := toolMap[tc.Index]
				if !ok {
					entry = &accumEntry{}
					toolMap[tc.Index] = entry
				}
				if tc.ID != "" {
					entry.id = tc.ID
				}
				if tc.Function.Name != "" {
					entry.name = tc.Function.Name
					ch <- StreamChunk{ToolCallName: tc.Function.Name}
				}
				entry.args.WriteString(tc.Function.Arguments)
			}
		}

		if err := stream.Err(); err != nil {
			ch <- StreamChunk{Err: fmt.Errorf("streaming error: %w", err), Done: true}
			return
		}

		toolCalls := make([]ToolCall, 0, len(toolMap))
		for i := int64(0); ; i++ {
			entry, ok := toolMap[i]
			if !ok {
				break
			}
			toolCalls = append(toolCalls, ToolCall{ID: entry.id, Name: entry.name, Arguments: entry.args.String()})
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
