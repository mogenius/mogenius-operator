package ai

import (
	"context"
	"fmt"
	"strings"

	json "github.com/goccy/go-json"
)

func (ai *aiManager) InjectAiChatSystemPrompt(prompt string) bool {
	ai.chatPromptMu.Lock()
	defer ai.chatPromptMu.Unlock()
	ai.customChatSystemPrompt = prompt
	return true
}

func (ai *aiManager) InjectAiChatGitHubAiContextSystemPrompt(prompt string) bool {
	ai.chatPromptMu.Lock()
	defer ai.chatPromptMu.Unlock()
	ai.customChatGitHubAiContextSystemPrompt = prompt
	return true
}

// fetchGitHubAiContext tries to fetch .ai-context.md from the configured GitHub repo
// via the MCP get_file_contents tool.
func (ai *aiManager) fetchGitHubAiContext(ctx context.Context) (content string, err error) {
	repo, err := ai.getGitHubRepo()
	if err != nil || repo == "" {
		return "", fmt.Errorf("no GitHub repo configured: %w", err)
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid GitHub repo format, expected owner/repo: %s", repo)
	}

	if ai.mcpManager == nil {
		return "", fmt.Errorf("mcpManager is nil")
	}

	if !ai.mcpManager.HasSession("github") {
		return "", fmt.Errorf("no GitHub MCP session available")
	}

	ai.logger.Info("Pre-fetching .ai-context.md from GitHub", "owner", parts[0], "repo", parts[1])
	result, err := ai.mcpManager.CallTool(ctx, "get_file_contents", map[string]any{
		"owner": parts[0],
		"repo":  parts[1],
		"path":  ".ai-context.md",
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch .ai-context.md from %s: %w", repo, err)
	}

	ai.logger.Info("Successfully pre-loaded .ai-context.md from GitHub", "repo", repo, "contentLength", len(result))
	return result, nil
}

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
	systemPrompt := ai.customChatSystemPrompt
	githubPrompt := ai.customChatGitHubAiContextSystemPrompt
	ai.chatPromptMu.RUnlock()

	if systemPrompt == "" {
		ai.logger.Warn("No AI model configuration found, using default value")
	}
	if githubPrompt == "" {
		ai.logger.Warn("No GitHub AI context system prompt configured, using default value")
	}

	if repo, err := ai.getGitHubRepo(); err == nil && repo != "" {
		systemPrompt += "\n\n" + strings.ReplaceAll(githubPrompt, "{{GITHUB_REPO}}", repo)
	}
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{USER_NAME}}", fmt.Sprintf("%s %s", ioChannel.User.FirstName, ioChannel.User.LastName))
	systemPrompt = strings.ReplaceAll(systemPrompt, "{{USER_EMAIL}}", ioChannel.User.Email)

	// Pre-fetch .ai-context.md from GitHub if PAT and repo are configured
	if aiContext, err := ai.fetchGitHubAiContext(ctx); err != nil {
		ai.logger.Warn("Could not pre-fetch .ai-context.md", "error", err)
	} else if aiContext != "" {
		systemPrompt += "\n\n## Pre-loaded .ai-context.md\n" + aiContext
	}

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
