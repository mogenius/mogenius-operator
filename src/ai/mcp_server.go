package ai

import (
	"context"
	"fmt"
	"time"
)

// MCPServerConnector defines a standard interface for connecting to MCP servers.
// Each MCP integration (GitHub, GitLab, etc.) should implement this interface.
type MCPServerConnector interface {
	// Name returns the identifier for this MCP server (e.g. "github", "gitlab").
	Name() string
	// Connect establishes a connection to the MCP server and registers its tools.
	Connect(ctx context.Context, manager *mcpClientManager) error
}

// connectMCPServers iterates over all registered MCP server connectors and connects them.
func (ai *aiManager) connectMCPServers() {
	for _, connector := range ai.mcpConnectors {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := connector.Connect(ctx, ai.mcpManager); err != nil {
			ai.logger.Info("MCP server not available", "name", connector.Name(), "error", err)
		} else {
			ai.logger.Info("MCP server connected successfully", "name", connector.Name())
		}
		cancel()
	}
}

// --- GitHub MCP Server ---

type gitHubMCPConnector struct {
	patGetter  func() (string, error)
	repoGetter func() (string, error)
}

func newGitHubMCPConnector(patGetter func() (string, error), repoGetter func() (string, error)) MCPServerConnector {
	return &gitHubMCPConnector{
		patGetter:  patGetter,
		repoGetter: repoGetter,
	}
}

func (g *gitHubMCPConnector) Name() string {
	return "github"
}

func (g *gitHubMCPConnector) Connect(ctx context.Context, manager *mcpClientManager) error {
	pat, err := g.patGetter()
	if err != nil {
		return fmt.Errorf("not configured (no GITHUB_PAT in secret): %w", err)
	}

	return manager.Connect(ctx, MCPServerConfig{
		Name: "github",
		URL:  "https://api.githubcopilot.com/mcp/",
		Pat:  pat,
	})
}