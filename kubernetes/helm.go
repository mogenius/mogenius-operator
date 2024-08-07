package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/structs"
	"os"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"gopkg.in/yaml.v2"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
)

var helmCache = cache.New(2*time.Hour, 30*time.Minute) // cache with default expiration time of 2 hours and cleanup interval of 30 minutes

func CreateHelmChart(helmReleaseName string, helmRepoName string, helmRepoUrl string, helmTask structs.HelmTaskEnum, helmChartName string, helmFlags string, successFunc func(), failFunc func(output string, err error)) {
	structs.CreateShellCommandGoRoutine("Add/Update Helm Repo & Execute chart.", fmt.Sprintf("helm repo add %s %s; helm repo update; helm %s %s %s %s", helmRepoName, helmRepoUrl, helmTask, helmReleaseName, helmChartName, helmFlags), successFunc, failFunc)
}

func DeleteHelmChart(job *structs.Job, helmReleaseName string, wg *sync.WaitGroup) {
	structs.CreateShellCommand("helm uninstall", "Uninstall chart", job, fmt.Sprintf("helm uninstall %s", helmReleaseName), wg)
}

func HelmStatus(namespace string, chartname string) release.Status {
	cacheKey := namespace + "/" + chartname
	cacheTime := 1 * time.Second

	// Check if the data is already in the cache
	if cachedData, found := helmCache.Get(cacheKey); found {
		return cachedData.(release.Status)
	}

	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmStatus Init Error: %s", err.Error())
		helmCache.Set(cacheKey, release.StatusUnknown, cacheTime)
		return release.StatusUnknown
	}

	get := action.NewGet(actionConfig)
	chart, err := get.Run(chartname)
	if err != nil && err.Error() != "release: not found" {
		K8sLogger.Errorf("HelmStatus List Error: %s", err.Error())
		helmCache.Set(cacheKey, release.StatusUnknown, cacheTime)
		return release.StatusUnknown
	}

	if chart == nil {
		return release.StatusUnknown
	} else {
		return chart.Info.Status
	}
}

//
//
//
//
// NEW CODE
//
//
//
//

func HelmRepoAdd(helmRepoName string, helmRepoUrl string) (string, error) {
	settings := cli.New()

	// Create a new Helm repository entry
	entry := &repo.Entry{
		Name: helmRepoName,
		URL:  helmRepoUrl,
	}

	// Initialize the file where repositories are stored
	file := settings.RepositoryConfig

	// Load the existing repositories
	repoFile, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to load repository file: %s", err)
	}

	// Check if the repository already exists
	if repoFile.Has(helmRepoName) {
		return fmt.Sprintf("repository name (%s) already exists", helmRepoName), nil
	}

	// Add the new repository entry
	chartRepo, err := repo.NewChartRepository(entry, getter.All(settings))
	if err != nil {
		return "", fmt.Errorf("failed to create new chart repository: %s", err)
	}
	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		return "", fmt.Errorf("failed to download index file: %s", err)
	}

	repoFile.Update(entry)

	// Write the updated repository file
	if err := repoFile.WriteFile(file, 0644); err != nil {
		return "", fmt.Errorf("failed to write repository file: %s", err)
	}

	return "Repo Added", nil
}

func HelmRepoUpdate() (string, error) {
	result := ""
	settings := cli.New()

	// Initialize the file where repositories are stored
	file := settings.RepositoryConfig

	// Load the existing repositories
	repoFile, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return result, fmt.Errorf("failed to load repository file: %s", err)
	}

	// Update the repositories
	for _, re := range repoFile.Repositories {
		chartRepo, err := repo.NewChartRepository(re, getter.All(settings))
		if err != nil {
			result += fmt.Sprintf("failed to create new chart repository: %s\n", err)
			continue
		}
		if _, err := chartRepo.DownloadIndexFile(); err != nil {
			result += fmt.Sprintf("failed to download index file: %s\n", err)
			continue
		}
		result += fmt.Sprintf("Repo Updated: %s\n", re.Name)
	}

	return result, nil
}

func HelmRepoList() ([]*repo.Entry, error) {
	settings := cli.New()

	// Initialize the file where repositories are stored
	file := settings.RepositoryConfig

	// Load the existing repositories
	repoFile, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return []*repo.Entry{}, fmt.Errorf("failed to load repository file: %s", err)
	}

	result := []*repo.Entry{}
	for _, re := range repoFile.Repositories {
		// re.CAFile = ""
		// re.CertFile = ""
		// re.KeyFile = ""
		// re.Username = ""
		// re.Password = ""
		result = append(result, re)
	}

	return result, nil
}

func HelmRepoRemove(helmRepoName string) (string, error) {
	settings := cli.New()

	// Initialize the file where repositories are stored
	file := settings.RepositoryConfig

	// Load the existing repositories
	repoFile, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to load repository file: %s", err)
	}

	// Check if the repository exists
	if !repoFile.Has(helmRepoName) {
		return fmt.Sprintf("repository name (%s) does not exist", helmRepoName), nil
	}

	// Remove the repository entry
	repoFile.Remove(helmRepoName)

	// Write the updated repository file
	if err := repoFile.WriteFile(file, 0644); err != nil {
		return "", fmt.Errorf("failed to write repository file: %s", err)
	}

	return fmt.Sprintf("Repo '%s' removed", helmRepoName), nil
}

func HelmInstall(namespace string, chart string, version string, releaseName string, values string, dryRun bool) (string, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmInstall Init Error: %s\n", err.Error())
		return "", err
	}

	install := action.NewInstall(actionConfig)
	install.DryRun = dryRun
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.Version = version
	install.Timeout = 300 * time.Second

	chartPath, err := install.LocateChart(chart, settings)
	if err != nil {
		K8sLogger.Errorf("HelmInstall LocateChart Error: %s\n", err.Error())
		return "", err
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		K8sLogger.Errorf("HelmInstall Load Error: %s\n", err.Error())
		return "", err
	}

	// Parse the values string into a map
	valuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(values), &valuesMap); err != nil {
		K8sLogger.Errorf("HelmInstall Values Unmarshal Error: %s\n", err.Error())
		return "", err
	}

	release, err := install.Run(chartRequested, valuesMap)
	if err != nil {
		K8sLogger.Errorf("HelmInstall Run Error: %s\n", err.Error())
		return "", err
	}
	if release == nil {
		return "", fmt.Errorf("HelmInstall Error: Release not found")
	}

	return installStatus(*release), nil
}

func HelmUpgrade(namespace string, chart string, version string, releaseName string, values string, dryRun bool) (string, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmUpgrade Init Error: %s\n", err.Error())
		return "", err
	}

	upgrade := action.NewUpgrade(actionConfig)
	upgrade.DryRun = dryRun
	upgrade.Namespace = namespace
	upgrade.Version = version
	upgrade.Timeout = 300 * time.Second

	chartPath, err := upgrade.LocateChart(chart, settings)
	if err != nil {
		K8sLogger.Errorf("HelmUpgrade LocateChart Error: %s\n", err.Error())
		return "", err
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		K8sLogger.Errorf("HelmUpgrade Load Error: %s\n", err.Error())
		return "", err
	}

	// Parse the values string into a map
	valuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(values), &valuesMap); err != nil {
		K8sLogger.Errorf("HelmUpgrade Values Unmarshal Error: %s\n", err.Error())
		return "", err
	}

	release, err := upgrade.Run(releaseName, chartRequested, valuesMap)
	if err != nil {
		K8sLogger.Errorf("HelmUpgrade Run Error: %s\n", err.Error())
		return "", err
	}
	if release == nil {
		return "", fmt.Errorf("HelmUpgrade Error: Release not found")
	}

	return installStatus(*release), nil
}

func HelmUninstall(namespace string, releaseName string, dryRun bool) (string, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmUninstall Init Error: %s\n", err.Error())
		return "", err
	}

	uninstall := action.NewUninstall(actionConfig)
	uninstall.DryRun = dryRun
	_, err := uninstall.Run(releaseName)
	if err != nil {
		K8sLogger.Errorf("HelmUninstall Run Error: %s\n", err.Error())
		return "", err
	}

	return fmt.Sprintf("Release '%s' uninstalled", releaseName), nil
}

func installStatus(rel release.Release) string {
	result := ""
	result += fmt.Sprintf("%s (%s)\n", rel.Name, rel.Info.Status)
	result += fmt.Sprintf("%s\n", rel.Info.Description)
	result += fmt.Sprintf("%s\n", rel.Info.Notes)
	return result
}

func HelmReleaseList(namespace string) ([]*release.Release, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmReleaseList Init Error: %s", err.Error())
		return []*release.Release{}, err
	}

	list := action.NewList(actionConfig)
	releases, err := list.Run()
	if err != nil {
		K8sLogger.Errorf("HelmReleaseList List Error: %s", err.Error())
		return releases, err
	}
	return releases, nil
}

func HelmReleaseStatus(namespace string, chartname string) (string, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmReleaseStatus Init Error: %s", err.Error())
		return "", err
	}

	status := action.NewStatus(actionConfig)
	release, err := status.Run(chartname)
	if err != nil {
		K8sLogger.Errorf("HelmReleaseStatus List Error: %s", err.Error())
		return "", err
	}
	if release == nil {
		return "", fmt.Errorf("HelmReleaseStatus Error: Release not found")
	}
	return statusString(*release), nil
}

func statusString(rel release.Release) string {
	result := ""
	result += fmt.Sprintf("NAME: %s\n", rel.Name)
	result += fmt.Sprintf("LAST DEPLOYED: %s\n", rel.Info.LastDeployed)
	result += fmt.Sprintf("NAMESPACE: %s\n", rel.Namespace)
	result += fmt.Sprintf("STATUS: %s\n", rel.Info.Status)
	result += fmt.Sprintf("REVISION:%d\n", rel.Version)
	result += fmt.Sprintf("CHART: %s-%s\n", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
	return result
}

func HelmChartShow(chartname string, format action.ShowOutputFormat) (string, error) {
	settings := cli.New()

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), "", os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmChartShow Init Error: %s", err.Error())
		return "", err
	}

	// Fetch the chart
	chartPathOptions := action.ChartPathOptions{}
	chartPath, err := chartPathOptions.LocateChart(chartname, settings)
	if err != nil {
		K8sLogger.Errorf("HelmShow LocateChart Error: %s\n", err.Error())
		return "", err
	}

	// Show the chart
	show := action.NewShowWithConfig(format, actionConfig)
	result, err := show.Run(chartPath)
	if err != nil {
		K8sLogger.Errorf("HelmShow Run Error: %s", err.Error())
		return "", err
	}

	return result, nil
}

func HelmHistory(namespace string, releaseName string) ([]*release.Release, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmHistory Init Error: %s", err.Error())
		return []*release.Release{}, err
	}

	history := action.NewHistory(actionConfig)
	history.Max = 10
	releases, err := history.Run(releaseName)
	if err != nil {
		K8sLogger.Errorf("HelmHistory List Error: %s", err.Error())
		return releases, err
	}
	return releases, nil
}

func HelmRollback(namespace string, release string, revision int) (string, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmRollback Init Error: %s", err.Error())
		return "", err
	}

	rollback := action.NewRollback(actionConfig)
	rollback.Version = revision
	err := rollback.Run(release)
	if err != nil {
		K8sLogger.Errorf("HelmRollback Run Error: %s", err.Error())
		return "", err
	}

	return fmt.Sprintf("Rolled back '%s/%s' to revision %d", namespace, release, revision), nil
}

func HelmGet(namespace string, releaseName string, helmGetType structs.HelmGetEnum) (string, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmGet Init Error: %s", err.Error())
		return "", err
	}

	get := action.NewGet(actionConfig)
	release, err := get.Run(releaseName)
	if err != nil && err.Error() != "release: not found" {
		K8sLogger.Errorf("HelmGet List Error: %s", err.Error())
		return "", err
	}

	if release == nil {
		return "", err
	} else {

		switch helmGetType {
		case structs.HelmGetAll:
			return printAllGet(release), nil
		case structs.HelmGetHooks:
			return printHooks(release), nil
		case structs.HelmGetManifest:
			return release.Manifest, nil
		case structs.HelmGetNotes:
			return release.Info.Notes, nil
		case structs.HelmGetValues:
			return yamlString(release.Config), nil

		default:
			return "", fmt.Errorf("HelmGet Error: Unknown HelmGetEnum")
		}
	}
}

func printAllGet(rel *release.Release) string {
	result := ""
	result += fmt.Sprintf("Release Name: %s\n", rel.Name)
	result += fmt.Sprintf("Namespace: %s\n", rel.Namespace)
	result += fmt.Sprintf("Status: %s\n", rel.Info.Status)
	result += fmt.Sprintf("Chart: %s-%s\n", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
	result += fmt.Sprintf("Manifest:\n%s\n", rel.Manifest)
	result += fmt.Sprintf("Notes:\n%s\n", rel.Info.Notes)
	result += fmt.Sprintf("Values:\n%s\n", yamlString(rel.Config))

	result += printHooks(rel)

	return result
}

func printHooks(rel *release.Release) string {
	result := ""
	if rel.Hooks != nil {
		result += "Hooks:\n"
		for _, hook := range rel.Hooks {
			result += fmt.Sprintf("  Name: %s\n", hook.Name)
			result += fmt.Sprintf("  Kind: %s\n", hook.Kind)
			result += fmt.Sprintf("  Manifest: %s\n", hook.Manifest)
			result += fmt.Sprintf("  Events: %v\n", hook.Events)
		}
	}

	return result
}

func yamlString(data map[string]interface{}) string {
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		K8sLogger.Errorf("Error while marshaling. %v", err)
		return ""
	}

	return string(yamlData)
}
