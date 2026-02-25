package ai

import (
	"log/slog"
	"mogenius-operator/src/valkeyclient"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
)

type ToolContext struct {
	AllowedNamespaces   map[string]bool            // namespaces with full access (from type="namespace"); nil = no restriction
	AllowedHelmReleases map[string]map[string]bool // namespace → {releaseName: true} (from type="helm"); nil = no helm-level restriction
	AllowedArgoCDApps   map[string]bool            // ArgoCD app names (from type="argocd"); nil = no argocd-level restriction
	Role                string                     // "viewer", "editor", "admin", "" = no restriction
}

// hasRestrictions returns true when any scoping is configured.
func (tc *ToolContext) hasRestrictions() bool {
	if tc == nil {
		return false
	}
	return tc.AllowedNamespaces != nil || tc.AllowedHelmReleases != nil || tc.AllowedArgoCDApps != nil
}

func (tc *ToolContext) IsNamespaceAllowed(namespace string) bool {
	if tc == nil || !tc.hasRestrictions() {
		return true
	}
	if tc.AllowedNamespaces != nil && tc.AllowedNamespaces[namespace] {
		return true
	}
	if tc.AllowedHelmReleases != nil {
		if _, ok := tc.AllowedHelmReleases[namespace]; ok {
			return true
		}
	}
	return false
}

func (tc *ToolContext) IsHelmReleaseAllowed(namespace, releaseName string) bool {
	if tc == nil {
		return true
	}
	// Full namespace access includes all releases
	if tc.AllowedNamespaces != nil && tc.AllowedNamespaces[namespace] {
		return true
	}
	// No helm-level restriction configured
	if tc.AllowedHelmReleases == nil {
		return true
	}
	releases, ok := tc.AllowedHelmReleases[namespace]
	if !ok {
		return false
	}
	return releases[releaseName]
}

func (tc *ToolContext) IsResourceAllowed(namespace string, annotations map[string]string) bool {
	if tc == nil || !tc.hasRestrictions() {
		return true
	}
	// Full namespace access
	if tc.AllowedNamespaces != nil && tc.AllowedNamespaces[namespace] {
		return true
	}
	// Check helm release ownership
	if tc.AllowedHelmReleases != nil {
		if releases, ok := tc.AllowedHelmReleases[namespace]; ok {
			releaseName := annotations["meta.helm.sh/release-name"]
			if releaseName != "" && releases[releaseName] {
				return true
			}
		}
	}
	// Check ArgoCD app ownership
	if tc.AllowedArgoCDApps != nil {
		appName := annotations["argocd.argoproj.io/instance"]
		if appName != "" && tc.AllowedArgoCDApps[appName] {
			return true
		}
	}
	return false
}

func (tc *ToolContext) IsEditor() bool {
	if tc == nil || tc.Role == "" {
		return true
	}
	return tc.Role == "editor"
}

func (tc *ToolContext) IsAdmin() bool {
	if tc == nil || tc.Role == "" {
		return true
	}
	return tc.Role == "admin"
}

func newToolContextFromIOChannel(ioChannel IOChatChannel) *ToolContext {
	if ioChannel.IsAdmin || (ioChannel.WorkspaceSpec == nil && ioChannel.WorkspaceGrant == nil) {
		return nil
	}

	tc := &ToolContext{}

	if ioChannel.WorkspaceGrant != nil {
		tc.Role = ioChannel.WorkspaceGrant.Role
	}

	if ioChannel.WorkspaceSpec != nil && len(ioChannel.WorkspaceSpec.Resources) > 0 {
		for _, res := range ioChannel.WorkspaceSpec.Resources {
			switch res.Type {
			case "namespace":
				if res.Id != "" {
					if tc.AllowedNamespaces == nil {
						tc.AllowedNamespaces = make(map[string]bool)
					}
					tc.AllowedNamespaces[res.Id] = true
				}
			case "helm":
				if res.Namespace != "" && res.Id != "" {
					if tc.AllowedHelmReleases == nil {
						tc.AllowedHelmReleases = make(map[string]map[string]bool)
					}
					if tc.AllowedHelmReleases[res.Namespace] == nil {
						tc.AllowedHelmReleases[res.Namespace] = make(map[string]bool)
					}
					tc.AllowedHelmReleases[res.Namespace][res.Id] = true
				}
			case "argocd":
				if res.Id != "" {
					if tc.AllowedArgoCDApps == nil {
						tc.AllowedArgoCDApps = make(map[string]bool)
					}
					tc.AllowedArgoCDApps[res.Id] = true
				}
			}
		}
	}

	return tc
}

func mergeAnnotationsAndLabels(annotations, labels map[string]string) map[string]string {
	merged := make(map[string]string, len(annotations)+len(labels))
	for k, v := range annotations {
		merged[k] = v
	}
	for k, v := range labels {
		merged[k] = v
	}
	return merged
}

type toolHandler = func(map[string]any, *ToolContext, valkeyclient.ValkeyClient, *slog.Logger) string

// toolDefinitions is the combined registry of all AI tool handlers.
var toolDefinitions = mergeToolMaps(
	kubernetesToolDefinitions,
	helmToolDefinitions,
)

// viewerAllowedTools contains the only tools a viewer is allowed to use.
var viewerAllowedTools = map[string]bool{
	"get_kubernetes_resources":  true,
	"list_kubernetes_resources": true,
	"check_kubernetes_resource": true,
	"get_pod_logs":              true,
	"get_pod_events":            true,
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
