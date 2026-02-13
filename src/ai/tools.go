package ai

import (
	"log/slog"
	"mogenius-operator/src/valkeyclient"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
)

type toolHandler = func(map[string]any, valkeyclient.ValkeyClient, *slog.Logger) string

// toolDefinitions is the combined registry of all AI tool handlers.
var toolDefinitions = mergeToolMaps(
	kubernetesToolDefinitions,
	helmToolDefinitions,
)

// viewerAllowedTools contains the only tools a viewer is allowed to use.
var viewerAllowedTools = map[string]bool{
	"get_kubernetes_resources":  true,
	"list_kubernetes_resources": true,
	// helm tools
	"helm_chart_search":    true,
	"helm_chart_show":      true,
	"helm_chart_versions":  true,
	"helm_repo_list":       true,
	"helm_release_list":    true,
	"helm_release_get":     true,
	"helm_release_history": true,
	"helm_release_status":  true,
}

func isViewerRole(ioChannel IOChatChannel) bool {
	return ioChannel.WorkspaceGrant != nil && ioChannel.WorkspaceGrant.Role == "viewer"
}

func filterOpenAiTools(tools []openai.ChatCompletionToolUnionParam, ioChannel IOChatChannel) []openai.ChatCompletionToolUnionParam {
	if !isViewerRole(ioChannel) {
		return tools
	}
	filtered := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, t := range tools {
		if t.OfFunction != nil && !viewerAllowedTools[t.OfFunction.Function.Name] {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}

func filterAnthropicTools(tools []anthropic.ToolParam, ioChannel IOChatChannel) []anthropic.ToolParam {
	if !isViewerRole(ioChannel) {
		return tools
	}
	filtered := make([]anthropic.ToolParam, 0, len(tools))
	for _, t := range tools {
		if !viewerAllowedTools[t.Name] {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}

func filterOllamaTools(tools []api.Tool, ioChannel IOChatChannel) []api.Tool {
	if !isViewerRole(ioChannel) {
		return tools
	}
	filtered := make([]api.Tool, 0, len(tools))
	for _, t := range tools {
		if !viewerAllowedTools[t.Function.Name] {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}

func mergeToolMaps(maps ...map[string]toolHandler) map[string]toolHandler {
	result := make(map[string]toolHandler)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
