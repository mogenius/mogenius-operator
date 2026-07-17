package ai

import (
	"log/slog"
	map0 "maps"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ToolContext struct {
	AllowedNamespaces   map[string]bool            // namespaces with full access (from type="namespace"); nil = no restriction
	AllowedHelmReleases map[string]map[string]bool // namespace → {releaseName: true} (from type="helm"); nil = no helm-level restriction
	AllowedArgoCDApps   map[string]bool            // ArgoCD app names (from type="argocd"); nil = no argocd-level restriction
	Role                string                     // "viewer", "editor", "admin", "" = no restriction

	// Audit attribution — who triggered this tool call. Nil User means the
	// unattended insight path (which only ever gets read-only tools).
	User      *structs.User
	Workspace string

	// AuditSource overrides the audit log source for mutating tool calls;
	// empty defaults to "ai-chat" (the interactive chat path).
	AuditSource string

	// RequireActionableFindings makes submit_analysis reject findings without
	// an applicable structured proposal at submission time, so the model can
	// fix them in-conversation instead of having them silently dropped after
	// the run (whole-scope agent runs only).
	RequireActionableFindings bool
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
	return newToolContextFromUserGrant(ioChannel.User, ioChannel.Workspace, ioChannel.IsAdmin, ioChannel.WorkspaceSpec, ioChannel.WorkspaceGrant)
}

// newToolContextFromUserGrant builds a ToolContext from a user's workspace
// grant. Shared by the chat path and the task-approval execution path so both
// enforce identical scoping.
func newToolContextFromUserGrant(user *structs.User, workspace string, isAdmin bool, workspaceSpec *v1alpha1.WorkspaceSpec, workspaceGrant *v1alpha1.GrantSpec) *ToolContext {
	tc := &ToolContext{
		User:      user,
		Workspace: workspace,
	}

	if isAdmin || (workspaceSpec == nil && workspaceGrant == nil) {
		// No restrictions: an empty Role and nil allow-maps behave exactly
		// like the former nil ToolContext in every permission check; the
		// context now only carries audit attribution.
		return tc
	}

	if workspaceGrant != nil {
		tc.Role = workspaceGrant.Role
	}

	if workspaceSpec != nil && len(workspaceSpec.Resources) > 0 {
		for _, res := range workspaceSpec.Resources {
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

// newToolContextFromAgent builds the read-only ToolContext an agent's
// unattended analysis runs under. The role is always "viewer" (an empty Role
// would pass IsEditor/IsAdmin) and the namespace allow-map must be non-empty —
// callers must not run an agent whose scope resolved to zero namespaces.
func newToolContextFromAgent(agent *v1alpha1.Agent, resolvedNamespaces []string) *ToolContext {
	allowed := make(map[string]bool, len(resolvedNamespaces))
	for _, ns := range resolvedNamespaces {
		if ns != "" {
			allowed[ns] = true
		}
	}
	return &ToolContext{
		AllowedNamespaces: allowed,
		Role:              "viewer",
		User: &structs.User{
			FirstName: "Agent",
			LastName:  agent.Name,
			Email:     "agent:" + agent.Name + "@system",
			Source:    "ai-agent",
		},
		Workspace: agent.Spec.Scope.WorkspaceRef,
	}
}

// auditAiToolMutation writes a resource-indexed audit log entry for a
// mutating AI tool call — same key scheme, diff handling and sanitization
// as the socketapi handlers, so "who changed resource X" queries also find
// AI-driven changes. It runs synchronously inside the tool: the mutation is
// audited even when the chat turn aborts before its summary entry.
func auditAiToolMutation(tc *ToolContext, logger *slog.Logger, toolName string, args map[string]any, result any, err error, oldObj, newObj *unstructured.Unstructured) {
	// "no unattributed actions": fall back to the insight system user when
	// no chat user is present (defense in depth — the insight path only
	// offers read-only tools).
	user := structs.User{FirstName: "AI", LastName: "Insights", Email: "ai-insights@system", Source: "ai-insights"}
	workspace := ""
	if tc != nil {
		if tc.User != nil {
			user = *tc.User
			user.Source = "ai-chat"
			if tc.AuditSource != "" {
				user.Source = tc.AuditSource
			}
		}
		workspace = tc.Workspace
	}
	datagram := structs.Datagram{
		Id:        utils.NanoId(),
		Pattern:   "ai/tool/" + toolName,
		Payload:   args,
		CreatedAt: time.Now(),
		User:      user,
		Workspace: workspace,
	}
	_, _ = store.AddToAuditLog(datagram, logger, result, err, oldObj, newObj)
}

func mergeAnnotationsAndLabels(annotations, labels map[string]string) map[string]string {
	merged := make(map[string]string, len(annotations)+len(labels))
	map0.Copy(merged, annotations)
	map0.Copy(merged, labels)
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

// readOnly*Tools restrict a tool set to the read-only viewer allowlist. Used
// by the automatic insight path, which runs unattended and without a
// ToolContext — a nil ToolContext passes every role/namespace check, so
// write tools (and external MCP tools) must never be offered to the model
// there.
func readOnlyAnthropicTools(tools []anthropic.ToolParam) []anthropic.ToolParam {
	filtered := make([]anthropic.ToolParam, 0, len(tools))
	for _, t := range tools {
		if viewerAllowedTools[t.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func readOnlyOpenAiTools(tools []openai.ChatCompletionToolUnionParam) []openai.ChatCompletionToolUnionParam {
	filtered := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, t := range tools {
		if t.OfFunction != nil && viewerAllowedTools[t.OfFunction.Function.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func readOnlyOllamaTools(tools []api.Tool) []api.Tool {
	filtered := make([]api.Tool, 0, len(tools))
	for _, t := range tools {
		if viewerAllowedTools[t.Function.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
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
		map0.Copy(result, m)
	}
	return result
}
