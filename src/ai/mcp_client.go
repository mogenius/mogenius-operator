package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
)

// invalidToolNameChars matches any character not allowed in LLM tool names.
// OpenAI requires ^[a-zA-Z0-9_-]+$; Anthropic and Ollama are similarly strict.
var invalidToolNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// sanitizeToolName replaces characters that are invalid in LLM function names
// with underscores.
func sanitizeToolName(name string) string {
	return invalidToolNameChars.ReplaceAllString(name, "_")
}

// MCPTransportType selects the transport protocol for an MCP server.
type MCPTransportType string

const (
	MCPTransportStreamableHTTP MCPTransportType = "streamableHttp"
	MCPTransportSSE            MCPTransportType = "sse"
)

// MCPServerConfig describes a remote MCP server to connect to.
type MCPServerConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`

	// Transport selects the wire protocol; defaults to StreamableHTTP when empty.
	Transport MCPTransportType `json:"transport,omitempty"`

	// Pat is a Personal Access Token added as a Bearer token (kept for the
	// existing GitHub connector; use Headers for new connectors).
	Pat string `json:"-"`

	// Headers are fully resolved (secret-free) HTTP headers sent with every
	// request. Callers resolve secret references before populating this map.
	Headers map[string]string `json:"-"`

	// ToolPolicies maps tool names to their execution policies. A nil or empty map
	// means all discovered tools are available with readOnlyHint-based defaults.
	// When non-empty, tools absent from the map are implicitly denied.
	ToolPolicies map[string]v1alpha1.MCPToolPolicyType `json:"-"`
}

// mcpClientManager manages connections to MCP servers and exposes their tools.
type mcpClientManager struct {
	logger   *slog.Logger
	sessions map[string]*mcpSession // keyed by server name
	mu       sync.RWMutex
}

type mcpSession struct {
	name                string
	session             *mcp.ClientSession
	allTools            []*mcp.Tool                           // all tools as reported by the server (unfiltered)
	tools               []*mcp.Tool                          // policy-filtered subset used for tool dispatch
	sanitizedToOriginal map[string]string                     // sanitized LLM name → original MCP name
	toolPolicies        map[string]v1alpha1.MCPToolPolicyType // nil = all tools with defaults; stored for probes
}

// headerTransport adds a Bearer token (pat) and any extra resolved headers to
// every outgoing request.
type headerTransport struct {
	base    http.RoundTripper
	pat     string
	headers map[string]string
}

func (h *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if h.pat != "" {
		req.Header.Set("Authorization", "Bearer "+h.pat)
	}
	for name, value := range h.headers {
		req.Header.Set(name, value)
	}
	return h.base.RoundTrip(req)
}

func newMCPClientManager(logger *slog.Logger) *mcpClientManager {
	return &mcpClientManager{
		logger:   logger,
		sessions: make(map[string]*mcpSession),
	}
}

// Connect initializes a connection to a remote MCP server.
// Transport defaults to StreamableHTTP when cfg.Transport is empty.
func (m *mcpClientManager) Connect(ctx context.Context, cfg MCPServerConfig) error {
	m.logger.Info("Connecting to MCP server", "name", cfg.Name, "url", cfg.URL, "transport", cfg.Transport)

	httpClient := &http.Client{
		Transport: &headerTransport{
			base:    http.DefaultTransport,
			pat:     cfg.Pat,
			headers: cfg.Headers,
		},
	}

	client := mcp.NewClient(
		&mcp.Implementation{Name: "mogenius-operator", Version: "v1.0.0"},
		nil,
	)

	var transport mcp.Transport
	switch cfg.Transport {
	case MCPTransportSSE:
		transport = &mcp.SSEClientTransport{
			Endpoint:   cfg.URL,
			HTTPClient: httpClient,
		}
	default:
		transport = &mcp.StreamableClientTransport{
			Endpoint:   cfg.URL,
			HTTPClient: httpClient,
		}
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server %s: %w", cfg.Name, err)
	}

	// Discover tools
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		_ = session.Close()
		return fmt.Errorf("failed to list tools from MCP server %s: %w", cfg.Name, err)
	}

	// Capture the full unfiltered list before the policy filter mutates the slice.
	allTools := make([]*mcp.Tool, len(toolsResult.Tools))
	copy(allTools, toolsResult.Tools)

	// Apply policy filter: tools with an explicit 'deny' policy or unlisted tools
	// (when toolPolicies is non-empty) are excluded from the session's tool set.
	// An empty policy map means all discovered tools are available.
	tools := toolsResult.Tools
	if len(cfg.ToolPolicies) > 0 {
		filtered := tools[:0]
		for _, tool := range tools {
			policy, ok := cfg.ToolPolicies[tool.Name]
			if !ok {
				// Unlisted tool → implicit deny when policies are configured.
				continue
			}
			if policy != v1alpha1.MCPToolPolicyDeny {
				filtered = append(filtered, tool)
			}
		}
		tools = filtered
	}

	// Build sanitized→original name mapping for LLM-safe tool names.
	nameMap := make(map[string]string, len(tools))
	for _, tool := range tools {
		nameMap[sanitizeToolName(tool.Name)] = tool.Name
	}

	m.mu.Lock()
	if old, ok := m.sessions[cfg.Name]; ok {
		if err := old.session.Close(); err != nil {
			m.logger.Warn("closing stale MCP session before reconnect", "name", cfg.Name, "error", err)
		}
	}
	m.sessions[cfg.Name] = &mcpSession{
		name:                cfg.Name,
		session:             session,
		allTools:            allTools,
		tools:               tools,
		sanitizedToOriginal: nameMap,
		toolPolicies:        cfg.ToolPolicies,
	}
	m.mu.Unlock()

	return nil
}

// RefreshSessionTools calls ListTools on an existing live session and updates
// its in-memory tool list in place. Returns the (filtered) tool names.
// Returns an error if the session does not exist or the probe fails.
func (m *mcpClientManager) RefreshSessionTools(ctx context.Context, name string) ([]string, error) {
	m.mu.RLock()
	s, ok := m.sessions[name]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %q not found", name)
	}

	result, err := s.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("ListTools probe on %q: %w", name, err)
	}

	allTools := make([]*mcp.Tool, len(result.Tools))
	copy(allTools, result.Tools)

	tools := result.Tools
	if len(s.toolPolicies) > 0 {
		filtered := tools[:0]
		for _, tool := range tools {
			policy, ok := s.toolPolicies[tool.Name]
			if ok && policy != v1alpha1.MCPToolPolicyDeny {
				filtered = append(filtered, tool)
			}
		}
		tools = filtered
	}

	nameMap := make(map[string]string, len(tools))
	for _, tool := range tools {
		nameMap[sanitizeToolName(tool.Name)] = tool.Name
	}

	m.mu.Lock()
	s.allTools = allTools
	s.tools = tools
	s.sanitizedToOriginal = nameMap
	m.mu.Unlock()

	names := make([]string, 0, len(allTools))
	for _, tool := range allTools {
		names = append(names, tool.Name)
	}
	return names, nil
}

// RemoveSession closes and removes a named session if it exists.
func (m *mcpClientManager) RemoveSession(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[name]; ok {
		if err := s.session.Close(); err != nil {
			m.logger.Warn("Error closing MCP session on removal", "name", name, "error", err)
		}
		delete(m.sessions, name)
	}
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
// toolName can be the original MCP name or its sanitized LLM-safe form.
func (m *mcpClientManager) CallTool(ctx context.Context, toolName string, args map[string]any) (string, error) {
	m.mu.RLock()
	var (
		targetSession *mcp.ClientSession
		originalName  string
	)
	for _, s := range m.sessions {
		orig := toolName
		if mapped, ok := s.sanitizedToOriginal[toolName]; ok {
			orig = mapped
		}
		for _, tool := range s.tools {
			if tool.Name == orig {
				targetSession = s.session
				originalName = orig
				break
			}
		}
		if targetSession != nil {
			break
		}
	}
	m.mu.RUnlock()

	if targetSession == nil {
		return "", fmt.Errorf("MCP tool %q not found on any connected server", toolName)
	}

	result, err := targetSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      originalName,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("MCP tool call %q failed: %w", originalName, err)
	}
	if result.IsError {
		return fmt.Sprintf("MCP tool error: %s", extractMCPText(result)), nil
	}
	return extractMCPText(result), nil
}

// HasSession returns true if a session with the given name exists.
func (m *mcpClientManager) HasSession(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.sessions[name]
	return ok
}

// IsMCPTool returns true if the tool name belongs to an MCP server.
// toolName can be the original MCP name or its sanitized LLM-safe form.
func (m *mcpClientManager) IsMCPTool(toolName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sessions {
		// Check sanitized name mapping first (most common path from LLM responses).
		if _, ok := s.sanitizedToOriginal[toolName]; ok {
			return true
		}
		// Fallback: check original names directly.
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
				Name:        sanitizeToolName(tool.Name),
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
						Name:        sanitizeToolName(tool.Name),
						Description: openai.String(tool.Description),
						Parameters:  params,
					},
				},
			})
		}
	}
	return tools
}

// GetOllamaTools returns all MCP tools in Ollama SDK format.
func (m *mcpClientManager) GetOllamaTools() []api.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []api.Tool
	for _, s := range m.sessions {
		for _, tool := range s.tools {
			props, required := mcpSchemaToOllamaProperties(tool.InputSchema)
			tools = append(tools, api.Tool{
				Type: "function",
				Function: api.ToolFunction{
					Name:        sanitizeToolName(tool.Name),
					Description: tool.Description,
					Parameters: api.ToolFunctionParameters{
						Type:       "object",
						Properties: props,
						Required:   required,
					},
				},
			})
		}
	}
	return tools
}

// sessionFilter converts a session name slice into a lookup set.
// An empty (or nil) slice returns an empty non-nil map, meaning NOTHING is
// allowed — no sessions are in scope. Callers check with !filter[name].
func sessionFilter(sessions []string) map[string]bool {
	f := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		f[s] = true
	}
	return f
}

// GetAnthropicToolsForSessions returns MCP tools in Anthropic format, scoped
// to the named sessions. An empty sessions list returns no tools.
func (m *mcpClientManager) GetAnthropicToolsForSessions(sessions []string) []anthropic.ToolParam {
	filter := sessionFilter(sessions)
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []anthropic.ToolParam
	for _, s := range m.sessions {
		if !filter[s.name] {
			continue
		}
		for _, tool := range s.tools {
			properties, required := mcpSchemaToPropertiesAndRequired(tool.InputSchema)
			tools = append(tools, anthropic.ToolParam{
				Name:        sanitizeToolName(tool.Name),
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

// GetOpenAIToolsForSessions returns MCP tools in OpenAI format, scoped to the
// named sessions. An empty sessions list returns no tools.
func (m *mcpClientManager) GetOpenAIToolsForSessions(sessions []string) []openai.ChatCompletionToolUnionParam {
	filter := sessionFilter(sessions)
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []openai.ChatCompletionToolUnionParam
	for _, s := range m.sessions {
		if !filter[s.name] {
			continue
		}
		for _, tool := range s.tools {
			params := mcpSchemaToFunctionParams(tool.InputSchema)
			tools = append(tools, openai.ChatCompletionToolUnionParam{
				OfFunction: &openai.ChatCompletionFunctionToolParam{
					Function: openai.FunctionDefinitionParam{
						Name:        sanitizeToolName(tool.Name),
						Description: openai.String(tool.Description),
						Parameters:  params,
					},
				},
			})
		}
	}
	return tools
}

// GetOllamaToolsForSessions returns MCP tools in Ollama format, scoped to the
// named sessions. An empty sessions list returns no tools.
func (m *mcpClientManager) GetOllamaToolsForSessions(sessions []string) []api.Tool {
	filter := sessionFilter(sessions)
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []api.Tool
	for _, s := range m.sessions {
		if !filter[s.name] {
			continue
		}
		for _, tool := range s.tools {
			props, required := mcpSchemaToOllamaProperties(tool.InputSchema)
			tools = append(tools, api.Tool{
				Type: "function",
				Function: api.ToolFunction{
					Name:        sanitizeToolName(tool.Name),
					Description: tool.Description,
					Parameters: api.ToolFunctionParameters{
						Type:       "object",
						Properties: props,
						Required:   required,
					},
				},
			})
		}
	}
	return tools
}

// IsMCPToolInSessions returns true when toolName belongs to one of the named
// sessions. An empty sessions list returns false (no sessions in scope).
func (m *mcpClientManager) IsMCPToolInSessions(toolName string, sessions []string) bool {
	if len(sessions) == 0 {
		return false
	}
	filter := sessionFilter(sessions)
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sessions {
		if !filter[s.name] {
			continue
		}
		if _, ok := s.sanitizedToOriginal[toolName]; ok {
			return true
		}
		for _, tool := range s.tools {
			if tool.Name == toolName {
				return true
			}
		}
	}
	return false
}

// MCPToolNeedsApproval reports whether the tool's effective policy is
// NeedsApprove across the named sessions. Callers use this to gate execution
// through an approval flow before calling CallToolInSessions.
func (m *mcpClientManager) MCPToolNeedsApproval(toolName string, sessions []string) bool {
	if len(sessions) == 0 {
		return false
	}
	filter := sessionFilter(sessions)
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.sessions {
		if !filter[s.name] {
			continue
		}
		originalName := toolName
		if orig, ok := s.sanitizedToOriginal[toolName]; ok {
			originalName = orig
		}
		for _, tool := range s.tools {
			if tool.Name == originalName {
				if len(s.toolPolicies) == 0 {
					return false
				}
				policy, ok := s.toolPolicies[originalName]
				if !ok {
					return false
				}
				return policy == v1alpha1.MCPToolPolicyNeedsApprove
			}
		}
	}
	return false
}

// CallToolInSessions executes the named tool in one of the named sessions.
// It does not enforce approval policy — callers are responsible for calling
// MCPToolNeedsApproval and obtaining approval before invoking this function.
// Denied tools return an error.
func (m *mcpClientManager) CallToolInSessions(ctx context.Context, toolName string, args map[string]any, sessions []string) (string, error) {
	if len(sessions) == 0 {
		return "", fmt.Errorf("MCP tool %q: no sessions in scope", toolName)
	}
	filter := sessionFilter(sessions)

	m.mu.RLock()
	var (
		targetSession *mcp.ClientSession
		originalName  string
		denied        bool
	)
	for _, s := range m.sessions {
		if !filter[s.name] {
			continue
		}
		orig := toolName
		if mapped, ok := s.sanitizedToOriginal[toolName]; ok {
			orig = mapped
		}
		for _, tool := range s.tools {
			if tool.Name == orig {
				if len(s.toolPolicies) > 0 {
					policy, ok := s.toolPolicies[orig]
					if !ok {
						policy = v1alpha1.MCPToolPolicyDeny
					}
					if policy == v1alpha1.MCPToolPolicyDeny {
						denied = true
						originalName = orig
						break
					}
				}
				targetSession = s.session
				originalName = orig
				break
			}
		}
		if targetSession != nil || denied {
			break
		}
	}
	m.mu.RUnlock()

	if denied {
		return "", fmt.Errorf("MCP tool %q is denied by policy", originalName)
	}
	if targetSession == nil {
		return "", fmt.Errorf("MCP tool %q not found in requested sessions", toolName)
	}

	result, err := targetSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      originalName,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("MCP tool call %q failed: %w", originalName, err)
	}
	if result.IsError {
		return fmt.Sprintf("MCP tool error: %s", extractMCPText(result)), nil
	}
	return extractMCPText(result), nil
}

// GetToolsWithPolicies returns every tool reported by the server (including
// denied ones) together with its effective execution policy. Used by the
// reconciler to populate McpServerStatus.ToolsWithPolicies.
//
// Policy resolution:
//   - Non-empty toolPolicies map: look up each tool; unlisted tools get Deny.
//   - Empty toolPolicies map: derive from readOnlyHint — read-only →
//     AutoApprove, mutating → NeedsApprove.
func (m *mcpClientManager) GetToolsWithPolicies(sessionName string) []v1alpha1.MCPToolWithPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionName]
	if !ok {
		return nil
	}
	result := make([]v1alpha1.MCPToolWithPolicy, 0, len(s.allTools))
	for _, tool := range s.allTools {
		entry := v1alpha1.MCPToolWithPolicy{Name: tool.Name}
		if len(s.toolPolicies) > 0 {
			policy, ok := s.toolPolicies[tool.Name]
			if !ok {
				policy = v1alpha1.MCPToolPolicyDeny
			}
			entry.Policy = policy
		} else {
			// No explicit policies: derive from readOnlyHint.
			readOnly := tool.Annotations != nil && tool.Annotations.ReadOnlyHint
			entry.ReadOnly = readOnly
			if readOnly {
				entry.Policy = v1alpha1.MCPToolPolicyAutoApprove
			} else {
				entry.Policy = v1alpha1.MCPToolPolicyNeedsApprove
			}
		}
		result = append(result, entry)
	}
	return result
}

// mcpSchemaToOllamaProperties converts an MCP tool's InputSchema to the
// properties map and required slice used by the Ollama SDK.
func mcpSchemaToOllamaProperties(schema any) (*api.ToolPropertiesMap, []string) {
	m, ok := schema.(map[string]any)
	if !ok || m == nil {
		return api.NewToolPropertiesMap(), nil
	}

	properties := api.NewToolPropertiesMap()
	if props, ok := m["properties"].(map[string]any); ok {
		for k, v := range props {
			propMap, ok := v.(map[string]any)
			if !ok {
				continue
			}
			tp := api.ToolProperty{}
			if t, ok := propMap["type"].(string); ok {
				tp.Type = []string{t}
			}
			if d, ok := propMap["description"].(string); ok {
				tp.Description = d
			}
			if enum, ok := propMap["enum"].([]any); ok {
				for _, e := range enum {
					if s, ok := e.(string); ok {
						tp.Enum = append(tp.Enum, s)
					}
				}
			}
			properties.Set(k, tp)
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
	var resultText strings.Builder
	resultText.WriteString(texts[0])
	for i := 1; i < len(texts); i++ {
		resultText.WriteString("\n")
		resultText.WriteString(texts[i])
	}
	return resultText.String()
}

// mcpSchemaToPropertiesAndRequired converts an MCP tool's InputSchema (any / map[string]any)
// to the properties map and required slice used by the Anthropic SDK.
func mcpSchemaToPropertiesAndRequired(schema any) (map[string]any, []string) {
	m, ok := schema.(map[string]any)
	if !ok || m == nil {
		return map[string]any{}, nil
	}

	properties := make(map[string]any)
	if props, ok := m["properties"].(map[string]any); ok {
		maps.Copy(properties, props)
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
			"properties": map[string]any{},
		}
	}

	params := openai.FunctionParameters{
		"type": "object",
	}

	if props, ok := m["properties"].(map[string]any); ok {
		params["properties"] = props
	} else {
		params["properties"] = map[string]any{}
	}

	if req, ok := m["required"].([]any); ok {
		params["required"] = req
	}

	return params
}
