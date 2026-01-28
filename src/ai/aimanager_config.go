package ai

import (
	"fmt"
	"mogenius-operator/src/store"
	"strconv"

	v1 "k8s.io/api/core/v1"
)

const (
	// name of the Kubernetes secret that holds AI configuration
	AI_CONFIG_SECRET_NAME = "mogenius-ai-config"

	// secret keys for AI configuration
	AI_CONFIG_SDK_KEY               = "SDK"
	AI_CONFIG_MODEL_KEY             = "MODEL"
	AI_CONFIG_MAX_TOOL_CALLS_KEY    = "MAX_TOOL_CALLS"
	AI_CONFIG_API_KEY               = "API_KEY"
	AI_CONFIG_API_URL_KEY           = "API_URL"
	AI_CONFIG_DAILY_TOKEN_LIMIT_KEY = "DAILY_TOKEN_LIMIT"
)

type AiSdkType string

const (
	AiSdkTypeOpenAI    AiSdkType = "openai"
	AiSdkTypeAnthropic AiSdkType = "anthropic"
	AiSdkTypeOllama    AiSdkType = "ollama"
)

func (ai *aiManager) InjectAiPromptConfig(prompt AiPromptConfig) {
	ai.aiPromptConfig = &prompt
	ai.logger.Info("AI Prompt Config loaded successfully", "name", prompt.Name)
}

func (ai *aiManager) isAiPromptConfigInitialized() bool {
	return ai.aiPromptConfig != nil
}

func (ai *aiManager) isAiModelConfigInitialized() bool {
	ownNamespace := ai.config.Get("MO_OWN_NAMESPACE")
	configSecret, err := ai.secretGetter(ownNamespace, AI_CONFIG_SECRET_NAME)
	return err == nil && configSecret != nil
}

func (ai *aiManager) getSystemPrompt() string {
	return ai.aiPromptConfig.SystemPrompt
}
func (ai *aiManager) getAiFilters() []AiFilter {
	return ai.aiPromptConfig.Filters
}

func (ai *aiManager) getDailyTokenLimit() (int64, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_DAILY_TOKEN_LIMIT_KEY)
	if err != nil {
		return 0, fmt.Errorf("failed to get daily token limit: %v", err)
	}
	limit, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid daily token limit value: %v", err)
	}
	return limit, nil
}

func (ai *aiManager) getSdkType() (AiSdkType, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_SDK_KEY)
	if err != nil {
		return "", fmt.Errorf("failed to get API key: %v", err)
	}
	switch data {
	case "openai":
		return AiSdkTypeOpenAI, nil
	case "anthropic":
		return AiSdkTypeAnthropic, nil
	case "ollama":
		return AiSdkTypeOllama, nil
	default:
		return "", fmt.Errorf("unsupported SDK type: %s", data)
	}
}

func (ai *aiManager) getApiKey() (string, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_API_KEY)
	if err != nil {
		return "", fmt.Errorf("failed to get API key: %v", err)
	}
	return data, nil
}

func (ai *aiManager) getBaseUrl() (string, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_API_URL_KEY)
	if err != nil {
		return "", fmt.Errorf("failed to get base URL: %v", err)
	}
	return data, nil
}

func (ai *aiManager) getAiModel() (string, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_MODEL_KEY)
	if err != nil {
		return "", fmt.Errorf("failed to get AI model: %v", err)
	}
	return data, nil
}

func (ai *aiManager) getAiMaxToolCalls() (int, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_MAX_TOOL_CALLS_KEY)
	if err != nil {
		return 2, fmt.Errorf("failed to get AI maxToolCalls: %v", err)
	}

	maxToolCalls, err := strconv.Atoi(data)
	if err != nil {
		return 2, fmt.Errorf("invalid max tool calls value: %v", err)
	}

	return maxToolCalls, nil
}

func (ai *aiManager) getAiSettingByKey(key string) (string, error) {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve own namespace: %v", err)
	}
	var configSecret *v1.Secret
	configSecret = store.GetSecret(ownNamespace, AI_CONFIG_SECRET_NAME)
	// if the store is not populated, fetch the secret directly
	if configSecret == nil {
		configSecret, err = ai.secretGetter(ownNamespace, AI_CONFIG_SECRET_NAME)
		if err != nil {
			return "", fmt.Errorf("AI config secret '%s' not found in namespace '%s': %v", AI_CONFIG_SECRET_NAME, ownNamespace, err)
		}
	}

	data, exists := configSecret.Data[key]
	if !exists {
		return "", fmt.Errorf("key '%s' not found in AI config secret", key)
	}
	return string(data), nil
}
