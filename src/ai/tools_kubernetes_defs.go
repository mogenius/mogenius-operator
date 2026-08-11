package ai

// kubernetesToolSchemas defines the built-in Kubernetes tools once, provider-
// neutrally. The per-SDK slices below are generated from it via the converters
// in tools_schema.go, so a tool's name, params and description stay identical
// across OpenAI, Anthropic and Ollama.
var kubernetesToolSchemas = []toolSchema{
	{
		Name:        "get_kubernetes_resources",
		Description: "Get details of a specific Kubernetes resource by kind, name and namespace. Cost ladder: use list_kubernetes_resources for discovery, detail=summary here for triage, and detail=full ONLY when you need the complete manifest (e.g. to build an UpdateResource proposal).",
		Props: map[string]toolProp{
			"apiVersion": {Type: "string", Description: "API version (e.g. 'v1', 'apps/v1')"},
			"kind":       {Type: "string", Description: "Resource kind (e.g. 'Pod', 'Deployment')"},
			"name":       {Type: "string", Description: "Resource name"},
			"namespace":  {Type: "string", Description: "Namespace (optional for cluster-scoped)"},
			"detail":     {Type: "string", Enum: []string{"summary", "full"}, Description: "'summary' (recommended, ~1/3 of the tokens): identity, ownership, condensed status, key spec fields. 'full' (default): complete manifest"},
		},
		Required: []string{"kind", "apiVersion", "name"},
	},
	{
		Name:        "list_kubernetes_resources",
		Description: "List Kubernetes resources of a specific kind as compact summaries. Omit the namespace to list across ALL namespaces in one call — strongly preferred for cluster-wide sweeps; never iterate namespace by namespace.",
		Props: map[string]toolProp{
			"apiVersion": {Type: "string", Description: "API version (e.g. 'v1', 'apps/v1')"},
			"kind":       {Type: "string", Description: "Resource kind (e.g. 'Pod', 'Deployment')"},
			"namespace":  {Type: "string", Description: "Namespace filter. Omit to list across all namespaces (preferred)"},
		},
		Required: []string{"kind", "apiVersion"},
	},
	{
		Name:        "check_kubernetes_resource",
		Description: "Check existence and status of a single resource. Returns a compact summary instead of full details. Use get_kubernetes_resources only when you need the complete resource object.",
		Props: map[string]toolProp{
			"apiVersion": {Type: "string", Description: "API version (e.g. 'v1', 'apps/v1')"},
			"kind":       {Type: "string", Description: "Resource kind (e.g. 'Pod', 'Deployment')"},
			"name":       {Type: "string", Description: "Resource name"},
			"namespace":  {Type: "string", Description: "Namespace (optional for cluster-scoped)"},
		},
		Required: []string{"kind", "apiVersion", "name"},
	},
	{
		Name:        "update_kubernetes_resource",
		Description: "Update an existing Kubernetes resource with new YAML configuration.",
		Props: map[string]toolProp{
			"apiVersion": {Type: "string", Description: "API version (e.g. 'v1', 'apps/v1')"},
			"plural":     {Type: "string", Description: "Plural name (e.g. 'pods', 'deployments')"},
			"namespaced": {Type: "boolean", Description: "Namespaced (true) or cluster-scoped (false)"},
			"yamlData":   {Type: "string", Description: "Complete YAML definition of the resource"},
		},
		Required: []string{"apiVersion", "plural", "namespaced", "yamlData"},
	},
	{
		Name:        "delete_kubernetes_resource",
		Description: "Delete a Kubernetes resource by name and namespace.",
		Props: map[string]toolProp{
			"apiVersion": {Type: "string", Description: "API version (e.g. 'v1', 'apps/v1')"},
			"plural":     {Type: "string", Description: "Plural name (e.g. 'pods', 'deployments')"},
			"namespace":  {Type: "string", Description: "Namespace (empty for cluster-scoped)"},
			"name":       {Type: "string", Description: "Resource name to delete"},
		},
		Required: []string{"apiVersion", "plural", "name"},
	},
	{
		Name:        "create_kubernetes_resource",
		Description: "Create a new Kubernetes resource from YAML configuration.",
		Props: map[string]toolProp{
			"apiVersion": {Type: "string", Description: "API version (e.g. 'v1', 'apps/v1')"},
			"plural":     {Type: "string", Description: "Plural name (e.g. 'pods', 'deployments')"},
			"namespaced": {Type: "boolean", Description: "Namespaced (true) or cluster-scoped (false)"},
			"yamlData":   {Type: "string", Description: "Complete YAML definition of the resource"},
		},
		Required: []string{"apiVersion", "plural", "namespaced", "yamlData"},
	},
	{
		Name:        "get_pod_logs",
		Description: "Get logs from a pod's container. Returns the last N lines of log output. The response is automatically trimmed to fit within maxChars. Start with a small maxChars and increase only if you need more context.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Namespace of the pod"},
			"podName":   {Type: "string", Description: "Name of the pod"},
			"container": {Type: "string", Description: "Container name (optional, defaults to first container)"},
			"tailLines": {Type: "integer", Description: "Number of lines to return from the end (default 100)"},
			"previous":  {Type: "boolean", Description: "Return logs from previous terminated container (default false)"},
			"maxChars":  {Type: "integer", Description: "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."},
		},
		Required: []string{"namespace", "podName"},
	},
	{
		Name:        "get_pod_events",
		Description: "Get Kubernetes events for a specific pod. Shows warnings, errors, and lifecycle events. The response is automatically trimmed to fit within maxChars, keeping the most recent events.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Namespace of the pod"},
			"podName":   {Type: "string", Description: "Name of the pod"},
			"maxChars":  {Type: "integer", Description: "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."},
		},
		Required: []string{"namespace", "podName"},
	},
}

var (
	kubernetesOpenAiTools    = mapSchemas(kubernetesToolSchemas, toolSchema.toOpenAI)
	kubernetesAnthropicTools = mapSchemas(kubernetesToolSchemas, toolSchema.toAnthropic)
	kubernetesOllamaTools    = mapSchemas(kubernetesToolSchemas, toolSchema.toOllama)
)
