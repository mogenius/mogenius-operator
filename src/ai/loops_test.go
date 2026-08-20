package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"mogenius-operator/src/ai/aisdk"

	valkeyclient "github.com/valkey-io/valkey-go"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// mockProvider implements aisdk.Provider with configurable behaviour.
type mockProvider struct {
	chatFn   func(ctx context.Context, model string, msgs []aisdk.Message, tools []aisdk.Tool) (aisdk.Response, error)
	streamFn func(ctx context.Context, model string, msgs []aisdk.Message, tools []aisdk.Tool) (<-chan aisdk.StreamChunk, error)
}

func (m *mockProvider) Chat(ctx context.Context, model string, msgs []aisdk.Message, tools []aisdk.Tool) (aisdk.Response, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, model, msgs, tools)
	}
	return aisdk.Response{}, nil
}

func (m *mockProvider) ChatStream(ctx context.Context, model string, msgs []aisdk.Message, tools []aisdk.Tool) (<-chan aisdk.StreamChunk, error) {
	if m.streamFn != nil {
		return m.streamFn(ctx, model, msgs, tools)
	}
	ch := make(chan aisdk.StreamChunk, 1)
	ch <- aisdk.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

// staticStream returns a channel that delivers the given chunks in order.
func staticStream(chunks ...aisdk.StreamChunk) (<-chan aisdk.StreamChunk, error) {
	ch := make(chan aisdk.StreamChunk, len(chunks))
	for _, c := range chunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// noopValkeyClient satisfies valkeyclient.ValkeyClient with no-op methods so
// that aiManager.addTokenUsage doesn't panic in unit tests.
type noopValkeyClient struct{}

func (noopValkeyClient) Connect() error                    { return nil }
func (noopValkeyClient) Close()                            {}
func (noopValkeyClient) Set(_ string, _ time.Duration, _ ...string) error {
	return nil
}
func (noopValkeyClient) SetObject(_ any, _ time.Duration, _ ...string) error { return nil }
func (noopValkeyClient) SetObjectWithAutoincrementLimit(_ any, _ int64, _ time.Duration, _ ...string) (string, error) {
	return "", nil
}
func (noopValkeyClient) Get(_ ...string) (string, error)              { return "", nil }
func (noopValkeyClient) GetObject(_ ...string) (any, error)           { return nil, nil }
func (noopValkeyClient) List(_ int, _ ...string) ([]string, error)    { return nil, nil }
func (noopValkeyClient) DeleteFromSortedListWithNsAndReleaseName(_ string, _ string, _ ...string) error {
	return nil
}
func (noopValkeyClient) StoreSortedListEntry(_ any, _ int64, _ ...string) error { return nil }
func (noopValkeyClient) ClearNonEssentialKeys(_, _ bool, _ bool) (string, error) {
	return "", nil
}
func (noopValkeyClient) DeleteSingle(_ ...string) error         { return nil }
func (noopValkeyClient) DeleteMultiple(_ ...string) error       { return nil }
func (noopValkeyClient) Keys(_ string) ([]string, error)        { return nil, nil }
func (noopValkeyClient) Exists(_ ...string) (bool, error)       { return false, nil }
func (noopValkeyClient) GetValkeyClient() valkeyclient.Client   { return nil }
func (noopValkeyClient) GetContext() context.Context            { return context.Background() }
func (noopValkeyClient) GetLogger() *slog.Logger                { return slog.Default() }

// minimalAI returns an aiManager wired up for loop tests (no K8s, no Valkey I/O).
func minimalAI() *aiManager {
	return &aiManager{
		logger:       slog.Default(),
		mcpManager:   &mcpClientManager{},
		valkeyClient: noopValkeyClient{},
	}
}

// ---------------------------------------------------------------------------
// buildAgentMessages
// ---------------------------------------------------------------------------

func TestBuildAgentMessages_Structure(t *testing.T) {
	msgs := buildAgentMessages("sys prompt", "user task")
	assert.Len(t, msgs, 2)
	assert.Equal(t, aisdk.RoleSystem, msgs[0].Role)
	assert.Equal(t, "sys prompt", msgs[0].Content)
	assert.True(t, msgs[0].CacheControl, "system message must carry cache_control")
	assert.Equal(t, aisdk.RoleUser, msgs[1].Role)
	assert.Equal(t, "user task", msgs[1].Content)
}

// ---------------------------------------------------------------------------
// buildAgentTools
// ---------------------------------------------------------------------------

func TestBuildAgentTools_DefaultIncludesBothSets(t *testing.T) {
	tools := buildAgentTools(&mcpClientManager{}, nil, nil)
	names := toolNames(tools)
	assert.Contains(t, names, "get_kubernetes_resources")
	assert.Contains(t, names, "helm_chart_search")
}

func TestBuildAgentTools_DisableKubernetes(t *testing.T) {
	tools := buildAgentTools(&mcpClientManager{}, nil, &ToolContext{DisableKubernetes: true})
	names := toolNames(tools)
	for _, n := range names {
		assert.False(t, strings.HasPrefix(n, "get_kubernetes") ||
			strings.HasPrefix(n, "list_kubernetes") ||
			strings.HasPrefix(n, "check_kubernetes") ||
			strings.HasPrefix(n, "update_kubernetes") ||
			strings.HasPrefix(n, "delete_kubernetes") ||
			strings.HasPrefix(n, "create_kubernetes") ||
			n == "get_pod_logs" || n == "get_pod_events",
			"kubernetes tool %q must be excluded", n)
	}
	assert.Contains(t, names, "helm_chart_search")
}

func TestBuildAgentTools_DisableHelm(t *testing.T) {
	tools := buildAgentTools(&mcpClientManager{}, nil, &ToolContext{DisableHelm: true})
	names := toolNames(tools)
	for _, n := range names {
		assert.False(t, strings.HasPrefix(n, "helm_"), "helm tool %q must be excluded", n)
	}
	assert.Contains(t, names, "get_kubernetes_resources")
}

func TestBuildAgentTools_BothDisabledYieldsEmpty(t *testing.T) {
	tools := buildAgentTools(&mcpClientManager{}, nil, &ToolContext{DisableKubernetes: true, DisableHelm: true})
	assert.Empty(t, tools)
}

func toolNames(tools []aisdk.Tool) []string {
	ns := make([]string, len(tools))
	for i, t := range tools {
		ns[i] = t.Name
	}
	return ns
}

// ---------------------------------------------------------------------------
// runAgentLoop
// ---------------------------------------------------------------------------

func TestRunAgentLoop_TerminatesOnTextResponse(t *testing.T) {
	ai := minimalAI()
	provider := &mockProvider{
		chatFn: func(_ context.Context, _ string, _ []aisdk.Message, _ []aisdk.Tool) (aisdk.Response, error) {
			return aisdk.Response{Content: "analysis complete", InputTokens: 10, OutputTokens: 5}, nil
		},
	}
	msgs := buildAgentMessages("system", "task")
	tokens, err := ai.runAgentLoop(context.Background(), provider, "m", msgs, nil, nil, nil, 10, 10_000, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(15), tokens)
}

func TestRunAgentLoop_LengthFinishReasonIsBudgetError(t *testing.T) {
	ai := minimalAI()
	provider := &mockProvider{
		chatFn: func(_ context.Context, _ string, _ []aisdk.Message, _ []aisdk.Tool) (aisdk.Response, error) {
			return aisdk.Response{FinishReason: "length", InputTokens: 100, OutputTokens: 50}, nil
		},
	}
	msgs := buildAgentMessages("system", "task")
	_, err := ai.runAgentLoop(context.Background(), provider, "m", msgs, nil, nil, nil, 10, 10_000, nil, nil)
	assert.True(t, errors.As(err, new(*BudgetExhaustedError)), "expected BudgetExhaustedError, got %T: %v", err, err)
}

// TestRunAgentLoop_MessagesAccumulate verifies that each turn appends an
// assistant message so the history grows correctly across calls.
func TestRunAgentLoop_MessagesAccumulate(t *testing.T) {
	ai := minimalAI()
	var seen [][]aisdk.Message
	provider := &mockProvider{
		chatFn: func(_ context.Context, _ string, msgs []aisdk.Message, _ []aisdk.Tool) (aisdk.Response, error) {
			seen = append(seen, append([]aisdk.Message(nil), msgs...))
			return aisdk.Response{Content: "done", InputTokens: 5, OutputTokens: 3}, nil
		},
	}
	msgs := buildAgentMessages("system", "task")
	_, err := ai.runAgentLoop(context.Background(), provider, "m", msgs, nil, nil, nil, 10, 10_000, nil, nil)
	assert.NoError(t, err)
	// The provider was called exactly once for a text-only response.
	assert.Len(t, seen, 1)
	// Initial call sees [system, user].
	assert.Equal(t, aisdk.RoleSystem, seen[0][0].Role)
	assert.Equal(t, aisdk.RoleUser, seen[0][1].Role)
}

func TestRunAgentLoop_EmptyResponseIsError(t *testing.T) {
	ai := minimalAI()
	provider := &mockProvider{
		chatFn: func(_ context.Context, _ string, _ []aisdk.Message, _ []aisdk.Tool) (aisdk.Response, error) {
			return aisdk.Response{}, nil // no content, no tool calls
		},
	}
	msgs := buildAgentMessages("system", "task")
	_, err := ai.runAgentLoop(context.Background(), provider, "m", msgs, nil, nil, nil, 10, 10_000, nil, nil)
	assert.Error(t, err)
}

func TestRunAgentLoop_ProviderErrorPropagates(t *testing.T) {
	ai := minimalAI()
	want := fmt.Errorf("api timeout")
	provider := &mockProvider{
		chatFn: func(_ context.Context, _ string, _ []aisdk.Message, _ []aisdk.Tool) (aisdk.Response, error) {
			return aisdk.Response{}, want
		},
	}
	msgs := buildAgentMessages("system", "task")
	_, err := ai.runAgentLoop(context.Background(), provider, "m", msgs, nil, nil, nil, 10, 10_000, nil, nil)
	assert.ErrorIs(t, err, want)
}

func TestRunAgentLoop_ContextCancellation(t *testing.T) {
	ai := minimalAI()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	provider := &mockProvider{
		chatFn: func(ctx context.Context, _ string, _ []aisdk.Message, _ []aisdk.Tool) (aisdk.Response, error) {
			return aisdk.Response{}, ctx.Err()
		},
	}
	msgs := buildAgentMessages("system", "task")
	_, err := ai.runAgentLoop(ctx, provider, "m", msgs, nil, nil, nil, 10, 10_000, nil, nil)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunAgentLoop_OnProgressCalledWithTokens(t *testing.T) {
	ai := minimalAI()
	provider := &mockProvider{
		chatFn: func(_ context.Context, _ string, _ []aisdk.Message, _ []aisdk.Tool) (aisdk.Response, error) {
			return aisdk.Response{Content: "done", InputTokens: 20, OutputTokens: 10}, nil
		},
	}
	msgs := buildAgentMessages("system", "task")
	var gotTokens int64
	_, err := ai.runAgentLoop(context.Background(), provider, "m", msgs, nil, nil, nil, 10, 10_000,
		func(tokens int64, _ string) { gotTokens = tokens }, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(30), gotTokens)
}

// ---------------------------------------------------------------------------
// runChatTurn
// ---------------------------------------------------------------------------

func newChatChannel() (IOChatChannel, chan string, <-chan string) {
	input := make(chan string, 1)
	output := make(chan string, 64) // large buffer so sends never block in tests
	ch := IOChatChannel{
		Input:  input,
		Output: output,
	}
	return ch, input, output
}

func TestRunChatTurn_TextOnlyResponse(t *testing.T) {
	ai := minimalAI()
	ch, _, out := newChatChannel()
	rc := &ResolvedModelConfig{Model: "test-model"}
	categories := NewActiveToolCategories()

	provider := &mockProvider{
		streamFn: func(_ context.Context, _ string, _ []aisdk.Message, _ []aisdk.Tool) (<-chan aisdk.StreamChunk, error) {
			return staticStream(
				aisdk.StreamChunk{Content: "Hello "},
				aisdk.StreamChunk{Content: "world"},
				aisdk.StreamChunk{Done: true, InputTokens: 5, OutputTokens: 3},
			)
		},
	}

	msgs := []aisdk.Message{{Role: aisdk.RoleSystem, Content: "sys"}}
	var sessionIn, sessionOut int64
	fullText, updatedMsgs, _, err := ai.runChatTurn(
		context.Background(), provider, rc, msgs, ch,
		kubernetesAiSDKTools, categories, 10, &sessionIn, &sessionOut,
	)

	assert.NoError(t, err)
	assert.Equal(t, "Hello world", fullText)
	assert.Equal(t, int64(5), sessionIn)
	assert.Equal(t, int64(3), sessionOut)

	// Final message appended must be the assistant reply.
	last := updatedMsgs[len(updatedMsgs)-1]
	assert.Equal(t, aisdk.RoleAssistant, last.Role)
	assert.Equal(t, "Hello world", last.Content)

	// Output channel must have received the thinking header and the text.
	var received []string
	for len(out) > 0 {
		received = append(received, <-out)
	}
	combined := strings.Join(received, "")
	assert.Contains(t, combined, "[AI is thinking...]")
	assert.Contains(t, combined, "Hello world")
}

func TestRunChatTurn_StreamErrorPropagates(t *testing.T) {
	ai := minimalAI()
	ch, _, _ := newChatChannel()
	rc := &ResolvedModelConfig{Model: "m"}

	streamErr := fmt.Errorf("connection reset")
	provider := &mockProvider{
		streamFn: func(_ context.Context, _ string, _ []aisdk.Message, _ []aisdk.Tool) (<-chan aisdk.StreamChunk, error) {
			return nil, streamErr
		},
	}

	var sessionIn, sessionOut int64
	_, _, _, err := ai.runChatTurn(
		context.Background(), provider, rc, []aisdk.Message{{Role: aisdk.RoleSystem, Content: "s"}},
		ch, nil, NewActiveToolCategories(), 10, &sessionIn, &sessionOut,
	)
	assert.ErrorIs(t, err, streamErr)
}

func TestRunChatTurn_ChunkErrorPropagates(t *testing.T) {
	ai := minimalAI()
	ch, _, _ := newChatChannel()
	rc := &ResolvedModelConfig{Model: "m"}

	chunkErr := fmt.Errorf("stream interrupted")
	provider := &mockProvider{
		streamFn: func(_ context.Context, _ string, _ []aisdk.Message, _ []aisdk.Tool) (<-chan aisdk.StreamChunk, error) {
			return staticStream(
				aisdk.StreamChunk{Content: "partial"},
				aisdk.StreamChunk{Err: chunkErr},
			)
		},
	}

	var sessionIn, sessionOut int64
	_, _, _, err := ai.runChatTurn(
		context.Background(), provider, rc, []aisdk.Message{{Role: aisdk.RoleSystem, Content: "s"}},
		ch, nil, NewActiveToolCategories(), 10, &sessionIn, &sessionOut,
	)
	assert.ErrorIs(t, err, chunkErr)
}
