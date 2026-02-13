package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
)

// MCPServerConfig describes a remote MCP server to connect to.
type MCPServerConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Pat  string `json:"-"` // Personal Access Token (never serialized)
}

// mcpClientManager manages connections to MCP servers and exposes their tools.
type mcpClientManager struct {
	logger   *slog.Logger
	sessions map[string]*mcpSession // keyed by server name
	mu       sync.RWMutex
}

type mcpSession struct {
	name    string
	session *mcp.ClientSession
	tools   []*mcp.Tool
}

type authTransport struct {
	base  http.RoundTripper
	token string
}

func (a *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+a.token)
	return a.base.RoundTrip(req)
}

func newMCPClientManager(logger *slog.Logger) *mcpClientManager {
	return &mcpClientManager{
		logger:   logger,
		sessions: make(map[string]*mcpSession),
	}
}

// Connect initializes a connection to a remote MCP server via Streamable HTTP.
func (m *mcpClientManager) Connect(ctx context.Context, cfg MCPServerConfig) error {
	m.logger.Info("Connecting to MCP server", "name", cfg.Name, "url", cfg.URL)

	client := mcp.NewClient(
		&mcp.Implementation{Name: "mogenius-operator", Version: "v1.0.0"},
		nil,
	)

	transport := &mcp.StreamableClientTransport{
		Endpoint: cfg.URL,
		HTTPClient: &http.Client{
			Transport: &authTransport{
				base:  http.DefaultTransport,
				token: cfg.Pat,
			},
		},
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server %s: %w", cfg.Name, err)
	}

	// Discover tools
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		session.Close()
		return fmt.Errorf("failed to list tools from MCP server %s: %w", cfg.Name, err)
	}

	m.mu.Lock()
	m.sessions[cfg.Name] = &mcpSession{
		name:    cfg.Name,
		session: session,
		tools:   toolsResult.Tools,
	}
	m.mu.Unlock()

	//m.logger.Info("Connected to MCP server", "name", cfg.Name, "toolCount", len(toolsResult.Tools))
	//for _, tool := range toolsResult.Tools {
	//	m.logger.Info("MCP tool discovered", "server", cfg.Name, "tool", tool.Name)
	//}

	return nil
}

// Close closes all MCP sessions.
func (m *mcpClientManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, s := range m.sessions {
		if err := s.session.Close(); err != nil {
			m.logger.Error("Error closing MCP session", "name", name, "error", err)
		}
	}
	m.sessions = make(map[string]*mcpSession)
}

// CallTool calls a tool on the appropriate MCP server.
func (m *mcpClientManager) CallTool(ctx context.Context, toolName string, args map[string]any) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sessions {
		for _, tool := range s.tools {
			if tool.Name == toolName {
				result, err := s.session.CallTool(ctx, &mcp.CallToolParams{
					Name:      toolName,
					Arguments: args,
				})
				if err != nil {
					return "", fmt.Errorf("MCP tool call %q failed: %w", toolName, err)
				}

				if result.IsError {
					return fmt.Sprintf("MCP tool error: %s", extractMCPText(result)), nil
				}

				return extractMCPText(result), nil
			}
		}
	}

	return "", fmt.Errorf("MCP tool %q not found on any connected server", toolName)
}

// HasSession returns true if a session with the given name exists.
func (m *mcpClientManager) HasSession(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.sessions[name]
	return ok
}

// IsMCPTool returns true if the tool name belongs to an MCP server.
func (m *mcpClientManager) IsMCPTool(toolName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sessions {
		for _, tool := range s.tools {
			if tool.Name == toolName {
				return true
			}
		}
	}
	return false
}

// GetAnthropicTools returns all MCP tools in Anthropic SDK format.
func (m *mcpClientManager) GetAnthropicTools() []anthropic.ToolParam {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []anthropic.ToolParam
	for _, s := range m.sessions {
		for _, tool := range s.tools {
			properties, required := mcpSchemaToPropertiesAndRequired(tool.InputSchema)
			tools = append(tools, anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Type:       "object",
					Properties: properties,
					Required:   required,
				},
			})
		}
	}
	return tools
}

// GetOpenAITools returns all MCP tools in OpenAI SDK format.
func (m *mcpClientManager) GetOpenAITools() []openai.ChatCompletionToolUnionParam {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []openai.ChatCompletionToolUnionParam
	for _, s := range m.sessions {
		for _, tool := range s.tools {
			params := mcpSchemaToFunctionParams(tool.InputSchema)
			tools = append(tools, openai.ChatCompletionToolUnionParam{
				OfFunction: &openai.ChatCompletionFunctionToolParam{
					Function: openai.FunctionDefinitionParam{
						Name:        tool.Name,
						Description: openai.String(tool.Description),
						Parameters:  params,
					},
				},
			})
		}
	}
	return tools
}

// extractMCPText extracts text from an MCP tool result.
func extractMCPText(result *mcp.CallToolResult) string {
	var texts []string
	for _, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			texts = append(texts, tc.Text)
		} else {
			b, err := json.Marshal(content)
			if err == nil {
				texts = append(texts, string(b))
			}
		}
	}
	if len(texts) == 0 {
		return "No content returned"
	}
	resultText := texts[0]
	for i := 1; i < len(texts); i++ {
		resultText += "\n" + texts[i]
	}
	return resultText
}

// mcpSchemaToPropertiesAndRequired converts an MCP tool's InputSchema (any / map[string]any)
// to the properties map and required slice used by the Anthropic SDK.
func mcpSchemaToPropertiesAndRequired(schema any) (map[string]interface{}, []string) {
	m, ok := schema.(map[string]any)
	if !ok || m == nil {
		return map[string]interface{}{}, nil
	}

	properties := make(map[string]interface{})
	if props, ok := m["properties"].(map[string]any); ok {
		for k, v := range props {
			properties[k] = v
		}
	}

	var required []string
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}

	return properties, required
}

// mcpSchemaToFunctionParams converts an MCP tool's InputSchema (any / map[string]any)
// to the FunctionParameters used by the OpenAI SDK.
func mcpSchemaToFunctionParams(schema any) openai.FunctionParameters {
	m, ok := schema.(map[string]any)
	if !ok || m == nil {
		return openai.FunctionParameters{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	params := openai.FunctionParameters{
		"type": "object",
	}

	if props, ok := m["properties"].(map[string]any); ok {
		params["properties"] = props
	} else {
		params["properties"] = map[string]interface{}{}
	}

	if req, ok := m["required"].([]any); ok {
		params["required"] = req
	}

	return params
}
