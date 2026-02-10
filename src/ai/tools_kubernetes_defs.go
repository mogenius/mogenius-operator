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
		"Get a specific Kubernetes resource by kind, name and namespace",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{
					"type":        "string",
					"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
				},
				"kind": map[string]string{
					"type":        "string",
					"description": "Kind of the resource (e.g. 'Pod', 'Deployment', 'Service')",
				},
				"name": map[string]string{
					"type":        "string",
					"description": "Name of the specific resource",
				},
				"namespace": map[string]string{
					"type":        "string",
					"description": "Namespace of the resource (optional for cluster-scoped resources)",
				},
			},
			"required": []string{
				"kind",
				"apiVersion",
				"name",
			},
		},
	),
	openaiFunc(
		"list_kubernetes_resources",
		"List all Kubernetes resources of a specific kind, optionally filtered by namespace",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{
					"type":        "string",
					"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
				},
				"kind": map[string]string{
					"type":        "string",
					"description": "Kind of the resource (e.g. 'Pod', 'Deployment', 'Service')",
				},
				"namespace": map[string]string{
					"type":        "string",
					"description": "Namespace to filter by (optional, leave empty for all namespaces)",
				},
			},
			"required": []string{
				"kind",
				"apiVersion",
			},
		},
	),
	openaiFunc(
		"update_kubernetes_resource",
		"Update an existing Kubernetes resource with new YAML configuration",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{
					"type":        "string",
					"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
				},
				"plural": map[string]string{
					"type":        "string",
					"description": "Plural name of the resource (e.g. 'pods', 'deployments', 'services')",
				},
				"namespaced": map[string]string{
					"type":        "boolean",
					"description": "Whether the resource is namespaced (true) or cluster-scoped (false)",
				},
				"yamlData": map[string]string{
					"type":        "string",
					"description": "Complete YAML definition of the resource to update",
				},
			},
			"required": []string{
				"apiVersion",
				"plural",
				"namespaced",
				"yamlData",
			},
		},
	),
	openaiFunc(
		"delete_kubernetes_resource",
		"Delete a Kubernetes resource by name and namespace",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{
					"type":        "string",
					"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
				},
				"plural": map[string]string{
					"type":        "string",
					"description": "Plural name of the resource (e.g. 'pods', 'deployments', 'services')",
				},
				"namespace": map[string]string{
					"type":        "string",
					"description": "Namespace of the resource (empty for cluster-scoped resources)",
				},
				"name": map[string]string{
					"type":        "string",
					"description": "Name of the resource to delete",
				},
			},
			"required": []string{
				"apiVersion",
				"plural",
				"name",
			},
		},
	),
	openaiFunc(
		"create_kubernetes_resource",
		"Create a new Kubernetes resource from YAML configuration",
		openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"apiVersion": map[string]string{
					"type":        "string",
					"description": "API version of the resource (e.g. 'v1', 'apps/v1')",
				},
				"plural": map[string]string{
					"type":        "string",
					"description": "Plural name of the resource (e.g. 'pods', 'deployments', 'services')",
				},
				"namespaced": map[string]string{
					"type":        "boolean",
					"description": "Whether the resource is namespaced (true) or cluster-scoped (false)",
				},
				"yamlData": map[string]string{
					"type":        "string",
					"description": "Complete YAML definition of the resource to create",
				},
			},
			"required": []string{
				"apiVersion",
				"plural",
				"namespaced",
				"yamlData",
			},
		},
	),
}

// --- Anthropic Kubernetes Tool Definitions ---

var kubernetesAnthropicTools = []anthropic.ToolParam{
	anthropicTool(
		"get_kubernetes_resources",
		"Get a specific Kubernetes resource by name. Use this when you know the exact name of the resource you want to retrieve.",
		map[string]any{
			"kind": map[string]any{
				"type":        "string",
				"description": "The kind of the Kubernetes resource (e.g., Pod, Deployment, Service).",
			},
			"apiVersion": map[string]any{
				"type":        "string",
				"description": "The API version of the resource (e.g., v1, apps/v1).",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "The name of the resource.",
			},
			"namespace": map[string]any{
				"type":        "string",
				"description": "The namespace of the resource (optional for cluster-scoped resources).",
			},
		}, []string{
			"kind",
			"apiVersion",
			"name",
		},
	),
	anthropicTool(
		"list_kubernetes_resources",
		"List all Kubernetes resources of a given kind, optionally filtered by namespace. Use this when you want to see multiple resources or don't know the exact name.",
		map[string]any{
			"kind": map[string]any{
				"type":        "string",
				"description": "The kind of the Kubernetes resource (e.g., Pod, Deployment, Service).",
			},
			"apiVersion": map[string]any{
				"type":        "string",
				"description": "The API version of the resource (e.g., v1, apps/v1).",
			},
			"namespace": map[string]any{
				"type":        "string",
				"description": "The namespace to list resources from (optional; omit to list from all namespaces or cluster-scoped resources).",
			},
		}, []string{
			"kind",
			"apiVersion",
		},
	),
	anthropicTool(
		"update_kubernetes_resource",
		"Update an existing Kubernetes resource with new YAML configuration. Use this to modify resources.",
		map[string]any{
			"apiVersion": map[string]any{
				"type":        "string",
				"description": "The API version of the resource (e.g., v1, apps/v1).",
			},
			"plural": map[string]any{
				"type":        "string",
				"description": "The plural name of the resource (e.g., pods, deployments, services).",
			},
			"namespaced": map[string]any{
				"type":        "boolean",
				"description": "Whether the resource is namespaced (true) or cluster-scoped (false).",
			},
			"yamlData": map[string]any{
				"type":        "string",
				"description": "Complete YAML definition of the resource to update.",
			},
		}, []string{
			"apiVersion",
			"plural",
			"namespaced",
			"yamlData",
		},
	),
	anthropicTool(
		"delete_kubernetes_resource",
		"Delete a Kubernetes resource by name and namespace.",
		map[string]any{
			"apiVersion": map[string]any{
				"type":        "string",
				"description": "The API version of the resource (e.g., v1, apps/v1).",
			},
			"plural": map[string]any{
				"type":        "string",
				"description": "The plural name of the resource (e.g., pods, deployments, services).",
			},
			"namespace": map[string]any{
				"type":        "string",
				"description": "The namespace of the resource (empty for cluster-scoped resources).",
			},
			"name": map[string]any{
				"type":        "string",
				"description": "The name of the resource to delete.",
			},
		}, []string{
			"apiVersion",
			"plural",
			"name",
		},
	),
	anthropicTool(
		"create_kubernetes_resource",
		"Create a new Kubernetes resource from YAML configuration.",
		map[string]any{
			"apiVersion": map[string]any{
				"type":        "string",
				"description": "The API version of the resource (e.g., v1, apps/v1).",
			},
			"plural": map[string]any{
				"type":        "string",
				"description": "The plural name of the resource (e.g., pods, deployments, services).",
			},
			"namespaced": map[string]any{
				"type":        "boolean",
				"description": "Whether the resource is namespaced (true) or cluster-scoped (false).",
			},
			"yamlData": map[string]any{
				"type":        "string",
				"description": "Complete YAML definition of the resource to create.",
			},
		}, []string{
			"apiVersion",
			"plural",
			"namespaced",
			"yamlData",
		},
	),
}

// --- Ollama Kubernetes Tool Definitions ---

var kubernetesOllamaTools = []api.Tool{
	ollamaTool(
		"get_kubernetes_resources",
		"Get a specific Kubernetes resource by kind, name and namespace",
		map[string]api.ToolProperty{
			"apiVersion": {
				Type:        []string{"string"},
				Description: "API version of the resource (e.g. 'v1', 'apps/v1')",
			},
			"kind": {
				Type:        []string{"string"},
				Description: "Kind of the resource (e.g. 'Pod', 'Deployment', 'Service')",
			},
			"name": {
				Type:        []string{"string"},
				Description: "Name of the specific resource",
			},
			"namespace": {
				Type:        []string{"string"},
				Description: "Namespace of the resource (optional for cluster-scoped resources)",
			},
		}, []string{
			"kind",
			"apiVersion",
			"name",
		},
	),
	ollamaTool(
		"list_kubernetes_resources",
		"List all Kubernetes resources of a specific kind, optionally filtered by namespace",
		map[string]api.ToolProperty{
			"apiVersion": {
				Type:        []string{"string"},
				Description: "API version of the resource (e.g. 'v1', 'apps/v1')",
			},
			"kind": {
				Type:        []string{"string"},
				Description: "Kind of the resource (e.g. 'Pod', 'Deployment', 'Service')",
			},
			"namespace": {
				Type:        []string{"string"},
				Description: "Namespace to filter by (optional, leave empty for all namespaces)",
			},
		}, []string{
			"kind",
			"apiVersion",
		},
	),
	ollamaTool(
		"update_kubernetes_resource",
		"Update an existing Kubernetes resource with new YAML configuration",
		map[string]api.ToolProperty{
			"apiVersion": {
				Type:        []string{"string"},
				Description: "API version of the resource (e.g. 'v1', 'apps/v1')",
			},
			"plural": {
				Type:        []string{"string"},
				Description: "Plural name of the resource (e.g. 'pods', 'deployments', 'services')",
			},
			"namespaced": {
				Type:        []string{"boolean"},
				Description: "Whether the resource is namespaced (true) or cluster-scoped (false)",
			},
			"yamlData": {
				Type:        []string{"string"},
				Description: "Complete YAML definition of the resource to update",
			},
		}, []string{
			"apiVersion",
			"plural",
			"namespaced",
			"yamlData",
		},
	),
	ollamaTool(
		"delete_kubernetes_resource",
		"Delete a Kubernetes resource by name and namespace",
		map[string]api.ToolProperty{
			"apiVersion": {
				Type:        []string{"string"},
				Description: "API version of the resource (e.g. 'v1', 'apps/v1')",
			},
			"plural": {
				Type:        []string{"string"},
				Description: "Plural name of the resource (e.g. 'pods', 'deployments', 'services')",
			},
			"namespace": {
				Type:        []string{"string"},
				Description: "Namespace of the resource (empty for cluster-scoped resources)",
			},
			"name": {
				Type:        []string{"string"},
				Description: "Name of the resource to delete",
			},
		}, []string{
			"apiVersion",
			"plural",
			"name",
		},
	),
	ollamaTool(
		"create_kubernetes_resource",
		"Create a new Kubernetes resource from YAML configuration",
		map[string]api.ToolProperty{
			"apiVersion": {
				Type:        []string{"string"},
				Description: "API version of the resource (e.g. 'v1', 'apps/v1')",
			},
			"plural": {
				Type:        []string{"string"},
				Description: "Plural name of the resource (e.g. 'pods', 'deployments', 'services')",
			},
			"namespaced": {
				Type:        []string{"boolean"},
				Description: "Whether the resource is namespaced (true) or cluster-scoped (false)",
			},
			"yamlData": {
				Type:        []string{"string"},
				Description: "Complete YAML definition of the resource to create",
			},
		}, []string{
			"apiVersion",
			"plural",
			"namespaced",
			"yamlData",
		},
	),
}