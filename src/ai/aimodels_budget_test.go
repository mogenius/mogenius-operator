package ai

import (
	"testing"
	"time"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }

// Ollama specs need no Secret plumbing, so resolveAiModel is testable on a
// bare aiManager.
func ollamaModelFixture(name string) *v1alpha1.AiModel {
	return &v1alpha1.AiModel{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "mogenius"},
		Spec: v1alpha1.AiModelSpec{
			Sdk:    "ollama",
			Model:  "qwen3-coder:30b",
			ApiUrl: "http://ollama:11434",
		},
	}
}

func TestResolveAiModelBuiltInDefaults(t *testing.T) {
	ai := &aiManager{}
	model := ollamaModelFixture("local")

	rc, err := ai.resolveAiModel(model, "modelRef")
	assert.NoError(t, err)
	assert.Equal(t, "local", rc.ModelCrName)
	assert.Equal(t, defaultMaxToolCalls, rc.MaxToolCalls)
	assert.Equal(t, defaultMaxTokensPerRun, rc.MaxTokensPerRun)
	assert.Equal(t, defaultDailyTokenLimit, rc.DailyTokenLimit)
}

func TestResolveAiModelSpecValuesBeatDefaults(t *testing.T) {
	ai := &aiManager{}
	model := ollamaModelFixture("local")
	model.Spec.MaxToolCalls = intPtr(80)
	model.Spec.MaxTokensPerRun = int64Ptr(0) // explicit unlimited
	model.Spec.DailyTokenLimit = int64Ptr(0) // explicit unlimited

	rc, err := ai.resolveAiModel(model, "default")
	assert.NoError(t, err)
	assert.Equal(t, 80, rc.MaxToolCalls)
	assert.Equal(t, int64(0), rc.MaxTokensPerRun)
	assert.Equal(t, int64(0), rc.DailyTokenLimit)
}

func TestApplyAgentBudgetOverrides(t *testing.T) {
	rc := &ResolvedModelConfig{MaxToolCalls: 50, MaxTokensPerRun: 30000, DailyTokenLimit: 300000}

	// nil agent spec: nothing changes
	applyAgentBudgetOverrides(rc, nil)
	assert.Equal(t, 50, rc.MaxToolCalls)

	// unset fields: model values stay
	applyAgentBudgetOverrides(rc, &v1alpha1.AgentSpec{})
	assert.Equal(t, 50, rc.MaxToolCalls)
	assert.Equal(t, int64(30000), rc.MaxTokensPerRun)

	// agent overrides beat the model; the daily limit is NOT agent-scoped
	applyAgentBudgetOverrides(rc, &v1alpha1.AgentSpec{
		MaxToolCalls:    intPtr(10),
		MaxTokensPerRun: int64Ptr(5000),
	})
	assert.Equal(t, 10, rc.MaxToolCalls)
	assert.Equal(t, int64(5000), rc.MaxTokensPerRun)
	assert.Equal(t, int64(300000), rc.DailyTokenLimit)
}

func TestAggregateTokenUsage(t *testing.T) {
	now := time.Now()
	startOfDay := startOfTodayUnix(now)
	yesterday := now.Add(-25 * time.Hour)

	entries := []UsedToken{
		{Timestamp: now, TokensUsed: 100, ModelRef: "model-a"},
		{Timestamp: now, TokensUsed: 50, ModelRef: "model-a"},
		{Timestamp: now, TokensUsed: 30, ModelRef: "model-b"},
		// Legacy entry without ModelRef: counts into the total only.
		{Timestamp: now, TokensUsed: 7},
		// Ignored and yesterday's entries never count.
		{Timestamp: now, TokensUsed: 999, ModelRef: "model-a", IsIgnored: true},
		{Timestamp: yesterday, TokensUsed: 500, ModelRef: "model-a"},
	}

	snapshot := aggregateTokenUsage(entries, startOfDay)
	assert.Equal(t, int64(187), snapshot.TotalTokens)
	assert.Equal(t, 4, snapshot.TotalRuns)
	assert.Equal(t, int64(150), snapshot.PerModel["model-a"])
	assert.Equal(t, int64(30), snapshot.PerModel["model-b"])
	_, hasEmpty := snapshot.PerModel[""]
	assert.False(t, hasEmpty, "entries without ModelRef must not create a per-model bucket")
}

func TestIsModelBudgetExceededUnlimited(t *testing.T) {
	ai := &aiManager{}
	// 0 = unlimited and nil rc never block, without touching Valkey.
	assert.False(t, ai.isModelBudgetExceeded(nil))
	assert.False(t, ai.isModelBudgetExceeded(&ResolvedModelConfig{ModelCrName: "x", DailyTokenLimit: 0}))
}

func chatModelFixture(name string, createdMinutesAgo int, isDefault, chatEnabled bool) v1alpha1.AiModel {
	return v1alpha1.AiModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Duration(createdMinutesAgo) * time.Minute)),
		},
		Spec: v1alpha1.AiModelSpec{Sdk: "ollama", Model: "m", ApiUrl: "http://x", Default: isDefault, ChatEnabled: chatEnabled},
	}
}

func TestPickChatModel(t *testing.T) {
	older := chatModelFixture("older", 60, false, true)
	newer := chatModelFixture("newer", 5, false, true)
	defaultOff := chatModelFixture("default-off", 120, true, false)
	disabled := chatModelFixture("disabled", 30, false, false)

	// Explicit ref wins when chat-enabled.
	picked, err := pickChatModel([]v1alpha1.AiModel{older, newer}, "newer")
	assert.NoError(t, err)
	assert.Equal(t, "newer", picked.Name)

	// Explicit ref to a non-chat model fails closed with a helpful message.
	_, err = pickChatModel([]v1alpha1.AiModel{older, disabled}, "disabled")
	assert.ErrorContains(t, err, "not enabled for chat")

	// Unknown ref fails.
	_, err = pickChatModel([]v1alpha1.AiModel{older}, "nope")
	assert.ErrorContains(t, err, "not found")

	// No selection: chat-enabled default wins.
	defaultOn := chatModelFixture("default-on", 10, true, true)
	picked, err = pickChatModel([]v1alpha1.AiModel{older, defaultOn}, "")
	assert.NoError(t, err)
	assert.Equal(t, "default-on", picked.Name)

	// Default not chat-enabled: oldest chat-enabled wins.
	picked, err = pickChatModel([]v1alpha1.AiModel{newer, older, defaultOff}, "")
	assert.NoError(t, err)
	assert.Equal(t, "older", picked.Name)

	// Nothing enabled for chat: configuration error.
	_, err = pickChatModel([]v1alpha1.AiModel{disabled, defaultOff}, "")
	assert.ErrorContains(t, err, "no AI model is enabled for chat")
}
