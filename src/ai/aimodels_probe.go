package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mogenius-operator/src/store"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/openai/openai-go/v3"

	"github.com/ollama/ollama/api"
)

// aiModelProbeTimeout bounds one connectivity test; slower than this counts
// as failed so the UI never hangs on an unreachable endpoint.
const aiModelProbeTimeout = 30 * time.Second

// aiModelProbePrompt keeps the test as cheap as possible — a single short
// turn, no tools, no system prompt.
const aiModelProbePrompt = "Reply with the single word: OK"

// aiModelProbeMaxReplyChars caps the reply echoed back to the UI.
const aiModelProbeMaxReplyChars = 200

// AiModelTestResult reports the outcome of a connectivity probe against one
// AiModel: whether the provider answered, how long it took and either the
// reply snippet or the provider error. Probe failures are results, not
// errors — the UI renders both the same way.
type AiModelTestResult struct {
	Success    bool   `json:"success"`
	Model      string `json:"model"`
	Sdk        string `json:"sdk"`
	DurationMs int64  `json:"durationMs"`
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
}

// TestAiModel resolves the named AiModel (including its API key Secret) and
// sends a minimal single-turn prompt to the provider. An error return means
// the request itself was invalid (unknown model); provider/config problems
// come back as an unsuccessful result.
func (ai *aiManager) TestAiModel(name string) (*AiModelTestResult, error) {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve own namespace: %w", err)
	}
	model, err := store.GetAiModel(ownNamespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get AiModel %q: %w", name, err)
	}
	if model == nil {
		return nil, fmt.Errorf("AiModel %q not found in namespace %q", name, ownNamespace)
	}

	result := &AiModelTestResult{Model: model.Spec.Model, Sdk: model.Spec.Sdk}

	rc, err := ai.resolveAiModel(model, "test")
	if err != nil {
		// Unresolvable config (invalid spec, missing Secret/key) is exactly
		// what the test is for — report it as the outcome.
		result.Error = err.Error()
		return result, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), aiModelProbeTimeout)
	defer cancel()

	start := time.Now()
	reply, err := ai.probeModel(ctx, rc)
	result.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		ai.logger.Info("AiModel test failed", "aimodel", name, "sdk", result.Sdk, "model", result.Model, "error", err)
		return result, nil
	}

	result.Success = true
	result.Message = strings.TrimSpace(reply)
	if len(result.Message) > aiModelProbeMaxReplyChars {
		result.Message = result.Message[:aiModelProbeMaxReplyChars] + "…"
	}
	ai.logger.Info("AiModel test succeeded", "aimodel", name, "sdk", result.Sdk, "model", result.Model, "durationMs", result.DurationMs)
	return result, nil
}

// probeModel sends the probe prompt through the provider matching the
// resolved config and returns the model's raw text reply.
func (ai *aiManager) probeModel(ctx context.Context, rc *ResolvedModelConfig) (string, error) {
	switch rc.Sdk {
	case AiSdkTypeAnthropic:
		client := ai.newAnthropicClientFor(rc)
		message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(rc.Model),
			MaxTokens: int64(16),
			Messages: []anthropic.MessageParam{
				{
					Role:    anthropic.MessageParamRoleUser,
					Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(aiModelProbePrompt)},
				},
			},
		})
		if err != nil {
			return "", err
		}
		for _, block := range message.Content {
			if block.Type == "text" && block.Text != "" {
				return block.Text, nil
			}
		}
		return "", fmt.Errorf("provider returned no text content")

	case AiSdkTypeOpenAI:
		client := ai.newOpenAIClientFor(rc)
		completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model: rc.Model,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(aiModelProbePrompt),
			},
		})
		if err != nil {
			return "", err
		}
		if len(completion.Choices) == 0 {
			return "", fmt.Errorf("provider returned no choices")
		}
		return completion.Choices[0].Message.Content, nil

	case AiSdkTypeOllama:
		client, err := ai.newOllamaClientFor(rc)
		if err != nil {
			return "", err
		}
		stream := false
		var reply strings.Builder
		err = client.Chat(ctx, &api.ChatRequest{
			Model:    rc.Model,
			Stream:   &stream,
			Messages: []api.Message{{Role: "user", Content: aiModelProbePrompt}},
		}, func(resp api.ChatResponse) error {
			reply.WriteString(resp.Message.Content)
			return nil
		})
		if err != nil {
			return "", err
		}
		return reply.String(), nil

	default:
		return "", fmt.Errorf("unsupported AI SDK type: %s", rc.Sdk)
	}
}
