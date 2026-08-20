package ai

import (
	"context"
	"log/slog"
	map0 "maps"
	"mogenius-operator/src/ai/aisdk"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"strings"
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
	AllowedFluxReleases map[string]bool            // Flux owner keys (from type="flux", see fluxOwnerKey); nil = no flux-level restriction
	Role                string                     // "viewer", "editor", "admin", "" = no restriction

	// Audit attribution — who triggered this tool call. Nil User means the
	// unattended insight path (which only ever gets read-only tools).
	User      *structs.User
	Workspace string

	// AuditSource overrides the audit log source for mutating tool calls;
	// empty defaults to "ai-chat" (the interactive chat path).
	AuditSource string

	// ExcludeResources holds resource identities that already have an open
	// proposal from an earlier run. They are filtered out of list results and
	// refused by get so a whole-scope run neither re-inspects nor re-reports
	// what a user has not decided on yet — saving the tokens of re-analysis.
	ExcludeResources map[string]bool

	// McpSessions lists the McpServer CR names whose tools are available to this
	// agent run. An empty slice means no CRD-defined MCP servers are connected.
	McpSessions []string

	// DisableKubernetes removes the built-in Kubernetes tool group for this run.
	// Zero value (false) keeps the tools enabled — safe default for all paths.
	DisableKubernetes bool

	// DisableHelm removes the built-in Helm tool group for this run.
	// Zero value (false) keeps the tools enabled — safe default for all paths.
	DisableHelm bool

	// InterceptMutatingTools makes the built-in mutating K8s/Helm tools available
	// to the model but intercepts them before execution, creating an approval
	// request instead. Used for agent runs so agents can propose K8s mutations
	// without needing elevated RBAC — the approving user supplies the permissions.
	InterceptMutatingTools bool

	// CreateApprovalRequest is populated for agent runs. When a tool call is
	// intercepted (mutating built-in or MCP needsApprove), this callback
	// persists a PROPOSED task and blocks until the user approves or rejects
	// it. On approval it returns the approver's ToolContext (preserving the
	// agent's namespace scope but carrying the approver's role and identity),
	// or an error when rejected or the context is cancelled.
	CreateApprovalRequest func(ctx context.Context, toolName string, args map[string]any) (*ToolContext, error)
}

// mutatingBuiltinTools is the set of built-in tool names that mutate Kubernetes
// resources. When InterceptMutatingTools is true these are offered to the model
// but intercepted before execution via CreateApprovalRequest.
var mutatingBuiltinTools = map[string]bool{
	"update_kubernetes_resource": true,
	"create_kubernetes_resource": true,
	"delete_kubernetes_resource": true,
}

// aiResourceKey is the canonical identity used for ExcludeResources lookups.
func aiResourceKey(apiVersion, kind, namespace, name string) string {
	return apiVersion + "|" + kind + "|" + namespace + "|" + name
}

// IsResourceExcluded reports whether a resource already has an open proposal.
func (tc *ToolContext) IsResourceExcluded(apiVersion, kind, namespace, name string) bool {
	if tc == nil || len(tc.ExcludeResources) == 0 {
		return false
	}
	return tc.ExcludeResources[aiResourceKey(apiVersion, kind, namespace, name)]
}

// fluxOwnerKey is the lookup key for AllowedFluxReleases: the identity of the
// owning Flux CR. kind is "Kustomization" or "HelmRelease" — the two kinds
// whose controllers stamp ownership labels
// (kustomize|helm.toolkit.fluxcd.io/name|namespace) on applied objects.
func fluxOwnerKey(kind, namespace, name string) string {
	return kind + "\x00" + namespace + "\x00" + name
}

// hasRestrictions returns true when any scoping is configured.
func (tc *ToolContext) hasRestrictions() bool {
	if tc == nil {
		return false
	}
	return tc.AllowedNamespaces != nil || tc.AllowedHelmReleases != nil || tc.AllowedArgoCDApps != nil || tc.AllowedFluxReleases != nil
}

// hasOwnershipRestrictions reports whether a resource outside an allowed
// namespace could still be allowed via GitOps ownership (ArgoCD instance
// annotation or Flux ownership labels). Callers that would otherwise reject a
// namespace early must then defer the decision to IsResourceAllowed on the
// concrete object.
func (tc *ToolContext) hasOwnershipRestrictions() bool {
	if tc == nil {
		return false
	}
	return tc.AllowedArgoCDApps != nil || tc.AllowedFluxReleases != nil
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
	// Check Flux ownership (labels stamped by kustomize-/helm-controller on
	// every object they apply).
	if tc.AllowedFluxReleases != nil {
		if name := annotations["kustomize.toolkit.fluxcd.io/name"]; name != "" {
			ns := annotations["kustomize.toolkit.fluxcd.io/namespace"]
			if tc.AllowedFluxReleases[fluxOwnerKey("Kustomization", ns, name)] {
				return true
			}
		}
		if name := annotations["helm.toolkit.fluxcd.io/name"]; name != "" {
			ns := annotations["helm.toolkit.fluxcd.io/namespace"]
			if tc.AllowedFluxReleases[fluxOwnerKey("HelmRelease", ns, name)] {
				return true
			}
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
			case "flux":
				// Workspace resource id is "<Kind>/<name>", namespace is the
				// CR's namespace — matching the ownership labels Flux stamps
				// on applied objects (see fluxOwnerKey).
				kind, name, found := strings.Cut(res.Id, "/")
				if found && name != "" && res.Namespace != "" {
					if tc.AllowedFluxReleases == nil {
						tc.AllowedFluxReleases = make(map[string]bool)
					}
					tc.AllowedFluxReleases[fluxOwnerKey(kind, res.Namespace, name)] = true
				}
			}
		}
	}

	return tc
}

// newToolContextFromAgent builds the ToolContext for agent runs. No role is set
// because mutating tools are gated by InterceptMutatingTools + CreateApprovalRequest
// (wired in buildAgentTaskContext), not by the role check in the tools themselves.
// The namespace allow-map must be non-empty — callers must not run an agent
// whose scope resolved to zero namespaces.
func newToolContextFromAgent(agent *v1alpha1.Agent, resolvedNamespaces []string) *ToolContext {
	allowed := make(map[string]bool, len(resolvedNamespaces))
	for _, ns := range resolvedNamespaces {
		if ns != "" {
			allowed[ns] = true
		}
	}
	return &ToolContext{
		Role:                   "viewer",
		AllowedNamespaces:      allowed,
		InterceptMutatingTools: true,
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

func filterAiSDKTools(tools []aisdk.Tool, ioChannel IOChatChannel) []aisdk.Tool {
	if !isViewerRole(ioChannel) {
		return tools
	}
	filtered := make([]aisdk.Tool, 0, len(tools))
	for _, t := range tools {
		if !viewerAllowedTools[t.Name] {
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
