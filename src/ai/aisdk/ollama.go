package aisdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ollama/ollama/api"
)

// OllamaProvider wraps the Ollama SDK behind the Provider interface.
type OllamaProvider struct {
	client *api.Client
}

var _ Provider = (*OllamaProvider)(nil)

// NewOllamaProvider creates a provider backed by a local Ollama instance.
// baseURL must point to the running Ollama server (e.g. "http://localhost:11434").
func NewOllamaProvider(baseURL string) (*OllamaProvider, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Ollama base URL: %w", err)
	}
	return &OllamaProvider{client: api.NewClient(u, http.DefaultClient)}, nil
}

func toOllamaMessages(messages []Message) []api.Message {
	result := make([]api.Message, 0, len(messages))
	for _, m := range messages {
		msg := api.Message{Role: string(m.Role), Content: m.Content}

		if m.Role == RoleTool {
			msg.ToolName = m.ToolName
		}

		if len(m.ToolCalls) > 0 {
			ollamaCalls := make([]api.ToolCall, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				var args api.ToolCallFunctionArguments
				_ = json.Unmarshal([]byte(tc.Arguments), &args)
				ollamaCalls[i] = api.ToolCall{
					Function: api.ToolCallFunction{Name: tc.Name, Arguments: args},
				}
			}
			msg.ToolCalls = ollamaCalls
		}

		result = append(result, msg)
	}
	return result
}

func ollamaToolCalls(calls []api.ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	result := make([]ToolCall, len(calls))
	for i, tc := range calls {
		argsBytes, _ := json.Marshal(tc.Function.Arguments)
		result[i] = ToolCall{Name: tc.Function.Name, Arguments: string(argsBytes)}
	}
	return result
}

func (p *OllamaProvider) Chat(ctx context.Context, model string, messages []Message) (Response, error) {
	falsePtr := false

	var content string
	var toolCalls []api.ToolCall
	var inputTokens, outputTokens int64

	err := p.client.Chat(ctx, &api.ChatRequest{
		Model:    model,
		Messages: toOllamaMessages(messages),
		Stream:   &falsePtr,
	}, func(resp api.ChatResponse) error {
		content += resp.Message.Content
		if resp.Done {
			toolCalls = resp.Message.ToolCalls
			inputTokens = int64(resp.PromptEvalCount)
			outputTokens = int64(resp.EvalCount)
		}
		return nil
	})
	if err != nil {
		return Response{}, err
	}

	return Response{
		Content:      content,
		ToolCalls:    ollamaToolCalls(toolCalls),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		FinishReason: "stop",
	}, nil
}

func (p *OllamaProvider) ChatStream(ctx context.Context, model string, messages []Message) (<-chan StreamChunk, error) {
	truePtr := true

	ch := make(chan StreamChunk, 16)
	go func() {
		defer close(ch)

		var inputTokens, outputTokens int64
		var lastToolCalls []api.ToolCall

		err := p.client.Chat(ctx, &api.ChatRequest{
			Model:    model,
			Messages: toOllamaMessages(messages),
			Stream:   &truePtr,
		}, func(resp api.ChatResponse) error {
			if resp.Message.Content != "" {
				select {
				case ch <- StreamChunk{Content: resp.Message.Content}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			// Ollama delivers tool calls in intermediate chunks before Done.
			for _, tc := range resp.Message.ToolCalls {
				select {
				case ch <- StreamChunk{ToolCallName: tc.Function.Name}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if len(resp.Message.ToolCalls) > 0 {
				lastToolCalls = append(lastToolCalls, resp.Message.ToolCalls...)
			}

			if resp.Done {
				inputTokens = int64(resp.PromptEvalCount)
				outputTokens = int64(resp.EvalCount)
			}
			return nil
		})

		if err != nil {
			ch <- StreamChunk{Err: fmt.Errorf("streaming error: %w", err), Done: true}
			return
		}

		ch <- StreamChunk{
			Done:         true,
			ToolCalls:    ollamaToolCalls(lastToolCalls),
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			FinishReason: "stop",
		}
	}()

	return ch, nil
}
