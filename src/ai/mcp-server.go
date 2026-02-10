package ai

import (
	"context"
	"time"
)

func (ai *aiManager) connectGitHubMCPServers() {
	pat, err := ai.getGitHubPat()
	if err != nil {
		ai.logger.Info("GitHub MCP server not configured (no GITHUB_PAT in secret), skipping", "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = ai.mcpManager.Connect(ctx, MCPServerConfig{
		Name: "github",
		URL:  "https://api.githubcopilot.com/mcp/",
		Pat:  pat,
	})
	if err != nil {
		ai.logger.Error("Failed to connect to GitHub MCP server", "error", err)
		return
	}
	ai.logger.Info("GitHub MCP server connected successfully")
}
