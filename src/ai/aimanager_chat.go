package ai

import (
	"context"
	"fmt"
	"strings"

	"encoding/json"
)

func (ai *aiManager) sendTokens(inputTokens, outputTokenCount int64, sessionInputTokens, sessionOutputTokens *int64, ctx context.Context, ioChannel IOChatChannel) {
	tokensJSON, _ := json.Marshal(map[string]any{
		"input":         inputTokens,
		"output":        outputTokenCount,
		"sessionInput":  *sessionInputTokens,
		"sessionOutput": *sessionOutputTokens,
	})
	select {
	case ioChannel.Output <- fmt.Sprintf("[TOKENS:%s]", tokensJSON):
	case <-ctx.Done():
	}
}

func (ai *aiManager) Chat(ctx context.Context, ioChannel IOChatChannel) error {
	modelConfigInitialized := ai.isAiModelConfigInitialized()
	if !modelConfigInitialized {
		return fmt.Errorf("AI model configuration not initialized")
	}

	model, err := ai.getAiModel()
	if err != nil {
		return fmt.Errorf("failed to get AI model: %w", err)
	}

	maxToolCalls, err := ai.getAiMaxToolCalls()
	if err != nil {
		ai.logger.Warn("Error getting AI max tool calls (using default value)", "error", err, "defaultMaxToolCalls", maxToolCalls)
	}

	sdk, err := ai.getSdkType()
	if err != nil {
		return err
	}

	// Connect to configured MCP servers
	ai.connectMCPServers()

	// Build system prompt with user info
	ai.chatPromptMu.RLock()
	systemPrompt := ai.aiPrompts.ChatSystemPrompt
	githubSystemPrompt := ai.aiPrompts.GithubSystemPrompt
	gitMemoryRepositorySystemPrompt := ai.aiPrompts.GitMemoryRepositorySystemPrompt
	ai.chatPromptMu.RUnlock()

	if systemPrompt == "" {
		ai.logger.Warn("No AI model configuration found, using default value")
	}

	if pat, err := ai.getGitHubPat(); err == nil && pat != "" {
		systemPrompt += "\n\n" + githubSystemPrompt
	}

	if repo, err := ai.getGitMemoryRepository(); err == nil && repo != "" {
		systemPrompt += "\n\n" + strings.ReplaceAll(gitMemoryRepositorySystemPrompt, "{{MEMORY_REPO}}", repo)
	}
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{USER_NAME}}", fmt.Sprintf("%s %s", ioChannel.User.FirstName, ioChannel.User.LastName))
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{USER_EMAIL}}", ioChannel.User.Email)

	if ioChannel.User != nil {
		userInfo := ""
		if ioChannel.User.FirstName != "" {
			userInfo = ioChannel.User.FirstName
			if ioChannel.User.LastName != "" {
				userInfo += " " + ioChannel.User.LastName
			}
		}
		if userInfo != "" {
			systemPrompt += fmt.Sprintf("\n\nYou are chatting with %s.", userInfo)
		}
		if ioChannel.User.Email != "" {
			systemPrompt += fmt.Sprintf(" Their email is %s.", ioChannel.User.Email)
		}
	}

	// Append workspace role to system prompt so the LLM knows the user's permissions
	if ioChannel.WorkspaceGrant != nil && ioChannel.WorkspaceGrant.Role != "" {
		roleDescs := map[string]string{
			"viewer": "read-only access to Kubernetes and Helm resources",
			"editor": "read/write access to Kubernetes and Helm resources within allowed namespaces",
			"admin":  "full access to all Kubernetes and Helm resources",
		}
		if desc, ok := roleDescs[ioChannel.WorkspaceGrant.Role]; ok {
			systemPrompt += fmt.Sprintf("\n\nUser role: %s (%s).", ioChannel.WorkspaceGrant.Role, desc)
		}
	}

	switch sdk {
	case AiSdkTypeOpenAI:
		return ai.openaiChat(ctx, ioChannel, systemPrompt, model, maxToolCalls)
	case AiSdkTypeAnthropic:
		return ai.anthropicChat(ctx, ioChannel, systemPrompt, model, maxToolCalls)
	case AiSdkTypeOllama:
		return ai.ollamaChat(ctx, ioChannel, systemPrompt, model, maxToolCalls)
	default:
		return fmt.Errorf("unsupported AI SDK type: %s", sdk)
	}
}
