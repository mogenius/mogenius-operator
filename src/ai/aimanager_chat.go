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

// emitChatError pushes a user-visible error bubble (plus the completion
// marker so the input unlocks) into the chat stream. Early failures used to
// be logged only — the browser saw a silently dead socket.
func emitChatError(ctx context.Context, ioChannel IOChatChannel, message string) {
	select {
	case ioChannel.Output <- fmt.Sprintf("\n[Error: %s]", message):
	case <-ctx.Done():
		return
	}
	select {
	case ioChannel.Output <- "[COMPLETED]":
	case <-ctx.Done():
	}
}

func (ai *aiManager) Chat(ctx context.Context, ioChannel IOChatChannel) error {
	// Chat is driven purely by chat-enabled AiModels: the session's model
	// choice (dropdown) arrives as ioChannel.ModelRef; without one the
	// default-flagged model wins if chat-enabled, else the oldest.
	rc, err := ai.resolveChatModelConfig(ioChannel.ModelRef)
	if err != nil {
		emitChatError(ctx, ioChannel, err.Error())
		return fmt.Errorf("failed to resolve chat model: %w", err)
	}

	// Exhausted daily budget: tell the user right away instead of on the
	// first message. The per-turn gate inside the SDK loops still guards
	// every message (the budget may free up mid-session via a reset).
	if ai.isModelBudgetExceeded(rc) {
		emitChatError(ctx, ioChannel, ai.modelBudgetError(rc))
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

	switch rc.Sdk {
	case AiSdkTypeOpenAI:
		return ai.openaiChat(ctx, ioChannel, systemPrompt, rc)
	case AiSdkTypeAnthropic:
		return ai.anthropicChat(ctx, ioChannel, systemPrompt, rc)
	case AiSdkTypeOllama:
		return ai.ollamaChat(ctx, ioChannel, systemPrompt, rc)
	default:
		emitChatError(ctx, ioChannel, fmt.Sprintf("unsupported AI SDK type: %s", rc.Sdk))
		return fmt.Errorf("unsupported AI SDK type: %s", rc.Sdk)
	}
}
