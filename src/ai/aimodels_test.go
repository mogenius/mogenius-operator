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

func TestNormalizeAiModelSpec(t *testing.T) {
	tests := []struct {
		name     string
		sdk      string
		apiUrl   string
		expected string
	}{
		{name: "openai default cleared", sdk: "openai", apiUrl: "https://api.openai.com/v1", expected: ""},
		{name: "openai default with trailing slash cleared", sdk: "openai", apiUrl: "https://api.openai.com/v1/", expected: ""},
		{name: "openai default with whitespace cleared", sdk: "openai", apiUrl: "  https://api.openai.com/v1 ", expected: ""},
		{name: "anthropic default cleared", sdk: "anthropic", apiUrl: "https://api.anthropic.com", expected: ""},
		{name: "anthropic default with trailing slash cleared", sdk: "anthropic", apiUrl: "https://api.anthropic.com/", expected: ""},
		{name: "openai custom endpoint kept", sdk: "openai", apiUrl: "https://litellm.example.com/v1", expected: "https://litellm.example.com/v1"},
		{name: "openai host without /v1 kept", sdk: "openai", apiUrl: "https://api.openai.com", expected: "https://api.openai.com"},
		{name: "anthropic proxy kept", sdk: "anthropic", apiUrl: "https://gateway.example.com/anthropic", expected: "https://gateway.example.com/anthropic"},
		{name: "ollama url kept", sdk: "ollama", apiUrl: "http://ollama.ollama.svc:11434", expected: "http://ollama.ollama.svc:11434"},
		{name: "empty stays empty", sdk: "openai", apiUrl: "", expected: ""},
		{name: "whitespace-only trimmed", sdk: "ollama", apiUrl: "   ", expected: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := NormalizeAiModelSpec(v1alpha1.AiModelSpec{Sdk: tt.sdk, Model: "some-model", ApiUrl: tt.apiUrl})
			assert.Equal(t, tt.expected, spec.ApiUrl)
			assert.Equal(t, tt.sdk, spec.Sdk)
			assert.Equal(t, "some-model", spec.Model)
		})
	}
}

func TestSupportedAiSdks(t *testing.T) {
	sdks := SupportedAiSdks()
	assert.Len(t, sdks, 3)

	byName := make(map[string]AiSdkInfo, len(sdks))
	for _, info := range sdks {
		// Every advertised SDK must be accepted by the parser.
		_, err := parseSdkType(info.Sdk)
		assert.NoError(t, err, info.Sdk)
		byName[info.Sdk] = info
	}

	assert.Equal(t, "https://api.openai.com/v1", byName["openai"].DefaultApiUrl)
	assert.True(t, byName["openai"].ApiKeyRequired)
	assert.False(t, byName["openai"].ApiUrlRequired)

	assert.Equal(t, "https://api.anthropic.com", byName["anthropic"].DefaultApiUrl)
	assert.True(t, byName["anthropic"].ApiKeyRequired)
	assert.False(t, byName["anthropic"].ApiUrlRequired)

	assert.Empty(t, byName["ollama"].DefaultApiUrl)
	assert.False(t, byName["ollama"].ApiKeyRequired)
	assert.True(t, byName["ollama"].ApiUrlRequired)
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
		assert.Nil(t, PickDefaultAiModel(nil))
	})

	t.Run("no default marked", func(t *testing.T) {
		assert.Nil(t, PickDefaultAiModel([]v1alpha1.AiModel{model("a", false, now)}))
	})

	t.Run("single default among several", func(t *testing.T) {
		picked := PickDefaultAiModel([]v1alpha1.AiModel{
			model("a", false, now),
			model("b", true, now),
			model("c", false, now),
		})
		assert.NotNil(t, picked)
		assert.Equal(t, "b", picked.Name)
	})

	t.Run("oldest default wins", func(t *testing.T) {
		picked := PickDefaultAiModel([]v1alpha1.AiModel{
			model("newer", true, now),
			model("older", true, now.Add(-time.Hour)),
		})
		assert.NotNil(t, picked)
		assert.Equal(t, "older", picked.Name)
	})

	t.Run("name breaks creation ties deterministically", func(t *testing.T) {
		picked := PickDefaultAiModel([]v1alpha1.AiModel{
			model("zeta", true, now),
			model("alpha", true, now),
		})
		assert.NotNil(t, picked)
		assert.Equal(t, "alpha", picked.Name)
	})
}

func TestValidateAiModelDefaultUnique(t *testing.T) {
	model := func(name string, isDefault bool) v1alpha1.AiModel {
		return v1alpha1.AiModel{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       v1alpha1.AiModelSpec{Sdk: "anthropic", Model: "m", Default: isDefault},
		}
	}
	defaultSpec := v1alpha1.AiModelSpec{Sdk: "anthropic", Model: "m", Default: true}
	nonDefaultSpec := v1alpha1.AiModelSpec{Sdk: "anthropic", Model: "m"}

	t.Run("non-default spec always passes", func(t *testing.T) {
		err := ValidateAiModelDefaultUnique("new", nonDefaultSpec, []v1alpha1.AiModel{model("other", true)})
		assert.NoError(t, err)
	})

	t.Run("first default passes", func(t *testing.T) {
		err := ValidateAiModelDefaultUnique("new", defaultSpec, []v1alpha1.AiModel{model("other", false)})
		assert.NoError(t, err)
	})

	t.Run("second default is rejected", func(t *testing.T) {
		err := ValidateAiModelDefaultUnique("new", defaultSpec, []v1alpha1.AiModel{model("other", true)})
		assert.ErrorContains(t, err, `"other" is already marked as default`)
	})

	t.Run("updating the current default passes", func(t *testing.T) {
		err := ValidateAiModelDefaultUnique("current", defaultSpec, []v1alpha1.AiModel{model("current", true), model("other", false)})
		assert.NoError(t, err)
	})
}
