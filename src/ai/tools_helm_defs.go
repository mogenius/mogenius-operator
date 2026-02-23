package ai

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/ollama/ollama/api"
	"github.com/openai/openai-go/v3"
)

// --- OpenAI Helm Tool Definitions ---

var helmOpenAiTools = []openai.ChatCompletionToolUnionParam{
	openaiFunc("helm_repo_add", "Add a Helm chart repository.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"name":               map[string]string{"type": "string", "description": "Repository name"},
			"url":                map[string]string{"type": "string", "description": "Repository URL"},
			"username":           map[string]string{"type": "string", "description": "Auth username (optional)"},
			"password":           map[string]string{"type": "string", "description": "Auth password (optional)"},
			"insecureSkipTLS":    map[string]string{"type": "boolean", "description": "Skip TLS verification (optional)"},
			"passCredentialsAll": map[string]string{"type": "boolean", "description": "Pass credentials to all domains (optional)"},
		},
		"required": []string{"name", "url"},
	}),
	openaiFunc("helm_repo_patch", "Update an existing Helm chart repository configuration.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"name":               map[string]string{"type": "string", "description": "Current repository name"},
			"newName":            map[string]string{"type": "string", "description": "New repository name"},
			"url":                map[string]string{"type": "string", "description": "New repository URL"},
			"username":           map[string]string{"type": "string", "description": "Auth username (optional)"},
			"password":           map[string]string{"type": "string", "description": "Auth password (optional)"},
			"insecureSkipTLS":    map[string]string{"type": "boolean", "description": "Skip TLS verification (optional)"},
			"passCredentialsAll": map[string]string{"type": "boolean", "description": "Pass credentials to all domains (optional)"},
		},
		"required": []string{"name", "newName", "url"},
	}),
	openaiFunc("helm_repo_update", "Update all Helm chart repositories to fetch latest chart information.", openai.FunctionParameters{
		"type": "object", "properties": map[string]any{},
	}),
	openaiFunc("helm_repo_list", "List all configured Helm chart repositories.", openai.FunctionParameters{
		"type": "object", "properties": map[string]any{},
	}),
	openaiFunc("helm_repo_remove", "Remove a Helm chart repository.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]string{"type": "string", "description": "Repository name to remove"},
		},
		"required": []string{"name"},
	}),
	openaiFunc("helm_chart_search", "Search for Helm charts across all configured repositories.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]string{"type": "string", "description": "Chart name or keyword (optional, empty lists all)"},
		},
	}),
	openaiFunc("helm_chart_install", "Install a Helm chart as a new release.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]string{"type": "string", "description": "Target namespace"},
			"chart":     map[string]string{"type": "string", "description": "Chart reference (e.g. 'repo/chart-name')"},
			"release":   map[string]string{"type": "string", "description": "Release name"},
			"version":   map[string]string{"type": "string", "description": "Chart version (optional, latest if empty)"},
			"values":    map[string]string{"type": "string", "description": "YAML values override (optional)"},
			"dryRun":    map[string]string{"type": "boolean", "description": "Simulate without changes (optional)"},
		},
		"required": []string{"namespace", "chart", "release"},
	}),
	openaiFunc("helm_oci_install", "Install a Helm chart from an OCI registry.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"ociChartUrl": map[string]string{"type": "string", "description": "OCI chart URL (e.g. 'oci://registry/chart')"},
			"namespace":   map[string]string{"type": "string", "description": "Target namespace"},
			"release":     map[string]string{"type": "string", "description": "Release name"},
			"version":     map[string]string{"type": "string", "description": "Chart version (optional)"},
			"values":      map[string]string{"type": "string", "description": "YAML values override (optional)"},
			"dryRun":      map[string]string{"type": "boolean", "description": "Simulate without changes (optional)"},
			"authHost":    map[string]string{"type": "string", "description": "OCI registry auth host (optional)"},
			"username":    map[string]string{"type": "string", "description": "OCI registry username (optional)"},
			"password":    map[string]string{"type": "string", "description": "OCI registry password (optional)"},
		},
		"required": []string{"ociChartUrl", "namespace", "release"},
	}),
	openaiFunc("helm_chart_show", "Show information about a Helm chart (values, readme, metadata, or CRDs).", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"chart":      map[string]string{"type": "string", "description": "Chart reference (e.g. 'repo/chart-name')"},
			"showFormat": map[string]string{"type": "string", "description": "Format: 'all', 'chart', 'values', 'readme', or 'crds'"},
			"version":    map[string]string{"type": "string", "description": "Chart version (optional)"},
		},
		"required": []string{"chart", "showFormat"},
	}),
	openaiFunc("helm_chart_versions", "List all available versions of a Helm chart.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"chart": map[string]string{"type": "string", "description": "Chart reference (e.g. 'repo/chart-name')"},
		},
		"required": []string{"chart"},
	}),
	openaiFunc("helm_release_upgrade", "Upgrade an existing Helm release.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]string{"type": "string", "description": "Release namespace"},
			"chart":     map[string]string{"type": "string", "description": "Chart reference (e.g. 'repo/chart-name')"},
			"release":   map[string]string{"type": "string", "description": "Release name"},
			"version":   map[string]string{"type": "string", "description": "Chart version (optional)"},
			"values":    map[string]string{"type": "string", "description": "YAML values override (optional)"},
			"dryRun":    map[string]string{"type": "boolean", "description": "Simulate without changes (optional)"},
		},
		"required": []string{"namespace", "chart", "release"},
	}),
	openaiFunc("helm_release_uninstall", "Uninstall a Helm release.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]string{"type": "string", "description": "Release namespace"},
			"release":   map[string]string{"type": "string", "description": "Release name"},
			"dryRun":    map[string]string{"type": "boolean", "description": "Simulate without changes (optional)"},
		},
		"required": []string{"namespace", "release"},
	}),
	openaiFunc("helm_release_list", "List all Helm releases in a namespace.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]string{"type": "string", "description": "Namespace (empty for all)"},
		},
	}),
	openaiFunc("helm_release_status", "Get the status of a Helm release.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]string{"type": "string", "description": "Release namespace"},
			"release":   map[string]string{"type": "string", "description": "Release name"},
		},
		"required": []string{"namespace", "release"},
	}),
	openaiFunc("helm_release_history", "Get the revision history of a Helm release.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]string{"type": "string", "description": "Release namespace"},
			"release":   map[string]string{"type": "string", "description": "Release name"},
		},
		"required": []string{"namespace", "release"},
	}),
	openaiFunc("helm_release_rollback", "Rollback a Helm release to a previous revision.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]string{"type": "string", "description": "Release namespace"},
			"release":   map[string]string{"type": "string", "description": "Release name"},
			"revision":  map[string]string{"type": "integer", "description": "Revision number to rollback to"},
		},
		"required": []string{"namespace", "release", "revision"},
	}),
	openaiFunc("helm_release_get", "Get detailed information about a Helm release.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]string{"type": "string", "description": "Release namespace"},
			"release":   map[string]string{"type": "string", "description": "Release name"},
			"getFormat": map[string]string{"type": "string", "description": "Format: 'all', 'hooks', 'manifest', 'notes', or 'values'"},
		},
		"required": []string{"namespace", "release", "getFormat"},
	}),
	openaiFunc("helm_release_link", "Link a Helm release to a repository name for tracking.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace":   map[string]string{"type": "string", "description": "Release namespace"},
			"releaseName": map[string]string{"type": "string", "description": "Release name"},
			"repoName":    map[string]string{"type": "string", "description": "Repository name to link"},
		},
		"required": []string{"namespace", "releaseName", "repoName"},
	}),
	openaiFunc("helm_release_get_workloads", "Get the Kubernetes workloads managed by a Helm release.", openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]string{"type": "string", "description": "Release namespace"},
			"release":   map[string]string{"type": "string", "description": "Release name"},
		},
		"required": []string{"namespace", "release"},
	}),
}

// --- Anthropic Helm Tool Definitions ---

var helmAnthropicTools = []anthropic.ToolParam{
	anthropicTool("helm_repo_add", "Add a Helm chart repository.", map[string]any{
		"name":               map[string]any{"type": "string", "description": "Repository name."},
		"url":                map[string]any{"type": "string", "description": "Repository URL."},
		"username":           map[string]any{"type": "string", "description": "Auth username (optional)."},
		"password":           map[string]any{"type": "string", "description": "Auth password (optional)."},
		"insecureSkipTLS":    map[string]any{"type": "boolean", "description": "Skip TLS verification (optional)."},
		"passCredentialsAll": map[string]any{"type": "boolean", "description": "Pass credentials to all domains (optional)."},
	}, []string{"name", "url"}),
	anthropicTool("helm_repo_patch", "Update an existing Helm chart repository configuration.", map[string]any{
		"name":               map[string]any{"type": "string", "description": "Current repository name."},
		"newName":            map[string]any{"type": "string", "description": "New repository name."},
		"url":                map[string]any{"type": "string", "description": "New repository URL."},
		"username":           map[string]any{"type": "string", "description": "Auth username (optional)."},
		"password":           map[string]any{"type": "string", "description": "Auth password (optional)."},
		"insecureSkipTLS":    map[string]any{"type": "boolean", "description": "Skip TLS verification (optional)."},
		"passCredentialsAll": map[string]any{"type": "boolean", "description": "Pass credentials to all domains (optional)."},
	}, []string{"name", "newName", "url"}),
	anthropicTool("helm_repo_update", "Update all Helm chart repositories to fetch latest chart information.", map[string]any{}, nil),
	anthropicTool("helm_repo_list", "List all configured Helm chart repositories.", map[string]any{}, nil),
	anthropicTool("helm_repo_remove", "Remove a Helm chart repository.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Repository name to remove."},
	}, []string{"name"}),
	anthropicTool("helm_chart_search", "Search for Helm charts across all configured repositories.", map[string]any{
		"name": map[string]any{"type": "string", "description": "Chart name or keyword (optional, empty lists all)."},
	}, nil),
	anthropicTool("helm_chart_install", "Install a Helm chart as a new release.", map[string]any{
		"namespace": map[string]any{"type": "string", "description": "Target namespace."},
		"chart":     map[string]any{"type": "string", "description": "Chart reference (e.g. 'repo/chart-name')."},
		"release":   map[string]any{"type": "string", "description": "Release name."},
		"version":   map[string]any{"type": "string", "description": "Chart version (optional, latest if empty)."},
		"values":    map[string]any{"type": "string", "description": "YAML values override (optional)."},
		"dryRun":    map[string]any{"type": "boolean", "description": "Simulate without changes (optional)."},
	}, []string{"namespace", "chart", "release"}),
	anthropicTool("helm_oci_install", "Install a Helm chart from an OCI registry.", map[string]any{
		"ociChartUrl": map[string]any{"type": "string", "description": "OCI chart URL (e.g. 'oci://registry/chart')."},
		"namespace":   map[string]any{"type": "string", "description": "Target namespace."},
		"release":     map[string]any{"type": "string", "description": "Release name."},
		"version":     map[string]any{"type": "string", "description": "Chart version (optional)."},
		"values":      map[string]any{"type": "string", "description": "YAML values override (optional)."},
		"dryRun":      map[string]any{"type": "boolean", "description": "Simulate without changes (optional)."},
		"authHost":    map[string]any{"type": "string", "description": "OCI registry auth host (optional)."},
		"username":    map[string]any{"type": "string", "description": "OCI registry username (optional)."},
		"password":    map[string]any{"type": "string", "description": "OCI registry password (optional)."},
	}, []string{"ociChartUrl", "namespace", "release"}),
	anthropicTool("helm_chart_show", "Show information about a Helm chart (values, readme, metadata, or CRDs).", map[string]any{
		"chart":      map[string]any{"type": "string", "description": "Chart reference (e.g. 'repo/chart-name')."},
		"showFormat": map[string]any{"type": "string", "description": "Format: 'all', 'chart', 'values', 'readme', or 'crds'."},
		"version":    map[string]any{"type": "string", "description": "Chart version (optional)."},
	}, []string{"chart", "showFormat"}),
	anthropicTool("helm_chart_versions", "List all available versions of a Helm chart.", map[string]any{
		"chart": map[string]any{"type": "string", "description": "Chart reference (e.g. 'repo/chart-name')."},
	}, []string{"chart"}),
	anthropicTool("helm_release_upgrade", "Upgrade an existing Helm release.", map[string]any{
		"namespace": map[string]any{"type": "string", "description": "Release namespace."},
		"chart":     map[string]any{"type": "string", "description": "Chart reference (e.g. 'repo/chart-name')."},
		"release":   map[string]any{"type": "string", "description": "Release name."},
		"version":   map[string]any{"type": "string", "description": "Chart version (optional)."},
		"values":    map[string]any{"type": "string", "description": "YAML values override (optional)."},
		"dryRun":    map[string]any{"type": "boolean", "description": "Simulate without changes (optional)."},
	}, []string{"namespace", "chart", "release"}),
	anthropicTool("helm_release_uninstall", "Uninstall a Helm release.", map[string]any{
		"namespace": map[string]any{"type": "string", "description": "Release namespace."},
		"release":   map[string]any{"type": "string", "description": "Release name."},
		"dryRun":    map[string]any{"type": "boolean", "description": "Simulate without changes (optional)."},
	}, []string{"namespace", "release"}),
	anthropicTool("helm_release_list", "List all Helm releases in a namespace.", map[string]any{
		"namespace": map[string]any{"type": "string", "description": "Namespace (empty for all)."},
	}, nil),
	anthropicTool("helm_release_status", "Get the status of a Helm release.", map[string]any{
		"namespace": map[string]any{"type": "string", "description": "Release namespace."},
		"release":   map[string]any{"type": "string", "description": "Release name."},
	}, []string{"namespace", "release"}),
	anthropicTool("helm_release_history", "Get the revision history of a Helm release.", map[string]any{
		"namespace": map[string]any{"type": "string", "description": "Release namespace."},
		"release":   map[string]any{"type": "string", "description": "Release name."},
	}, []string{"namespace", "release"}),
	anthropicTool("helm_release_rollback", "Rollback a Helm release to a previous revision.", map[string]any{
		"namespace": map[string]any{"type": "string", "description": "Release namespace."},
		"release":   map[string]any{"type": "string", "description": "Release name."},
		"revision":  map[string]any{"type": "integer", "description": "Revision number to rollback to."},
	}, []string{"namespace", "release", "revision"}),
	anthropicTool("helm_release_get", "Get detailed information about a Helm release.", map[string]any{
		"namespace": map[string]any{"type": "string", "description": "Release namespace."},
		"release":   map[string]any{"type": "string", "description": "Release name."},
		"getFormat": map[string]any{"type": "string", "description": "Format: 'all', 'hooks', 'manifest', 'notes', or 'values'."},
	}, []string{"namespace", "release", "getFormat"}),
	anthropicTool("helm_release_link", "Link a Helm release to a repository name for tracking.", map[string]any{
		"namespace":   map[string]any{"type": "string", "description": "Release namespace."},
		"releaseName": map[string]any{"type": "string", "description": "Release name."},
		"repoName":    map[string]any{"type": "string", "description": "Repository name to link."},
	}, []string{"namespace", "releaseName", "repoName"}),
	anthropicTool("helm_release_get_workloads", "Get the Kubernetes workloads managed by a Helm release.", map[string]any{
		"namespace": map[string]any{"type": "string", "description": "Release namespace."},
		"release":   map[string]any{"type": "string", "description": "Release name."},
	}, []string{"namespace", "release"}),
}

// --- Ollama Helm Tool Definitions ---

var helmOllamaTools = []api.Tool{
	ollamaTool("helm_repo_add", "Add a Helm chart repository.", map[string]api.ToolProperty{
		"name":               {Type: []string{"string"}, Description: "Repository name"},
		"url":                {Type: []string{"string"}, Description: "Repository URL"},
		"username":           {Type: []string{"string"}, Description: "Auth username (optional)"},
		"password":           {Type: []string{"string"}, Description: "Auth password (optional)"},
		"insecureSkipTLS":    {Type: []string{"boolean"}, Description: "Skip TLS verification (optional)"},
		"passCredentialsAll": {Type: []string{"boolean"}, Description: "Pass credentials to all domains (optional)"},
	}, []string{"name", "url"}),
	ollamaTool("helm_repo_patch", "Update an existing Helm chart repository configuration.", map[string]api.ToolProperty{
		"name":               {Type: []string{"string"}, Description: "Current repository name"},
		"newName":            {Type: []string{"string"}, Description: "New repository name"},
		"url":                {Type: []string{"string"}, Description: "New repository URL"},
		"username":           {Type: []string{"string"}, Description: "Auth username (optional)"},
		"password":           {Type: []string{"string"}, Description: "Auth password (optional)"},
		"insecureSkipTLS":    {Type: []string{"boolean"}, Description: "Skip TLS verification (optional)"},
		"passCredentialsAll": {Type: []string{"boolean"}, Description: "Pass credentials to all domains (optional)"},
	}, []string{"name", "newName", "url"}),
	ollamaTool("helm_repo_update", "Update all Helm chart repositories to fetch latest chart information.", nil, nil),
	ollamaTool("helm_repo_list", "List all configured Helm chart repositories.", nil, nil),
	ollamaTool("helm_repo_remove", "Remove a Helm chart repository.", map[string]api.ToolProperty{
		"name": {Type: []string{"string"}, Description: "Repository name to remove"},
	}, []string{"name"}),
	ollamaTool("helm_chart_search", "Search for Helm charts across all configured repositories.", map[string]api.ToolProperty{
		"name": {Type: []string{"string"}, Description: "Chart name or keyword (optional, empty lists all)"},
	}, nil),
	ollamaTool("helm_chart_install", "Install a Helm chart as a new release.", map[string]api.ToolProperty{
		"namespace": {Type: []string{"string"}, Description: "Target namespace"},
		"chart":     {Type: []string{"string"}, Description: "Chart reference (e.g. 'repo/chart-name')"},
		"release":   {Type: []string{"string"}, Description: "Release name"},
		"version":   {Type: []string{"string"}, Description: "Chart version (optional, latest if empty)"},
		"values":    {Type: []string{"string"}, Description: "YAML values override (optional)"},
		"dryRun":    {Type: []string{"boolean"}, Description: "Simulate without changes (optional)"},
	}, []string{"namespace", "chart", "release"}),
	ollamaTool("helm_oci_install", "Install a Helm chart from an OCI registry.", map[string]api.ToolProperty{
		"ociChartUrl": {Type: []string{"string"}, Description: "OCI chart URL (e.g. 'oci://registry/chart')"},
		"namespace":   {Type: []string{"string"}, Description: "Target namespace"},
		"release":     {Type: []string{"string"}, Description: "Release name"},
		"version":     {Type: []string{"string"}, Description: "Chart version (optional)"},
		"values":      {Type: []string{"string"}, Description: "YAML values override (optional)"},
		"dryRun":      {Type: []string{"boolean"}, Description: "Simulate without changes (optional)"},
		"authHost":    {Type: []string{"string"}, Description: "OCI registry auth host (optional)"},
		"username":    {Type: []string{"string"}, Description: "OCI registry username (optional)"},
		"password":    {Type: []string{"string"}, Description: "OCI registry password (optional)"},
	}, []string{"ociChartUrl", "namespace", "release"}),
	ollamaTool("helm_chart_show", "Show information about a Helm chart (values, readme, metadata, or CRDs).", map[string]api.ToolProperty{
		"chart":      {Type: []string{"string"}, Description: "Chart reference (e.g. 'repo/chart-name')"},
		"showFormat": {Type: []string{"string"}, Description: "Format: 'all', 'chart', 'values', 'readme', or 'crds'"},
		"version":    {Type: []string{"string"}, Description: "Chart version (optional)"},
	}, []string{"chart", "showFormat"}),
	ollamaTool("helm_chart_versions", "List all available versions of a Helm chart.", map[string]api.ToolProperty{
		"chart": {Type: []string{"string"}, Description: "Chart reference (e.g. 'repo/chart-name')"},
	}, []string{"chart"}),
	ollamaTool("helm_release_upgrade", "Upgrade an existing Helm release.", map[string]api.ToolProperty{
		"namespace": {Type: []string{"string"}, Description: "Release namespace"},
		"chart":     {Type: []string{"string"}, Description: "Chart reference (e.g. 'repo/chart-name')"},
		"release":   {Type: []string{"string"}, Description: "Release name"},
		"version":   {Type: []string{"string"}, Description: "Chart version (optional)"},
		"values":    {Type: []string{"string"}, Description: "YAML values override (optional)"},
		"dryRun":    {Type: []string{"boolean"}, Description: "Simulate without changes (optional)"},
	}, []string{"namespace", "chart", "release"}),
	ollamaTool("helm_release_uninstall", "Uninstall a Helm release.", map[string]api.ToolProperty{
		"namespace": {Type: []string{"string"}, Description: "Release namespace"},
		"release":   {Type: []string{"string"}, Description: "Release name"},
		"dryRun":    {Type: []string{"boolean"}, Description: "Simulate without changes (optional)"},
	}, []string{"namespace", "release"}),
	ollamaTool("helm_release_list", "List all Helm releases in a namespace.", map[string]api.ToolProperty{
		"namespace": {Type: []string{"string"}, Description: "Namespace (empty for all)"},
	}, nil),
	ollamaTool("helm_release_status", "Get the status of a Helm release.", map[string]api.ToolProperty{
		"namespace": {Type: []string{"string"}, Description: "Release namespace"},
		"release":   {Type: []string{"string"}, Description: "Release name"},
	}, []string{"namespace", "release"}),
	ollamaTool("helm_release_history", "Get the revision history of a Helm release.", map[string]api.ToolProperty{
		"namespace": {Type: []string{"string"}, Description: "Release namespace"},
		"release":   {Type: []string{"string"}, Description: "Release name"},
	}, []string{"namespace", "release"}),
	ollamaTool("helm_release_rollback", "Rollback a Helm release to a previous revision.", map[string]api.ToolProperty{
		"namespace": {Type: []string{"string"}, Description: "Release namespace"},
		"release":   {Type: []string{"string"}, Description: "Release name"},
		"revision":  {Type: []string{"integer"}, Description: "Revision number to rollback to"},
	}, []string{"namespace", "release", "revision"}),
	ollamaTool("helm_release_get", "Get detailed information about a Helm release.", map[string]api.ToolProperty{
		"namespace": {Type: []string{"string"}, Description: "Release namespace"},
		"release":   {Type: []string{"string"}, Description: "Release name"},
		"getFormat": {Type: []string{"string"}, Description: "Format: 'all', 'hooks', 'manifest', 'notes', or 'values'"},
	}, []string{"namespace", "release", "getFormat"}),
	ollamaTool("helm_release_link", "Link a Helm release to a repository name for tracking.", map[string]api.ToolProperty{
		"namespace":   {Type: []string{"string"}, Description: "Release namespace"},
		"releaseName": {Type: []string{"string"}, Description: "Release name"},
		"repoName":    {Type: []string{"string"}, Description: "Repository name to link"},
	}, []string{"namespace", "releaseName", "repoName"}),
	ollamaTool("helm_release_get_workloads", "Get the Kubernetes workloads managed by a Helm release.", map[string]api.ToolProperty{
		"namespace": {Type: []string{"string"}, Description: "Release namespace"},
		"release":   {Type: []string{"string"}, Description: "Release name"},
	}, []string{"namespace", "release"}),
}

// --- Helper functions to reduce repetition ---

func newOllamaToolProperties(props map[string]api.ToolProperty) *api.ToolPropertiesMap {
	m := api.NewToolPropertiesMap()
	for k, v := range props {
		m.Set(k, v)
	}
	return m
}

func openaiFunc(name, description string, params openai.FunctionParameters) openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        name,
				Description: openai.String(description),
				Parameters:  params,
			},
		},
	}
}

func anthropicTool(name, description string, properties map[string]any, required []string) anthropic.ToolParam {
	return anthropic.ToolParam{
		Name:        name,
		Description: anthropic.String(description),
		InputSchema: anthropic.ToolInputSchemaParam{
			Type:       "object",
			Properties: properties,
			Required:   required,
		},
	}
}

func ollamaTool(name, description string, props map[string]api.ToolProperty, required []string) api.Tool {
	var properties *api.ToolPropertiesMap
	if props != nil {
		properties = newOllamaToolProperties(props)
	} else {
		properties = api.NewToolPropertiesMap()
	}
	return api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        name,
			Description: description,
			Parameters: api.ToolFunctionParameters{
				Type:       "object",
				Properties: properties,
				Required:   required,
			},
		},
	}
}
