package ai

// helmToolSchemas defines the built-in Helm tools once, provider-neutrally.
// The per-SDK slices below are generated from it via the converters in
// tools_schema.go.
var helmToolSchemas = []toolSchema{
	{
		Name:        "helm_repo_add",
		Description: "Add a Helm chart repository.",
		Props: map[string]toolProp{
			"name":               {Type: "string", Description: "Repository name"},
			"url":                {Type: "string", Description: "Repository URL"},
			"username":           {Type: "string", Description: "Auth username (optional)"},
			"password":           {Type: "string", Description: "Auth password (optional)"},
			"insecureSkipTLS":    {Type: "boolean", Description: "Skip TLS verification (optional)"},
			"passCredentialsAll": {Type: "boolean", Description: "Pass credentials to all domains (optional)"},
		},
		Required: []string{"name", "url"},
	},
	{
		Name:        "helm_repo_patch",
		Description: "Update an existing Helm chart repository configuration.",
		Props: map[string]toolProp{
			"name":               {Type: "string", Description: "Current repository name"},
			"newName":            {Type: "string", Description: "New repository name"},
			"url":                {Type: "string", Description: "New repository URL"},
			"username":           {Type: "string", Description: "Auth username (optional)"},
			"password":           {Type: "string", Description: "Auth password (optional)"},
			"insecureSkipTLS":    {Type: "boolean", Description: "Skip TLS verification (optional)"},
			"passCredentialsAll": {Type: "boolean", Description: "Pass credentials to all domains (optional)"},
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
		Props: map[string]toolProp{
			"name": {Type: "string", Description: "Repository name to remove"},
		},
		Required: []string{"name"},
	},
	{
		Name:        "helm_chart_search",
		Description: "Search for Helm charts across all configured repositories.",
		Props: map[string]toolProp{
			"name": {Type: "string", Description: "Chart name or keyword (optional, empty lists all)"},
		},
	},
	{
		Name:        "helm_chart_install",
		Description: "Install a Helm chart as a new release.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Target namespace"},
			"chart":     {Type: "string", Description: "Chart reference (e.g. 'repo/chart-name')"},
			"release":   {Type: "string", Description: "Release name"},
			"version":   {Type: "string", Description: "Chart version (optional, latest if empty)"},
			"values":    {Type: "string", Description: "YAML values override (optional)"},
			"dryRun":    {Type: "boolean", Description: "Simulate without changes (optional)"},
		},
		Required: []string{"namespace", "chart", "release"},
	},
	{
		Name:        "helm_oci_install",
		Description: "Install a Helm chart from an OCI registry.",
		Props: map[string]toolProp{
			"ociChartUrl": {Type: "string", Description: "OCI chart URL (e.g. 'oci://registry/chart')"},
			"namespace":   {Type: "string", Description: "Target namespace"},
			"release":     {Type: "string", Description: "Release name"},
			"version":     {Type: "string", Description: "Chart version (optional)"},
			"values":      {Type: "string", Description: "YAML values override (optional)"},
			"dryRun":      {Type: "boolean", Description: "Simulate without changes (optional)"},
			"authHost":    {Type: "string", Description: "OCI registry auth host (optional)"},
			"username":    {Type: "string", Description: "OCI registry username (optional)"},
			"password":    {Type: "string", Description: "OCI registry password (optional)"},
		},
		Required: []string{"ociChartUrl", "namespace", "release"},
	},
	{
		Name:        "helm_chart_show",
		Description: "Show information about a Helm chart (values, readme, metadata, or CRDs).",
		Props: map[string]toolProp{
			"chart":      {Type: "string", Description: "Chart reference (e.g. 'repo/chart-name')"},
			"showFormat": {Type: "string", Description: "Format: 'all', 'chart', 'values', 'readme', or 'crds'"},
			"version":    {Type: "string", Description: "Chart version (optional)"},
		},
		Required: []string{"chart", "showFormat"},
	},
	{
		Name:        "helm_chart_versions",
		Description: "List all available versions of a Helm chart.",
		Props: map[string]toolProp{
			"chart": {Type: "string", Description: "Chart reference (e.g. 'repo/chart-name')"},
		},
		Required: []string{"chart"},
	},
	{
		Name:        "helm_release_upgrade",
		Description: "Upgrade an existing Helm release.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Release namespace"},
			"chart":     {Type: "string", Description: "Chart reference (e.g. 'repo/chart-name')"},
			"release":   {Type: "string", Description: "Release name"},
			"version":   {Type: "string", Description: "Chart version (optional)"},
			"values":    {Type: "string", Description: "YAML values override (optional)"},
			"dryRun":    {Type: "boolean", Description: "Simulate without changes (optional)"},
		},
		Required: []string{"namespace", "chart", "release"},
	},
	{
		Name:        "helm_release_uninstall",
		Description: "Uninstall a Helm release.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Release namespace"},
			"release":   {Type: "string", Description: "Release name"},
			"dryRun":    {Type: "boolean", Description: "Simulate without changes (optional)"},
		},
		Required: []string{"namespace", "release"},
	},
	{
		Name:        "helm_release_list",
		Description: "List all Helm releases in a namespace.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Namespace (empty for all)"},
		},
	},
	{
		Name:        "helm_release_status",
		Description: "Get the status of a Helm release.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Release namespace"},
			"release":   {Type: "string", Description: "Release name"},
		},
		Required: []string{"namespace", "release"},
	},
	{
		Name:        "helm_release_history",
		Description: "Get the revision history of a Helm release.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Release namespace"},
			"release":   {Type: "string", Description: "Release name"},
		},
		Required: []string{"namespace", "release"},
	},
	{
		Name:        "helm_release_rollback",
		Description: "Rollback a Helm release to a previous revision.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Release namespace"},
			"release":   {Type: "string", Description: "Release name"},
			"revision":  {Type: "integer", Description: "Revision number to rollback to"},
		},
		Required: []string{"namespace", "release", "revision"},
	},
	{
		Name:        "helm_release_get",
		Description: "Get detailed information about a Helm release.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Release namespace"},
			"release":   {Type: "string", Description: "Release name"},
			"getFormat": {Type: "string", Description: "Format: 'all', 'hooks', 'manifest', 'notes', or 'values'"},
		},
		Required: []string{"namespace", "release", "getFormat"},
	},
	{
		Name:        "helm_release_link",
		Description: "Link a Helm release to a repository name for tracking.",
		Props: map[string]toolProp{
			"namespace":   {Type: "string", Description: "Release namespace"},
			"releaseName": {Type: "string", Description: "Release name"},
			"repoName":    {Type: "string", Description: "Repository name to link"},
		},
		Required: []string{"namespace", "releaseName", "repoName"},
	},
	{
		Name:        "helm_release_get_workloads",
		Description: "Get the Kubernetes workloads managed by a Helm release.",
		Props: map[string]toolProp{
			"namespace": {Type: "string", Description: "Release namespace"},
			"release":   {Type: "string", Description: "Release name"},
		},
		Required: []string{"namespace", "release"},
	},
}

var (
	helmOpenAiTools    = mapSchemas(helmToolSchemas, toolSchema.toOpenAI)
	helmAnthropicTools = mapSchemas(helmToolSchemas, toolSchema.toAnthropic)
	helmOllamaTools    = mapSchemas(helmToolSchemas, toolSchema.toOllama)
)
