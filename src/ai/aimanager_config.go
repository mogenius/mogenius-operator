package ai

import (
	"fmt"
	"mogenius-operator/src/store"

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
	// AI is configured as soon as at least one AiModel CR exists; the legacy
	// secret-based model configuration is gone.
	models, err := ai.listAiModels()
	return err == nil && len(models) > 0
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
