package ai

// The legacy mogenius-ai-config secret is no longer read: model/provider
// settings live on AiModel CRs, and the last remaining key (the GitHub PAT for
// the hard-coded GitHub MCP connector) was replaced by McpServer CRs. A
// leftover secret in the operator namespace is simply ignored.

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
