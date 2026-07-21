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
	AI_CONFIG_MAX_TOKENS_PER_RUN    = "MAX_TOKENS_PER_RUN"
	// GitHub Personal Access Token for AI to access GitHub API.
	// GitHub PAT fine-grained permissions recommendation:
	//  - only select repositories that the AI needs to access, e.g. "my-org/my-repo"
	//  - permissions:
	//      Contents (read-write) — required for reading repo files and writing .ai-context.md
	//      Metadata (read-only)
	//      Pull requests (read-write)
	AI_CONFIG_GITHUB_PAT            = "AI_CONFIG_GITHUB_PAT"
	AI_CONFIG_GIT_MEMORY_REPOSITORY = "AI_CONFIG_GIT_MEMORY_REPOSITORY" // <owner>/<repo> format, e.g. "my-org/my-repo"
)

type AiSdkType string

const (
	AiSdkTypeOpenAI    AiSdkType = "openai"
	AiSdkTypeAnthropic AiSdkType = "anthropic"
	AiSdkTypeOllama    AiSdkType = "ollama"
)

func (ai *aiManager) InjectAiPromptConfig(prompt AiPromptConfig, aiPrompts *AiPrompts) {
	ai.promptConfigMu.Lock()
	ai.aiPromptConfig = &prompt
	ai.promptConfigMu.Unlock()

	if aiPrompts != nil {
		ai.chatPromptMu.Lock()
		defer ai.chatPromptMu.Unlock()
		ai.aiPrompts = *aiPrompts
	}

	ai.logger.Info("AI Prompt Config loaded successfully", "name", prompt.Name)
}

// promptConfig returns the current config snapshot. The pointer is replaced
// atomically on inject and the pointee never mutated, so reads are safe once
// the pointer is fetched under the lock.
func (ai *aiManager) promptConfig() *AiPromptConfig {
	ai.promptConfigMu.RLock()
	defer ai.promptConfigMu.RUnlock()
	return ai.aiPromptConfig
}

func (ai *aiManager) isAiPromptConfigInitialized() bool {
	return ai.promptConfig() != nil
}

func (ai *aiManager) isAiModelConfigInitialized() bool {
	ownNamespace := ai.config.Get("MO_OWN_NAMESPACE")
	// Mirror getAiSettingByKey: try the cache first, then fall back to a
	// direct API read. Using only the direct call made this flag report
	// false whenever that single Get failed (e.g. API throttling), even
	// though the config existed and the cache held it.
	if store.GetSecret(ownNamespace, AI_CONFIG_SECRET_NAME) != nil {
		return true
	}
	configSecret, err := ai.secretGetter(ownNamespace, AI_CONFIG_SECRET_NAME)
	return err == nil && configSecret != nil
}

func (ai *aiManager) getSystemPrompt() string {
	cfg := ai.promptConfig()
	if cfg == nil {
		return ""
	}
	return cfg.SystemPrompt
}

func (ai *aiManager) GetPromptConfig() (*AiPromptConfig, error) {
	cfg := ai.promptConfig()
	if cfg == nil {
		return nil, fmt.Errorf("AI prompt configuration is not initialized")
	}
	return cfg, nil
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

// defaultMaxTokensPerRun caps a single run when MAX_TOKENS_PER_RUN is not
// configured. Runs finish gracefully at the cap with the findings collected
// so far; an explicit 0 opts into unlimited.
const defaultMaxTokensPerRun = int64(30000)

// getAiMaxTokensPerRun returns the per-run token budget; 0 means unlimited.
// A missing key or unparsable value falls back to the default cap.
func (ai *aiManager) getAiMaxTokensPerRun() int64 {
	data, err := ai.getAiSettingByKey(AI_CONFIG_MAX_TOKENS_PER_RUN)
	if err != nil {
		return defaultMaxTokensPerRun
	}
	limit, err := strconv.ParseInt(data, 10, 64)
	if err != nil || limit < 0 {
		ai.logger.Warn("Invalid max tokens per run value, using default", "value", data, "default", defaultMaxTokensPerRun)
		return defaultMaxTokensPerRun
	}
	return limit
}

func (ai *aiManager) getGitHubPat() (string, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_GITHUB_PAT)
	if err != nil {
		return "", fmt.Errorf("failed to get GitHub PAT: %v", err)
	}
	return data, nil
}

func (ai *aiManager) getGitMemoryRepository() (string, error) {
	data, err := ai.getAiSettingByKey(AI_CONFIG_GIT_MEMORY_REPOSITORY)
	if err != nil {
		return "", fmt.Errorf("failed to get GitHub repo: %v", err)
	}
	return data, nil
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
