package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mogenius-operator/src/helm"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/valkeyclient"

	"helm.sh/helm/v4/pkg/action"
)

var helmToolDefinitions = map[string]func(map[string]any, *ToolContext, valkeyclient.ValkeyClient, *slog.Logger) string{
	"helm_repo_add":              helmRepoAddTool,
	"helm_repo_patch":            helmRepoPatchTool,
	"helm_repo_update":           helmRepoUpdateTool,
	"helm_repo_list":             helmRepoListTool,
	"helm_repo_remove":           helmRepoRemoveTool,
	"helm_chart_search":          helmChartSearchTool,
	"helm_chart_install":         helmChartInstallTool,
	"helm_oci_install":           helmOciInstallTool,
	"helm_chart_show":            helmChartShowTool,
	"helm_chart_versions":        helmChartVersionsTool,
	"helm_release_upgrade":       helmReleaseUpgradeTool,
	"helm_release_uninstall":     helmReleaseUninstallTool,
	"helm_release_list":          helmReleaseListTool,
	"helm_release_status":        helmReleaseStatusTool,
	"helm_release_history":       helmReleaseHistoryTool,
	"helm_release_rollback":      helmReleaseRollbackTool,
	"helm_release_get":           helmReleaseGetTool,
	"helm_release_link":          helmReleaseLinkTool,
	"helm_release_get_workloads": helmReleaseGetWorkloadsTool,
}

func jsonResult(data any) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling result: %v", err)
	}
	return string(b)
}

// --- Repo Tools ---

func helmRepoAddTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can add Helm repos"
	}

	name, _ := args["name"].(string)
	url, _ := args["url"].(string)
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)
	insecureSkipTLS, _ := args["insecureSkipTLS"].(bool)
	passCredentialsAll, _ := args["passCredentialsAll"].(bool)

	logger.Info("Adding Helm repo", "name", name, "url", url)
	result, err := helm.HelmRepoAdd(helm.HelmRepoAddRequest{
		Name: name, Url: url, Username: username, Password: password,
		InsecureSkipTLSverify: insecureSkipTLS, PassCredentialsAll: passCredentialsAll,
	})
	if err != nil {
		return fmt.Sprintf("Error adding Helm repo: %v", err)
	}
	return result
}

func helmRepoPatchTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can patch Helm repos"
	}
	name, _ := args["name"].(string)
	newName, _ := args["newName"].(string)
	url, _ := args["url"].(string)
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)
	insecureSkipTLS, _ := args["insecureSkipTLS"].(bool)
	passCredentialsAll, _ := args["passCredentialsAll"].(bool)

	logger.Info("Patching Helm repo", "name", name, "newName", newName)
	result, err := helm.HelmRepoPatch(helm.HelmRepoPatchRequest{
		Name: name, NewName: newName, Url: url, Username: username, Password: password,
		InsecureSkipTLSverify: insecureSkipTLS, PassCredentialsAll: passCredentialsAll,
	})
	if err != nil {
		return fmt.Sprintf("Error patching Helm repo: %v", err)
	}
	return result
}

func helmRepoUpdateTool(_ map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can update Helm repos"
	}
	logger.Info("Updating all Helm repos")
	result, err := helm.HelmRepoUpdate()
	if err != nil {
		return fmt.Sprintf("Error updating Helm repos: %v", err)
	}
	return jsonResult(result)
}

func helmRepoListTool(_ map[string]any, _ *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	logger.Info("Listing Helm repos")
	result, err := helm.HelmRepoList()
	if err != nil {
		return fmt.Sprintf("Error listing Helm repos: %v", err)
	}
	return jsonResult(result)
}

func helmRepoRemoveTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can remove Helm repos"
	}
	name, _ := args["name"].(string)

	logger.Info("Removing Helm repo", "name", name)
	result, err := helm.HelmRepoRemove(helm.HelmRepoRemoveRequest{Name: name})
	if err != nil {
		return fmt.Sprintf("Error removing Helm repo: %v", err)
	}
	return result
}

// --- Chart Tools ---

func helmChartSearchTool(args map[string]any, _ *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	name, _ := args["name"].(string)

	logger.Info("Searching Helm charts", "name", name)
	result, err := helm.HelmChartSearch(helm.HelmChartSearchRequest{Name: name})
	if err != nil {
		return fmt.Sprintf("Error searching Helm charts: %v", err)
	}
	return jsonResult(result)
}

func helmChartInstallTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can install Helm charts"
	}
	namespace, _ := args["namespace"].(string)

	if !tc.IsNamespaceAllowed(namespace) {
		return fmt.Sprintf("Error: access to namespace %q is not allowed", namespace)
	}

	chart, _ := args["chart"].(string)
	release, _ := args["release"].(string)
	version, _ := args["version"].(string)
	values, _ := args["values"].(string)
	dryRun, _ := args["dryRun"].(bool)

	logger.Info("Installing Helm chart", "chart", chart, "release", release, "namespace", namespace)
	result, err := helm.HelmChartInstall(helm.HelmChartInstallUpgradeRequest{
		Namespace: namespace, Chart: chart, Release: release,
		Version: version, Values: values, DryRun: dryRun,
	})
	if err != nil {
		return fmt.Sprintf("Error installing Helm chart: %v", err)
	}
	return result
}

func helmOciInstallTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can install Helm OCI charts"
	}

	ociChartUrl, _ := args["ociChartUrl"].(string)
	namespace, _ := args["namespace"].(string)

	if !tc.IsNamespaceAllowed(namespace) {
		return fmt.Sprintf("Error: access to namespace %q is not allowed", namespace)
	}

	release, _ := args["release"].(string)
	version, _ := args["version"].(string)
	values, _ := args["values"].(string)
	dryRun, _ := args["dryRun"].(bool)
	authHost, _ := args["authHost"].(string)
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)

	logger.Info("Installing Helm OCI chart", "ociChartUrl", ociChartUrl, "release", release, "namespace", namespace)
	result, err := helm.HelmOciInstall(helm.HelmChartOciInstallUpgradeRequest{
		OCIChartUrl: ociChartUrl, Namespace: namespace, Release: release,
		Version: version, Values: values, DryRun: dryRun,
		AuthHost: authHost, Username: username, Password: password,
	})
	if err != nil {
		return fmt.Sprintf("Error installing OCI Helm chart: %v", err)
	}
	return result
}

func helmChartShowTool(args map[string]any, _ *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	chart, _ := args["chart"].(string)
	showFormat, _ := args["showFormat"].(string)
	version, _ := args["version"].(string)

	logger.Info("Showing Helm chart", "chart", chart, "format", showFormat)
	result, err := helm.HelmChartShow(helm.HelmChartShowRequest{
		Chart: chart, ShowFormat: action.ShowOutputFormat(showFormat), Version: version,
	})
	if err != nil {
		return fmt.Sprintf("Error showing Helm chart: %v", err)
	}
	return result
}

func helmChartVersionsTool(args map[string]any, _ *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	chart, _ := args["chart"].(string)

	logger.Info("Getting Helm chart versions", "chart", chart)
	result, err := helm.HelmChartVersion(helm.HelmChartVersionRequest{Chart: chart})
	if err != nil {
		return fmt.Sprintf("Error getting Helm chart versions: %v", err)
	}
	return jsonResult(result)
}

// --- Release Tools ---

func helmReleaseUpgradeTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can upgrade Helm releases"
	}

	namespace, _ := args["namespace"].(string)
	chart, _ := args["chart"].(string)
	release, _ := args["release"].(string)

	if !tc.IsHelmReleaseAllowed(namespace, release) {
		return fmt.Sprintf("Error: access to release %q in namespace %q is not allowed", release, namespace)
	}

	version, _ := args["version"].(string)
	values, _ := args["values"].(string)
	dryRun, _ := args["dryRun"].(bool)

	logger.Info("Upgrading Helm release", "release", release, "chart", chart, "namespace", namespace)
	result, err := helm.HelmReleaseUpgrade(helm.HelmChartInstallUpgradeRequest{
		Namespace: namespace, Chart: chart, Release: release,
		Version: version, Values: values, DryRun: dryRun,
	})
	if err != nil {
		return fmt.Sprintf("Error upgrading Helm release: %v", err)
	}
	return result
}

func helmReleaseUninstallTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can uninstall Helm releases"
	}

	namespace, _ := args["namespace"].(string)
	release, _ := args["release"].(string)

	if !tc.IsHelmReleaseAllowed(namespace, release) {
		return fmt.Sprintf("Error: access to release %q in namespace %q is not allowed", release, namespace)
	}

	dryRun, _ := args["dryRun"].(bool)

	logger.Info("Uninstalling Helm release", "release", release, "namespace", namespace)
	result, err := helm.HelmReleaseUninstall(helm.HelmReleaseUninstallRequest{
		Namespace: namespace, Release: release, DryRun: dryRun,
	})
	if err != nil {
		return fmt.Sprintf("Error uninstalling Helm release: %v", err)
	}
	return result
}

func helmReleaseListTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	namespace, _ := args["namespace"].(string)

	if !tc.IsNamespaceAllowed(namespace) {
		return fmt.Sprintf("Error: access to namespace %q is not allowed", namespace)
	}

	logger.Info("Listing Helm releases", "namespace", namespace)
	result, err := helm.HelmReleaseList(helm.HelmReleaseListRequest{Namespace: namespace})
	if err != nil {
		return fmt.Sprintf("Error listing Helm releases: %v", err)
	}
	return jsonResult(result)
}

func helmReleaseStatusTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	namespace, _ := args["namespace"].(string)
	release, _ := args["release"].(string)

	if !tc.IsHelmReleaseAllowed(namespace, release) {
		return fmt.Sprintf("Error: access to release %q in namespace %q is not allowed", release, namespace)
	}

	logger.Info("Getting Helm release status", "release", release, "namespace", namespace)
	result, err := helm.HelmReleaseStatus(helm.HelmReleaseStatusRequest{
		Namespace: namespace, Release: release,
	})
	if err != nil {
		return fmt.Sprintf("Error getting Helm release status: %v", err)
	}
	return jsonResult(result)
}

func helmReleaseHistoryTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	namespace, _ := args["namespace"].(string)
	release, _ := args["release"].(string)

	if !tc.IsHelmReleaseAllowed(namespace, release) {
		return fmt.Sprintf("Error: access to release %q in namespace %q is not allowed", release, namespace)
	}

	logger.Info("Getting Helm release history", "release", release, "namespace", namespace)
	result, err := helm.HelmReleaseHistory(helm.HelmReleaseHistoryRequest{
		Namespace: namespace, Release: release,
	})
	if err != nil {
		return fmt.Sprintf("Error getting Helm release history: %v", err)
	}
	return jsonResult(result)
}

func helmReleaseRollbackTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can rollback Helm releases"
	}

	namespace, _ := args["namespace"].(string)
	release, _ := args["release"].(string)

	if !tc.IsHelmReleaseAllowed(namespace, release) {
		return fmt.Sprintf("Error: access to release %q in namespace %q is not allowed", release, namespace)
	}

	revisionFloat, _ := args["revision"].(float64)
	revision := int(revisionFloat)

	logger.Info("Rolling back Helm release", "release", release, "namespace", namespace, "revision", revision)
	result, err := helm.HelmReleaseRollback(helm.HelmReleaseRollbackRequest{
		Namespace: namespace, Release: release, Revision: revision,
	})
	if err != nil {
		return fmt.Sprintf("Error rolling back Helm release: %v", err)
	}
	return result
}

func helmReleaseGetTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	namespace, _ := args["namespace"].(string)
	release, _ := args["release"].(string)

	if !tc.IsHelmReleaseAllowed(namespace, release) {
		return fmt.Sprintf("Error: access to release %q in namespace %q is not allowed", release, namespace)
	}

	getFormat, _ := args["getFormat"].(string)

	logger.Info("Getting Helm release details", "release", release, "namespace", namespace, "format", getFormat)
	result, err := helm.HelmReleaseGet(helm.HelmReleaseGetRequest{
		Namespace: namespace, Release: release, GetFormat: structs.HelmGetEnum(getFormat),
	})
	if err != nil {
		return fmt.Sprintf("Error getting Helm release: %v", err)
	}
	return result
}

func helmReleaseLinkTool(args map[string]any, tc *ToolContext, _ valkeyclient.ValkeyClient, logger *slog.Logger) string {
	if !tc.IsEditor() && !tc.IsAdmin() {
		return "Error: only users with editor or admin roles can link Helm releases to repos"
	}

	namespace, _ := args["namespace"].(string)
	releaseName, _ := args["releaseName"].(string)

	if !tc.IsHelmReleaseAllowed(namespace, releaseName) {
		return fmt.Sprintf("Error: access to release %q in namespace %q is not allowed", releaseName, namespace)
	}

	repoName, _ := args["repoName"].(string)

	logger.Info("Linking Helm release to repo", "release", releaseName, "repo", repoName, "namespace", namespace)
	err := helm.SaveRepoNameToValkey(namespace, releaseName, repoName)
	if err != nil {
		return fmt.Sprintf("Error linking Helm release to repo: %v", err)
	}
	return fmt.Sprintf("Successfully linked release '%s' to repo '%s'", releaseName, repoName)
}

func helmReleaseGetWorkloadsTool(args map[string]any, tc *ToolContext, valkeyClient valkeyclient.ValkeyClient, logger *slog.Logger) string {
	namespace, _ := args["namespace"].(string)
	release, _ := args["release"].(string)

	if !tc.IsHelmReleaseAllowed(namespace, release) {
		return fmt.Sprintf("Error: access to release %q in namespace %q is not allowed", release, namespace)
	}

	logger.Info("Getting Helm release workloads", "release", release, "namespace", namespace)
	result, err := helm.HelmReleaseGetWorkloads(valkeyClient, helm.HelmReleaseGetWorkloadsRequest{
		Namespace: namespace, Release: release,
	})
	if err != nil {
		return fmt.Sprintf("Error getting Helm release workloads: %v", err)
	}
	return jsonResult(result)
}
