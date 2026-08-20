package ai

import (
	"fmt"
	"mogenius-operator/src/ai/aisdk"
)

// newAiSDKProvider creates a provider-neutral aisdk.Provider from the resolved
// model config. Returns an error only for Ollama (URL parsing).
func newAiSDKProvider(rc *ResolvedModelConfig) (aisdk.Provider, error) {
	switch rc.Sdk {
	case AiSdkTypeOpenAI:
		return aisdk.NewOpenAIProvider(rc.ApiKey, rc.BaseUrl), nil
	case AiSdkTypeAnthropic:
		return aisdk.NewAnthropicProvider(rc.ApiKey, rc.BaseUrl), nil
	case AiSdkTypeOllama:
		return aisdk.NewOllamaProvider(rc.BaseUrl)
	default:
		return nil, fmt.Errorf("unsupported AI SDK type: %s", rc.Sdk)
	}
}
