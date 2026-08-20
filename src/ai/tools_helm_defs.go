package ai

import "mogenius-operator/src/ai/aisdk"

// helmAiSDKTools defines the built-in Helm tools in the provider-neutral
// aisdk.Tool format. Each provider converts them to its own SDK type.
var helmAiSDKTools = []aisdk.Tool{
	{
		Name:        "helm_repo_add",
		Description: "Add a Helm chart repository.",
		InputSchema: map[string]any{
			"name":               prop("string", "Repository name"),
			"url":                prop("string", "Repository URL"),
			"username":           prop("string", "Auth username (optional)"),
			"password":           prop("string", "Auth password (optional)"),
			"insecureSkipTLS":    prop("boolean", "Skip TLS verification (optional)"),
			"passCredentialsAll": prop("boolean", "Pass credentials to all domains (optional)"),
		},
		Required: []string{"name", "url"},
	},
	{
		Name:        "helm_repo_patch",
		Description: "Update an existing Helm chart repository configuration.",
		InputSchema: map[string]any{
			"name":               prop("string", "Current repository name"),
			"newName":            prop("string", "New repository name"),
			"url":                prop("string", "New repository URL"),
			"username":           prop("string", "Auth username (optional)"),
			"password":           prop("string", "Auth password (optional)"),
			"insecureSkipTLS":    prop("boolean", "Skip TLS verification (optional)"),
			"passCredentialsAll": prop("boolean", "Pass credentials to all domains (optional)"),
		},
		Required: []string{"name", "newName", "url"},
	},
	{
		Name:        "helm_repo_update",
		Description: "Update all Helm chart repositories to fetch latest chart information.",
	},
	{
		Name:        "helm_repo_list",
		Description: "List all configured Helm chart repositories.",
	},
	{
		Name:        "helm_repo_remove",
		Description: "Remove a Helm chart repository.",
		InputSchema: map[string]any{
			"name": prop("string", "Repository name to remove"),
		},
		Required: []string{"name"},
	},
	{
		Name:        "helm_chart_search",
		Description: "Search for Helm charts across all configured repositories.",
		InputSchema: map[string]any{
			"name": prop("string", "Chart name or keyword (optional, empty lists all)"),
		},
	},
	{
		Name:        "helm_chart_install",
		Description: "Install a Helm chart as a new release.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Target namespace"),
			"chart":     prop("string", "Chart reference (e.g. 'repo/chart-name')"),
			"release":   prop("string", "Release name"),
			"version":   prop("string", "Chart version (optional, latest if empty)"),
			"values":    prop("string", "YAML values override (optional)"),
			"dryRun":    prop("boolean", "Simulate without changes (optional)"),
		},
		Required: []string{"namespace", "chart", "release"},
	},
	{
		Name:        "helm_oci_install",
		Description: "Install a Helm chart from an OCI registry.",
		InputSchema: map[string]any{
			"ociChartUrl": prop("string", "OCI chart URL (e.g. 'oci://registry/chart')"),
			"namespace":   prop("string", "Target namespace"),
			"release":     prop("string", "Release name"),
			"version":     prop("string", "Chart version (optional)"),
			"values":      prop("string", "YAML values override (optional)"),
			"dryRun":      prop("boolean", "Simulate without changes (optional)"),
			"authHost":    prop("string", "OCI registry auth host (optional)"),
			"username":    prop("string", "OCI registry username (optional)"),
			"password":    prop("string", "OCI registry password (optional)"),
		},
		Required: []string{"ociChartUrl", "namespace", "release"},
	},
	{
		Name:        "helm_chart_show",
		Description: "Show information about a Helm chart (values, readme, metadata, or CRDs).",
		InputSchema: map[string]any{
			"chart":      prop("string", "Chart reference (e.g. 'repo/chart-name')"),
			"showFormat": prop("string", "Format: 'all', 'chart', 'values', 'readme', or 'crds'"),
			"version":    prop("string", "Chart version (optional)"),
		},
		Required: []string{"chart", "showFormat"},
	},
	{
		Name:        "helm_chart_versions",
		Description: "List all available versions of a Helm chart.",
		InputSchema: map[string]any{
			"chart": prop("string", "Chart reference (e.g. 'repo/chart-name')"),
		},
		Required: []string{"chart"},
	},
	{
		Name:        "helm_release_upgrade",
		Description: "Upgrade an existing Helm release.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Release namespace"),
			"chart":     prop("string", "Chart reference (e.g. 'repo/chart-name')"),
			"release":   prop("string", "Release name"),
			"version":   prop("string", "Chart version (optional)"),
			"values":    prop("string", "YAML values override (optional)"),
			"dryRun":    prop("boolean", "Simulate without changes (optional)"),
		},
		Required: []string{"namespace", "chart", "release"},
	},
	{
		Name:        "helm_release_uninstall",
		Description: "Uninstall a Helm release.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Release namespace"),
			"release":   prop("string", "Release name"),
			"dryRun":    prop("boolean", "Simulate without changes (optional)"),
		},
		Required: []string{"namespace", "release"},
	},
	{
		Name:        "helm_release_list",
		Description: "List all Helm releases in a namespace.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Namespace (empty for all)"),
		},
	},
	{
		Name:        "helm_release_status",
		Description: "Get the status of a Helm release.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Release namespace"),
			"release":   prop("string", "Release name"),
		},
		Required: []string{"namespace", "release"},
	},
	{
		Name:        "helm_release_history",
		Description: "Get the revision history of a Helm release.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Release namespace"),
			"release":   prop("string", "Release name"),
		},
		Required: []string{"namespace", "release"},
	},
	{
		Name:        "helm_release_rollback",
		Description: "Rollback a Helm release to a previous revision.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Release namespace"),
			"release":   prop("string", "Release name"),
			"revision":  prop("integer", "Revision number to rollback to"),
		},
		Required: []string{"namespace", "release", "revision"},
	},
	{
		Name:        "helm_release_get",
		Description: "Get detailed information about a Helm release.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Release namespace"),
			"release":   prop("string", "Release name"),
			"getFormat": prop("string", "Format: 'all', 'hooks', 'manifest', 'notes', or 'values'"),
		},
		Required: []string{"namespace", "release", "getFormat"},
	},
	{
		Name:        "helm_release_link",
		Description: "Link a Helm release to a repository name for tracking.",
		InputSchema: map[string]any{
			"namespace":   prop("string", "Release namespace"),
			"releaseName": prop("string", "Release name"),
			"repoName":    prop("string", "Repository name to link"),
		},
		Required: []string{"namespace", "releaseName", "repoName"},
	},
	{
		Name:        "helm_release_get_workloads",
		Description: "Get the Kubernetes workloads managed by a Helm release.",
		InputSchema: map[string]any{
			"namespace": prop("string", "Release namespace"),
			"release":   prop("string", "Release name"),
		},
		Required: []string{"namespace", "release"},
	},
}
