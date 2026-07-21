package ai

import (
	"testing"
	"time"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseSdkType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected AiSdkType
		wantErr  bool
	}{
		{name: "openai", input: "openai", expected: AiSdkTypeOpenAI},
		{name: "anthropic", input: "anthropic", expected: AiSdkTypeAnthropic},
		{name: "ollama", input: "ollama", expected: AiSdkTypeOllama},
		{name: "unknown", input: "gemini", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "case sensitive", input: "OpenAI", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdk, err := parseSdkType(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, sdk)
		})
	}
}

func TestValidateAiModelSpec(t *testing.T) {
	keyRef := &v1alpha1.SecretKeyRef{Name: "anthropic-credentials"}
	tests := []struct {
		name    string
		spec    v1alpha1.AiModelSpec
		wantErr string
	}{
		{
			name: "valid anthropic",
			spec: v1alpha1.AiModelSpec{Sdk: "anthropic", Model: "claude-sonnet-5", ApiKeySecretRef: keyRef},
		},
		{
			name: "valid openai with url",
			spec: v1alpha1.AiModelSpec{Sdk: "openai", Model: "gpt-5", ApiUrl: "https://api.openai.com/v1", ApiKeySecretRef: keyRef},
		},
		{
			name: "valid ollama without key",
			spec: v1alpha1.AiModelSpec{Sdk: "ollama", Model: "llama3.1:8b", ApiUrl: "http://ollama:11434"},
		},
		{
			name:    "unknown sdk",
			spec:    v1alpha1.AiModelSpec{Sdk: "gemini", Model: "gemini-pro"},
			wantErr: "unsupported SDK type",
		},
		{
			name:    "missing model",
			spec:    v1alpha1.AiModelSpec{Sdk: "anthropic", ApiKeySecretRef: keyRef},
			wantErr: "model must not be empty",
		},
		{
			name:    "ollama without apiUrl",
			spec:    v1alpha1.AiModelSpec{Sdk: "ollama", Model: "llama3.1:8b"},
			wantErr: "apiUrl is required",
		},
		{
			name:    "anthropic without secret ref",
			spec:    v1alpha1.AiModelSpec{Sdk: "anthropic", Model: "claude-sonnet-5"},
			wantErr: "apiKeySecretRef is required",
		},
		{
			name:    "openai with empty secret name",
			spec:    v1alpha1.AiModelSpec{Sdk: "openai", Model: "gpt-5", ApiKeySecretRef: &v1alpha1.SecretKeyRef{}},
			wantErr: "apiKeySecretRef is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAiModelSpec(tt.spec)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestPickDefaultAiModel(t *testing.T) {
	model := func(name string, isDefault bool, created time.Time) v1alpha1.AiModel {
		return v1alpha1.AiModel{
			ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(created)},
			Spec:       v1alpha1.AiModelSpec{Sdk: "anthropic", Model: "m", Default: isDefault},
		}
	}
	now := time.Now()

	t.Run("no models", func(t *testing.T) {
		assert.Nil(t, pickDefaultAiModel(nil))
	})

	t.Run("no default marked", func(t *testing.T) {
		assert.Nil(t, pickDefaultAiModel([]v1alpha1.AiModel{model("a", false, now)}))
	})

	t.Run("single default among several", func(t *testing.T) {
		picked := pickDefaultAiModel([]v1alpha1.AiModel{
			model("a", false, now),
			model("b", true, now),
			model("c", false, now),
		})
		assert.NotNil(t, picked)
		assert.Equal(t, "b", picked.Name)
	})

	t.Run("oldest default wins", func(t *testing.T) {
		picked := pickDefaultAiModel([]v1alpha1.AiModel{
			model("newer", true, now),
			model("older", true, now.Add(-time.Hour)),
		})
		assert.NotNil(t, picked)
		assert.Equal(t, "older", picked.Name)
	})

	t.Run("name breaks creation ties deterministically", func(t *testing.T) {
		picked := pickDefaultAiModel([]v1alpha1.AiModel{
			model("zeta", true, now),
			model("alpha", true, now),
		})
		assert.NotNil(t, picked)
		assert.Equal(t, "alpha", picked.Name)
	})
}
