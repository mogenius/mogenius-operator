package ai

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
)

// --- OpenAI Kubernetes Tool Definitions ---

var kubernetesOpenAiTools = []openai.ChatCompletionToolUnionParam{
	openaiFunc(
		"get_kubernetes_resources",
		"Get full details of a specific Kubernetes resource by kind, name and namespace.",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{"type": "string", "description": "API version (e.g. 'v1', 'apps/v1')"},
				"kind":       map[string]string{"type": "string", "description": "Resource kind (e.g. 'Pod', 'Deployment')"},
				"name":       map[string]string{"type": "string", "description": "Resource name"},
				"namespace":  map[string]string{"type": "string", "description": "Namespace (optional for cluster-scoped)"},
			},
			"required": []string{"kind", "apiVersion", "name"},
		},
	),
	openaiFunc(
		"list_kubernetes_resources",
		"List all Kubernetes resources of a specific kind, optionally filtered by namespace.",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{"type": "string", "description": "API version (e.g. 'v1', 'apps/v1')"},
				"kind":       map[string]string{"type": "string", "description": "Resource kind (e.g. 'Pod', 'Deployment')"},
				"namespace":  map[string]string{"type": "string", "description": "Namespace filter (optional, empty for all)"},
			},
			"required": []string{"kind", "apiVersion"},
		},
	),
	openaiFunc(
		"check_kubernetes_resource",
		"Check existence and status of a single resource. Returns a compact summary instead of full details. Use get_kubernetes_resources only when you need the complete resource object.",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{"type": "string", "description": "API version (e.g. 'v1', 'apps/v1')"},
				"kind":       map[string]string{"type": "string", "description": "Resource kind (e.g. 'Pod', 'Deployment')"},
				"name":       map[string]string{"type": "string", "description": "Resource name"},
				"namespace":  map[string]string{"type": "string", "description": "Namespace (optional for cluster-scoped)"},
			},
			"required": []string{"kind", "apiVersion", "name"},
		},
	),
	openaiFunc(
		"update_kubernetes_resource",
		"Update an existing Kubernetes resource with new YAML configuration.",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{"type": "string", "description": "API version (e.g. 'v1', 'apps/v1')"},
				"plural":     map[string]string{"type": "string", "description": "Plural name (e.g. 'pods', 'deployments')"},
				"namespaced": map[string]string{"type": "boolean", "description": "Namespaced (true) or cluster-scoped (false)"},
				"yamlData":   map[string]string{"type": "string", "description": "Complete YAML definition of the resource"},
			},
			"required": []string{"apiVersion", "plural", "namespaced", "yamlData"},
		},
	),
	openaiFunc(
		"delete_kubernetes_resource",
		"Delete a Kubernetes resource by name and namespace.",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{"type": "string", "description": "API version (e.g. 'v1', 'apps/v1')"},
				"plural":     map[string]string{"type": "string", "description": "Plural name (e.g. 'pods', 'deployments')"},
				"namespace":  map[string]string{"type": "string", "description": "Namespace (empty for cluster-scoped)"},
				"name":       map[string]string{"type": "string", "description": "Resource name to delete"},
			},
			"required": []string{"apiVersion", "plural", "name"},
		},
	),
	openaiFunc(
		"create_kubernetes_resource",
		"Create a new Kubernetes resource from YAML configuration.",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{"type": "string", "description": "API version (e.g. 'v1', 'apps/v1')"},
				"plural":     map[string]string{"type": "string", "description": "Plural name (e.g. 'pods', 'deployments')"},
				"namespaced": map[string]string{"type": "boolean", "description": "Namespaced (true) or cluster-scoped (false)"},
				"yamlData":   map[string]string{"type": "string", "description": "Complete YAML definition of the resource"},
			},
			"required": []string{"apiVersion", "plural", "namespaced", "yamlData"},
		},
	),
	openaiFunc(
		"get_pod_logs",
		"Get logs from a pod's container. Returns the last N lines of log output. The response is automatically trimmed to fit within maxChars. Start with a small maxChars and increase only if you need more context.",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace": map[string]string{"type": "string", "description": "Namespace of the pod"},
				"podName":   map[string]string{"type": "string", "description": "Name of the pod"},
				"container": map[string]string{"type": "string", "description": "Container name (optional, defaults to first container)"},
				"tailLines": map[string]string{"type": "integer", "description": "Number of lines to return from the end (default 100)"},
				"previous":  map[string]string{"type": "boolean", "description": "Return logs from previous terminated container (default false)"},
				"maxChars":  map[string]string{"type": "integer", "description": "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."},
			},
			"required": []string{"namespace", "podName"},
		},
	),
	openaiFunc(
		"get_pod_events",
		"Get Kubernetes events for a specific pod. Shows warnings, errors, and lifecycle events. The response is automatically trimmed to fit within maxChars, keeping the most recent events.",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace": map[string]string{"type": "string", "description": "Namespace of the pod"},
				"podName":   map[string]string{"type": "string", "description": "Name of the pod"},
				"maxChars":  map[string]string{"type": "integer", "description": "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."},
			},
			"required": []string{"namespace", "podName"},
		},
	),
}

// --- Anthropic Kubernetes Tool Definitions ---

var kubernetesAnthropicTools = []anthropic.ToolParam{
	anthropicTool(
		"get_kubernetes_resources",
		"Get full details of a specific Kubernetes resource by name.",
		map[string]any{
			"kind":       map[string]any{"type": "string", "description": "Resource kind (e.g., Pod, Deployment)."},
			"apiVersion": map[string]any{"type": "string", "description": "API version (e.g., v1, apps/v1)."},
			"name":       map[string]any{"type": "string", "description": "Resource name."},
			"namespace":  map[string]any{"type": "string", "description": "Namespace (optional for cluster-scoped)."},
		}, []string{"kind", "apiVersion", "name"},
	),
	anthropicTool(
		"list_kubernetes_resources",
		"List all Kubernetes resources of a given kind, optionally filtered by namespace.",
		map[string]any{
			"kind":       map[string]any{"type": "string", "description": "Resource kind (e.g., Pod, Deployment)."},
			"apiVersion": map[string]any{"type": "string", "description": "API version (e.g., v1, apps/v1)."},
			"namespace":  map[string]any{"type": "string", "description": "Namespace filter (optional, omit for all)."},
		}, []string{"kind", "apiVersion"},
	),
	anthropicTool(
		"check_kubernetes_resource",
		"Check existence and status of a single resource. Returns a compact summary instead of full details. Use get_kubernetes_resources only when you need the complete resource object.",
		map[string]any{
			"kind":       map[string]any{"type": "string", "description": "Resource kind (e.g., Pod, Deployment)."},
			"apiVersion": map[string]any{"type": "string", "description": "API version (e.g., v1, apps/v1)."},
			"name":       map[string]any{"type": "string", "description": "Resource name."},
			"namespace":  map[string]any{"type": "string", "description": "Namespace (optional for cluster-scoped)."},
		}, []string{"kind", "apiVersion", "name"},
	),
	anthropicTool(
		"update_kubernetes_resource",
		"Update an existing Kubernetes resource with new YAML configuration.",
		map[string]any{
			"apiVersion": map[string]any{"type": "string", "description": "API version (e.g., v1, apps/v1)."},
			"plural":     map[string]any{"type": "string", "description": "Plural name (e.g., pods, deployments)."},
			"namespaced": map[string]any{"type": "boolean", "description": "Namespaced (true) or cluster-scoped (false)."},
			"yamlData":   map[string]any{"type": "string", "description": "Complete YAML definition of the resource."},
		}, []string{"apiVersion", "plural", "namespaced", "yamlData"},
	),
	anthropicTool(
		"delete_kubernetes_resource",
		"Delete a Kubernetes resource by name and namespace.",
		map[string]any{
			"apiVersion": map[string]any{"type": "string", "description": "API version (e.g., v1, apps/v1)."},
			"plural":     map[string]any{"type": "string", "description": "Plural name (e.g., pods, deployments)."},
			"namespace":  map[string]any{"type": "string", "description": "Namespace (empty for cluster-scoped)."},
			"name":       map[string]any{"type": "string", "description": "Resource name to delete."},
		}, []string{"apiVersion", "plural", "name"},
	),
	anthropicTool(
		"create_kubernetes_resource",
		"Create a new Kubernetes resource from YAML configuration.",
		map[string]any{
			"apiVersion": map[string]any{"type": "string", "description": "API version (e.g., v1, apps/v1)."},
			"plural":     map[string]any{"type": "string", "description": "Plural name (e.g., pods, deployments)."},
			"namespaced": map[string]any{"type": "boolean", "description": "Namespaced (true) or cluster-scoped (false)."},
			"yamlData":   map[string]any{"type": "string", "description": "Complete YAML definition of the resource."},
		}, []string{"apiVersion", "plural", "namespaced", "yamlData"},
	),
	anthropicTool(
		"get_pod_logs",
		"Get logs from a pod's container. Returns the last N lines of log output. The response is automatically trimmed to fit within maxChars. Start with a small maxChars and increase only if you need more context.",
		map[string]any{
			"namespace": map[string]any{"type": "string", "description": "Namespace of the pod."},
			"podName":   map[string]any{"type": "string", "description": "Name of the pod."},
			"container": map[string]any{"type": "string", "description": "Container name (optional, defaults to first container)."},
			"tailLines": map[string]any{"type": "integer", "description": "Number of lines to return from the end (default 100)."},
			"previous":  map[string]any{"type": "boolean", "description": "Return logs from previous terminated container (default false)."},
			"maxChars":  map[string]any{"type": "integer", "description": "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."},
		}, []string{"namespace", "podName"},
	),
	anthropicTool(
		"get_pod_events",
		"Get Kubernetes events for a specific pod. Shows warnings, errors, and lifecycle events. The response is automatically trimmed to fit within maxChars, keeping the most recent events.",
		map[string]any{
			"namespace": map[string]any{"type": "string", "description": "Namespace of the pod."},
			"podName":   map[string]any{"type": "string", "description": "Name of the pod."},
			"maxChars":  map[string]any{"type": "integer", "description": "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."},
		}, []string{"namespace", "podName"},
	),
}

// --- Ollama Kubernetes Tool Definitions ---

var kubernetesOllamaTools = []api.Tool{
	ollamaTool(
		"get_kubernetes_resources",
		"Get full details of a specific Kubernetes resource by kind, name and namespace.",
		map[string]api.ToolProperty{
			"apiVersion": {Type: []string{"string"}, Description: "API version (e.g. 'v1', 'apps/v1')"},
			"kind":       {Type: []string{"string"}, Description: "Resource kind (e.g. 'Pod', 'Deployment')"},
			"name":       {Type: []string{"string"}, Description: "Resource name"},
			"namespace":  {Type: []string{"string"}, Description: "Namespace (optional for cluster-scoped)"},
		}, []string{"kind", "apiVersion", "name"},
	),
	ollamaTool(
		"list_kubernetes_resources",
		"List all Kubernetes resources of a specific kind, optionally filtered by namespace.",
		map[string]api.ToolProperty{
			"apiVersion": {Type: []string{"string"}, Description: "API version (e.g. 'v1', 'apps/v1')"},
			"kind":       {Type: []string{"string"}, Description: "Resource kind (e.g. 'Pod', 'Deployment')"},
			"namespace":  {Type: []string{"string"}, Description: "Namespace filter (optional, empty for all)"},
		}, []string{"kind", "apiVersion"},
	),
	ollamaTool(
		"check_kubernetes_resource",
		"Check existence and status of a single resource. Returns a compact summary instead of full details. Use get_kubernetes_resources only when you need the complete resource object.",
		map[string]api.ToolProperty{
			"apiVersion": {Type: []string{"string"}, Description: "API version (e.g. 'v1', 'apps/v1')"},
			"kind":       {Type: []string{"string"}, Description: "Resource kind (e.g. 'Pod', 'Deployment')"},
			"name":       {Type: []string{"string"}, Description: "Resource name"},
			"namespace":  {Type: []string{"string"}, Description: "Namespace (optional for cluster-scoped)"},
		}, []string{"kind", "apiVersion", "name"},
	),
	ollamaTool(
		"update_kubernetes_resource",
		"Update an existing Kubernetes resource with new YAML configuration.",
		map[string]api.ToolProperty{
			"apiVersion": {Type: []string{"string"}, Description: "API version (e.g. 'v1', 'apps/v1')"},
			"plural":     {Type: []string{"string"}, Description: "Plural name (e.g. 'pods', 'deployments')"},
			"namespaced": {Type: []string{"boolean"}, Description: "Namespaced (true) or cluster-scoped (false)"},
			"yamlData":   {Type: []string{"string"}, Description: "Complete YAML definition of the resource"},
		}, []string{"apiVersion", "plural", "namespaced", "yamlData"},
	),
	ollamaTool(
		"delete_kubernetes_resource",
		"Delete a Kubernetes resource by name and namespace.",
		map[string]api.ToolProperty{
			"apiVersion": {Type: []string{"string"}, Description: "API version (e.g. 'v1', 'apps/v1')"},
			"plural":     {Type: []string{"string"}, Description: "Plural name (e.g. 'pods', 'deployments')"},
			"namespace":  {Type: []string{"string"}, Description: "Namespace (empty for cluster-scoped)"},
			"name":       {Type: []string{"string"}, Description: "Resource name to delete"},
		}, []string{"apiVersion", "plural", "name"},
	),
	ollamaTool(
		"create_kubernetes_resource",
		"Create a new Kubernetes resource from YAML configuration.",
		map[string]api.ToolProperty{
			"apiVersion": {Type: []string{"string"}, Description: "API version (e.g. 'v1', 'apps/v1')"},
			"plural":     {Type: []string{"string"}, Description: "Plural name (e.g. 'pods', 'deployments')"},
			"namespaced": {Type: []string{"boolean"}, Description: "Namespaced (true) or cluster-scoped (false)"},
			"yamlData":   {Type: []string{"string"}, Description: "Complete YAML definition of the resource"},
		}, []string{"apiVersion", "plural", "namespaced", "yamlData"},
	),
	ollamaTool(
		"get_pod_logs",
		"Get logs from a pod's container. Returns the last N lines of log output. The response is automatically trimmed to fit within maxChars. Start with a small maxChars and increase only if you need more context.",
		map[string]api.ToolProperty{
			"namespace": {Type: []string{"string"}, Description: "Namespace of the pod"},
			"podName":   {Type: []string{"string"}, Description: "Name of the pod"},
			"container": {Type: []string{"string"}, Description: "Container name (optional, defaults to first container)"},
			"tailLines": {Type: []string{"integer"}, Description: "Number of lines to return from the end (default 100)"},
			"previous":  {Type: []string{"boolean"}, Description: "Return logs from previous terminated container (default false)"},
			"maxChars":  {Type: []string{"integer"}, Description: "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."},
		}, []string{"namespace", "podName"},
	),
	ollamaTool(
		"get_pod_events",
		"Get Kubernetes events for a specific pod. Shows warnings, errors, and lifecycle events. The response is automatically trimmed to fit within maxChars, keeping the most recent events.",
		map[string]api.ToolProperty{
			"namespace": {Type: []string{"string"}, Description: "Namespace of the pod"},
			"podName":   {Type: []string{"string"}, Description: "Name of the pod"},
			"maxChars":  {Type: []string{"integer"}, Description: "Maximum characters in response (default 20000, max 50000). Use lower values to save tokens."},
		}, []string{"namespace", "podName"},
	),
}
