package ai

import (
	"context"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"time"
)

// NotifyMcpServerChanged reconnects the named McpServer CR and returns the names
// of tools discovered from the server. Called by the reconciler whenever a
// McpServer CR is created or updated.
func (ai *aiManager) NotifyMcpServerChanged(name string) ([]string, error) {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		return nil, fmt.Errorf("NotifyMcpServerChanged: %w", err)
	}

	tool, err := store.GetMcpServer(ownNamespace, name)
	if err != nil || tool == nil {
		return nil, fmt.Errorf("McpServer %q not found: %w", name, err)
	}

	cfg, err := ai.buildMcpConfig(tool)
	if err != nil {
		return nil, fmt.Errorf("McpServer %q: resolve config: %w", name, err)
	}

	// Remove any stale session before reconnecting.
	ai.mcpManager.RemoveSession(name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ai.mcpManager.Connect(ctx, cfg); err != nil {
		return nil, fmt.Errorf("McpServer %q: connect: %w", name, err)
	}

	return ai.mcpManager.discoveredToolNames(name), nil
}

// NotifyMcpServerDeleted removes the session for the named McpServer CR. Called
// by the reconciler when a McpServer CR is deleted.
func (ai *aiManager) NotifyMcpServerDeleted(name string) {
	ai.mcpManager.RemoveSession(name)
}

// RefreshAllMcpServerCRConnections connects to every McpServer CR in the operator
// namespace. Called once at startup after the store is seeded. Errors per
// server are logged but do not abort the others.
func (ai *aiManager) RefreshAllMcpServerCRConnections() {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		ai.logger.Warn("RefreshAllMcpServerCRConnections: failed to get own namespace", "error", err)
		return
	}
	tools, err := store.GetAllMcpServers(ownNamespace)
	if err != nil {
		ai.logger.Warn("RefreshAllMcpServerCRConnections: failed to list McpServers", "error", err)
		return
	}
	for _, tool := range tools {
		cfg, err := ai.buildMcpConfig(&tool)
		if err != nil {
			ai.logger.Warn("McpServer: skipping (resolve error)", "name", tool.Name, "error", err)
			continue
		}
		ai.mcpManager.RemoveSession(tool.Name)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := ai.mcpManager.Connect(ctx, cfg); err != nil {
			ai.logger.Warn("McpServer: failed to connect", "name", tool.Name, "error", err)
		} else {
			ai.logger.Info("McpServer: connected", "name", tool.Name)
		}
		cancel()
	}
}

// buildMcpConfig resolves secrets and returns an MCPServerConfig for a McpServer CR.
func (ai *aiManager) buildMcpConfig(tool *v1alpha1.McpServer) (MCPServerConfig, error) {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		return MCPServerConfig{}, err
	}

	headers := make(map[string]string)

	// Resolve auth.
	if a := tool.Spec.Auth; a != nil && a.Type != v1alpha1.McpAuthNone {
		if a.SecretRef == nil {
			return MCPServerConfig{}, fmt.Errorf("auth type %q requires secretRef", a.Type)
		}
		token, err := ai.resolveSecretValue(ownNamespace, a.SecretRef.Name, a.SecretRef.Key)
		if err != nil {
			return MCPServerConfig{}, fmt.Errorf("auth secretRef: %w", err)
		}
		switch a.Type {
		case v1alpha1.McpAuthBearer:
			headers["Authorization"] = "Bearer " + token
		case v1alpha1.McpAuthAPIKey:
			headers[a.Header] = token
		}
	}

	// Resolve extra headers.
	for _, h := range tool.Spec.Headers {
		if h.ValueFrom != nil {
			val, err := ai.resolveSecretValue(ownNamespace, h.ValueFrom.Name, h.ValueFrom.Key)
			if err != nil {
				return MCPServerConfig{}, fmt.Errorf("header %q valueFrom: %w", h.Name, err)
			}
			headers[h.Name] = val
		} else {
			headers[h.Name] = h.Value
		}
	}

	// Build allowlist.
	var allowedTools map[string]bool
	if len(tool.Spec.AllowedTools) > 0 {
		allowedTools = make(map[string]bool, len(tool.Spec.AllowedTools))
		for _, t := range tool.Spec.AllowedTools {
			allowedTools[t] = true
		}
	}

	var transport MCPTransportType
	switch tool.Spec.Transport {
	case v1alpha1.McpTransportSSE:
		transport = MCPTransportSSE
	default:
		transport = MCPTransportStreamableHTTP
	}

	return MCPServerConfig{
		Name:         tool.Name,
		URL:          tool.Spec.URL,
		Transport:    transport,
		Headers:      headers,
		AllowedTools: allowedTools,
	}, nil
}

// resolveSecretValue fetches one key from a Secret in the given namespace.
func (ai *aiManager) resolveSecretValue(namespace, secretName, key string) (string, error) {
	if ai.secretGetter == nil {
		return "", fmt.Errorf("no secret getter configured")
	}
	secret, err := ai.secretGetter(namespace, secretName)
	if err != nil {
		return "", fmt.Errorf("secret %s/%s: %w", namespace, secretName, err)
	}
	if secret == nil {
		return "", fmt.Errorf("secret %s/%s not found", namespace, secretName)
	}
	k := key
	if k == "" {
		k = "API_KEY"
	}
	val, ok := secret.Data[k]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %s/%s", k, namespace, secretName)
	}
	return string(val), nil
}

// discoveredToolNames returns the list of tool names in the named session.
func (m *mcpClientManager) discoveredToolNames(sessionName string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionName]
	if !ok {
		return nil
	}
	names := make([]string, 0, len(s.tools))
	for _, tool := range s.tools {
		names = append(names, tool.Name)
	}
	return names
}
