package ai

import "mogenius-operator/src/ai/aisdk"

// kubernetesAiSDKTools defines the built-in Kubernetes tools in the
// provider-neutral aisdk.Tool format. Each provider converts them to its own
// SDK type via toolsToAnthropic / toolsToOpenAI / toolsToOllama.
var kubernetesAiSDKTools = []aisdk.Tool{
	{
		Name:        "get_kubernetes_resources",
		Description: "Get details of a specific Kubernetes resource by kind, name and namespace. Cost ladder: use list_kubernetes_resources for discovery, detail=summary here for triage, and detail=full ONLY when you need the complete manifest (e.g. to build an UpdateResource proposal).",
		InputSchema: map[string]any{
			"apiVersion": prop("string", "API version (e.g. 'v1', 'apps/v1')"),
			"kind":       prop("string", "Resource kind (e.g. 'Pod', 'Deployment')"),
			"name":       prop("string", "Resource name"),
			"namespace":  prop("string", "Namespace (optional for cluster-scoped)"),
			"detail":     prop("string", "'summary' (recommended, ~1/3 of the tokens): identity, ownership, condensed status, key spec fields. 'full' (default): complete manifest", "summary", "full"),
		},
		Required: []string{"kind", "apiVersion", "name"},
	},
	{
		Name:        "list_kubernetes_resources",
		Description: "List Kubernetes resources of a specific kind as compact summaries. Omit the namespace to list across ALL namespaces in one call — strongly preferred for cluster-wide sweeps; never iterate namespace by namespace.",
		InputSchema: map[string]any{
			"apiVersion": prop("string", "API version (e.g. 'v1', 'apps/v1')"),
			"kind":       prop("string", "Resource kind (e.g. 'Pod', 'Deployment')"),
			"namespace":  prop("string", "Namespace filter. Omit to list across all namespaces (preferred)"),
		},
		Required: []string{"kind", "apiVersion"},
	},
	{
		Name:        "check_kubernetes_resource",
		Description: "Check existence and status of a single resource. Returns a compact summary instead of full details. Use get_kubernetes_resources only when you need the complete resource object.",
		InputSchema: map[string]any{
			"apiVersion": prop("string", "API version (e.g. 'v1', 'apps/v1')"),
			"kind":       prop("string", "Resource kind (e.g. 'Pod', 'Deployment')"),
			"name":       prop("string", "Resource name"),
			"namespace":  prop("string", "Namespace (optional for cluster-scoped)"),
		},
		Required: []string{"kind", "apiVersion", "name"},
	},
	{
		Name:        "update_kubernetes_resource",
		Description: "Update an existing Kubernetes resource with new YAML configuration.",
		InputSchema: map[string]any{
			"apiVersion": prop("string", "API version (e.g. 'v1', 'apps/v1')"),
			"plural":     prop("string", "Plural name (e.g. 'pods', 'deployments')"),
			"namespaced": prop("boolean", "Namespaced (true) or cluster-scoped (false)"),
			"yamlData":   prop("string", "Complete YAML definition of the resource"),
		},
		Required: []string{"apiVersion", "plural", "namespaced", "yamlData"},
	},
	{
		Name:        "delete_kubernetes_resource",
		Description: "Delete a Kubernetes resource by name and namespace.",
		InputSchema: map[string]any{
			"apiVersion": prop("string", "API version (e.g. 'v1', 'apps/v1')"),
			"plural":     prop("string", "Plural name (e.g. 'pods', 'deployments')"),
			"namespace":  prop("string", "Namespace (empty for cluster-scoped)"),
			"name":       prop("string", "Resource name to delete"),
		},
		Required: []string{"apiVersion", "plural", "name"},
	},
	{
		Name:        "create_kubernetes_resource",
		Description: "Create a new Kubernetes resource from YAML configuration.",
		InputSchema: map[string]any{
			"apiVersion": prop("string", "API version (e.g. 'v1', 'apps/v1')"),
			"plural":     prop("string", "Plural name (e.g. 'pods', 'deployments')"),
			"namespaced": prop("boolean", "Namespaced (true) or cluster-scoped (false)"),
			"yamlData":   prop("string", "Complete YAML definition of the resource"),
		},
		Required: []string{"apiVersion", "plural", "namespaced", "yamlData"},
	},
	{
		Name:        "get_pod_logs",
		Description: "Get logs from a pod's container. Returns the last N lines of log output. The response is automatically trimmed to fit within maxChars. Start with a small maxChars and increase only if you need more context.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Namespace of the pod"),
			"podName":   prop("string", "Name of the pod"),
			"container": prop("string", "Container name (optional, defaults to first container)"),
			"tailLines": prop("integer", "Number of lines to return from the end (default 100)"),
			"previous":  prop("boolean", "Return logs from previous terminated container (default false)"),
			"maxChars":  prop("integer", "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."),
		},
		Required: []string{"namespace", "podName"},
	},
	{
		Name:        "get_pod_events",
		Description: "Get Kubernetes events for a specific pod. Shows warnings, errors, and lifecycle events. The response is automatically trimmed to fit within maxChars, keeping the most recent events.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Namespace of the pod"),
			"podName":   prop("string", "Name of the pod"),
			"maxChars":  prop("integer", "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."),
		},
		Required: []string{"namespace", "podName"},
	},
}
