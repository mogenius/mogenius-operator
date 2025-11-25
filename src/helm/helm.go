package helm

import (
	"errors"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/logging"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"

	"github.com/patrickmn/go-cache"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	releaser "helm.sh/helm/v4/pkg/release"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
)

const (
	HELM_DATA_HOME   = "helm"
	HELM_CONFIG_HOME = "helm"

	HELM_CACHE_HOME              = "helm/cache"
	HELM_REPOSITORY_CACHE_FOLDER = "helm/cache/repository"
	HELM_REGISTRY_CONFIG_FILE    = "helm/config.json"
	HELM_REPOSITORY_CONFIG_FILE  = "helm/repositories.yaml"

	HELM_PLUGINS = "helm/plugins"

	MAXCHART_VERSIONS = 50
)

var (
	registryConfig   string
	repositoryConfig string
	repositoryCache  string
)

var ErrorRepoAlreadyExists = fmt.Errorf("repository name already exists")
var ErrorRepoFileDoesNotExist = fmt.Errorf("repository.yaml does not exist")

var helmLogger *slog.Logger
var config cfg.ConfigModule
var valkeyClient valkeyclient.ValkeyClient

var helmCache = cache.New(2*time.Hour, 30*time.Minute) // cache with default expiration time of 2 hours and cleanup interval of 30 minutes

func Setup(logManager logging.SlogManager, configModule cfg.ConfigModule, valkey valkeyclient.ValkeyClient) {
	helmLogger = logManager.CreateLogger("helm")
	config = configModule
	valkeyClient = valkey
}

func InitEnvs(configModule cfg.ConfigModule) {
	os.Setenv("HELM_CACHE_HOME", fmt.Sprintf("%s/%s", configModule.Get("MO_HELM_DATA_PATH"), HELM_CACHE_HOME))
	os.Setenv("HELM_CONFIG_HOME", fmt.Sprintf("%s/%s", configModule.Get("MO_HELM_DATA_PATH"), HELM_CONFIG_HOME))
	os.Setenv("HELM_DATA_HOME", fmt.Sprintf("%s/%s", configModule.Get("MO_HELM_DATA_PATH"), HELM_DATA_HOME))
	os.Setenv("HELM_PLUGINS", fmt.Sprintf("%s/%s", configModule.Get("MO_HELM_DATA_PATH"), HELM_PLUGINS))
	os.Setenv("HELM_REGISTRY_CONFIG", fmt.Sprintf("%s/%s", configModule.Get("MO_HELM_DATA_PATH"), HELM_REGISTRY_CONFIG_FILE))
	os.Setenv("HELM_REPOSITORY_CACHE", fmt.Sprintf("%s/%s", configModule.Get("MO_HELM_DATA_PATH"), HELM_REPOSITORY_CACHE_FOLDER))
	os.Setenv("HELM_REPOSITORY_CONFIG", fmt.Sprintf("%s/%s", configModule.Get("MO_HELM_DATA_PATH"), HELM_REPOSITORY_CONFIG_FILE))
	os.Setenv("HELM_LOG_LEVEL", "trace")
}

type HelmRepoAddRequest struct {
	Name string `json:"name" validate:"required"`
	Url  string `json:"url" validate:"required"`
	// Optional fields
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
	InsecureSkipTLSverify bool   `json:"insecureSkipTLSverify,omitempty"`
	PassCredentialsAll    bool   `json:"passCredentialsAll,omitempty"`
}

type HelmRepoPatchRequest struct {
	Name    string `json:"name" validate:"required"`
	NewName string `json:"newName" validate:"required"`
	Url     string `json:"url" validate:"required"`
	// Optional fields
	Username              string `json:"username,omitempty"`
	Password              string `json:"password,omitempty"`
	InsecureSkipTLSverify bool   `json:"insecureSkipTLSverify,omitempty"`
	PassCredentialsAll    bool   `json:"passCredentialsAll,omitempty"`
}

// in valkey release-NS:release-Name {repoName, repoUrl}
type HelmRelease struct {
	Name      string            `json:"name,omitempty"`
	Info      *release.Info     `json:"info,omitempty"`
	Chart     *chart.Chart      `json:"chart,omitempty"`
	Config    map[string]any    `json:"config,omitempty"`
	Manifest  string            `json:"manifest,omitempty"`
	Hooks     []*release.Hook   `json:"hooks,omitempty"`
	Version   int               `json:"version,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"-"`
	RepoName  string            `json:"repoName"`
}

type HelmValkeyRepoName struct {
	RepoName string `json:"repoName"`
}

type HelmRepoRemoveRequest struct {
	Name string `json:"name" validate:"required"`
}

type HelmChartSearchRequest struct {
	Name string `json:"name,omitempty"`
}

type HelmChartInstallUpgradeRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Chart     string `json:"chart" validate:"required"`
	Release   string `json:"release" validate:"required"`
	// Optional fields
	Version string `json:"version,omitempty"`
	Values  string `json:"values,omitempty"`
	DryRun  bool   `json:"dryRun,omitempty"`
}

type HelmChartOciInstallUpgradeRequest struct {
	RegistryUrl string `json:"registryUrl" validate:"required"`
	Namespace   string `json:"namespace" validate:"required"`
	Chart       string `json:"chart" validate:"required"`
	Release     string `json:"release" validate:"required"`
	// Optional fields
	Version string `json:"version,omitempty"`
	Values  string `json:"values,omitempty"`
	DryRun  bool   `json:"dryRun,omitempty"`
	// OCI specific fields
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type HelmChartShowRequest struct {
	Chart      string                  `json:"chart" validate:"required"`
	ShowFormat action.ShowOutputFormat `json:"format" validate:"required"` // "all" "chart" "values" "readme" "crds"
	Version    string                  `json:"version,omitempty"`          // optional, if not set, the latest version will be used
}

type HelmChartVersionRequest struct {
	Chart string `json:"chart" validate:"required"`
}

type HelmReleaseUninstallRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Release   string `json:"release" validate:"required"`
	// Optional fields
	DryRun bool `json:"dryRun,omitempty"`
}

type HelmReleaseListRequest struct {
	Namespace string `json:"namespace"`
}

type HelmReleaseStatusRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Release   string `json:"release" validate:"required"`
}

type HelmReleaseHistoryRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Release   string `json:"release" validate:"required"`
}

type HelmReleaseRollbackRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Release   string `json:"release" validate:"required"`
	Revision  int    `json:"revision" validate:"required"`
}

type HelmReleaseLinkRequest struct {
	Namespace   string `json:"namespace" validate:"required"`
	ReleaseName string `json:"releaseName" validate:"required"`
	RepoName    string `json:"repoName" validate:"required"` // e.g. bitnami/nginx
}

type HelmReleaseGetRequest struct {
	Namespace string              `json:"namespace" validate:"required"`
	Release   string              `json:"release" validate:"required"`
	GetFormat structs.HelmGetEnum `json:"getFormat" validate:"required"` // "all" "hooks" "manifest" "notes" "values"
}

type HelmReleaseGetWorkloadsRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Release   string `json:"release" validate:"required"`

	Whitelist []*utils.ResourceDescriptor `json:"whitelist"`
}

type HelmEntryWithoutPassword struct {
	Name                  string `json:"name"`
	URL                   string `json:"url"`
	InsecureSkipTLSverify bool   `json:"insecure_skip_tls_verify"`
	PassCredentialsAll    bool   `json:"pass_credentials_all"`
}

type HelmEntryStatus struct {
	Entry   *HelmEntryWithoutPassword `json:"entry"`
	Status  string                    `json:"status"` // success, error
	Message string                    `json:"message"`
}

type HelmChartInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
}

type HelmReleaseStatusInfo struct {
	Name         string    `json:"name"`
	LastDeployed time.Time `json:"lastDeployed"`
	Namespace    string    `json:"namespace"`
	Status       string    `json:"status"`
	Version      int       `json:"version"`
	Chart        string    `json:"chart"`
}

// Only for internal usage
func CreateHelmChart(helmReleaseName string, helmRepoName string, helmRepoUrl string, helmChartName string, helmValues string, helmChartVersion string) (output string, err error) {
	data := HelmChartInstallUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Chart:     helmChartName,
		Release:   helmReleaseName,
		Values:    helmValues,
		Version:   helmChartVersion,
	}

	// make sure repo is available
	addResult, err := HelmRepoAdd(HelmRepoAddRequest{
		Name: helmRepoName,
		Url:  helmRepoUrl,
	})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return addResult, fmt.Errorf("failed to add repository: %s", err.Error())
	}

	return HelmChartInstall(data)
}

// Only for internal usage
func DeleteHelmChart(helmReleaseName string, namespace string) (string, error) {
	data := HelmReleaseUninstallRequest{
		Namespace: namespace,
		Release:   helmReleaseName,
	}
	return HelmReleaseUninstall(data)
}

func HelmStatus(namespace string, releaseName string) releasecommon.Status {
	cacheKey := namespace + "/" + releaseName
	cacheTime := 1 * time.Second

	// Check if the data is already in the cache
	if cachedData, found := helmCache.Get(cacheKey); found {
		return cachedData.(releasecommon.Status)
	}

	settings := NewCli()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, ""); err != nil {
		helmLogger.Error("HelmStatus Init", "error", err.Error())
		helmCache.Set(cacheKey, releasecommon.StatusUnknown, cacheTime)
		return releasecommon.StatusUnknown
	}

	get := action.NewGet(actionConfig)
	rel, err := get.Run(releaseName)
	if err != nil && err.Error() != "release: not found" {
		helmLogger.Error("HelmStatus List", "error", err.Error())
		helmCache.Set(cacheKey, releasecommon.StatusUnknown, cacheTime)
		return releasecommon.StatusUnknown
	}

	re, ok := rel.(*release.Release)
	if !ok {
		return releasecommon.StatusUnknown
	}

	if re == nil {
		return releasecommon.StatusUnknown
	} else {
		return re.Info.Status
	}
}

// NEW CODE
func parseHelmEntry(entry *repo.Entry) *HelmEntryWithoutPassword {
	return &HelmEntryWithoutPassword{
		Name:                  entry.Name,
		URL:                   entry.URL,
		InsecureSkipTLSverify: entry.InsecureSkipTLSverify,
		PassCredentialsAll:    entry.PassCredentialsAll,
	}
}

func InitHelmConfig() error {
	_repositoryCache, ok := os.LookupEnv("HELM_REPOSITORY_CACHE")
	assert.Assert(ok)
	_repositoryConfig, ok := os.LookupEnv("HELM_REGISTRY_CONFIG")
	assert.Assert(ok)
	_registryConfig, ok := os.LookupEnv("HELM_REPOSITORY_CONFIG")
	assert.Assert(ok)

	repositoryCache = _repositoryCache
	repositoryConfig = _repositoryConfig
	registryConfig = _registryConfig

	// Set the HELM_HOME environment variable
	path := fmt.Sprintf("%s/%s", config.Get("MO_HELM_DATA_PATH"), HELM_DATA_HOME)

	// create helm home directory if it does not exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			helmLogger.Error("failed to create directory", "path", path, "error", err.Error())
			return err
		}
		helmLogger.Debug("Helm home directory created successfully", "path", path)

		// create cache directory if it does not exist
		if _, err := os.Stat(repositoryCache); os.IsNotExist(err) {
			err := os.MkdirAll(repositoryCache, 0755)
			if err != nil {
				helmLogger.Error("failed to create directory", "path", repositoryCache, "error", err.Error())
				return err
			}
			helmLogger.Debug("Helm cache directory created successfully", "path", repositoryCache)
		}

		// create plugins directory if it does not exist
		pluginsFolder := fmt.Sprintf("%s/%s", config.Get("MO_HELM_DATA_PATH"), HELM_PLUGINS)
		if _, err := os.Stat(pluginsFolder); os.IsNotExist(err) {
			err := os.MkdirAll(pluginsFolder, 0755)
			if err != nil {
				helmLogger.Error("failed to create directory", "path", pluginsFolder, "error", err.Error())
				return err
			}
			helmLogger.Debug("Helm plugins directory created successfully", "path", pluginsFolder)
		}
	}

	if _, err := os.Stat(repositoryConfig); os.IsNotExist(err) {
		destFile, err := os.Create(repositoryConfig)
		if err != nil {
			helmLogger.Error("failed to create repository config", "path", repositoryConfig, "error", err.Error())
		}
		defer destFile.Close()
	}

	_ = restoreRepositoryFileFromValkey()

	// add default repository
	data := HelmRepoAddRequest{
		Name: "mogenius",
		Url:  "https://helm.mogenius.com/public",
	}
	if _, err := HelmRepoAdd(data); err != nil {
		if err != ErrorRepoAlreadyExists {
			helmLogger.Error("failed to add default helm repository", "repoName", data.Name, "repoUrl", data.Url, "error", err.Error())
		}
	}

	if _, err := HelmRepoUpdate(); err != nil {
		helmLogger.Error("failed to update helm repositories", "error", err.Error())
	}

	return nil
}

func NewCli() *cli.EnvSettings {
	settings := cli.New()
	settings.RegistryConfig = registryConfig
	settings.RepositoryConfig = repositoryConfig
	settings.RepositoryCache = repositoryCache
	settings.Debug = true
	return settings
}

func HelmRepoAdd(data HelmRepoAddRequest) (string, error) {
	settings := NewCli()

	// Create a new Helm repository entry
	entry := &repo.Entry{
		Name:                  data.Name,
		URL:                   data.Url,
		Username:              data.Username,
		Password:              data.Password,
		InsecureSkipTLSverify: data.InsecureSkipTLSverify,
		PassCredentialsAll:    data.PassCredentialsAll,
	}

	// Load the existing repositories
	repoFile, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to load repository file: %s", err)
	}

	// Check if the repository already exists
	if repoFile.Has(data.Name) {
		return "", ErrorRepoAlreadyExists
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
	if err := repoFile.WriteFile(settings.RepositoryConfig, 0644); err != nil {
		return "", fmt.Errorf("failed to write repository file: %s", err)
	}

	_ = saveRepositoryFileToValkey()

	return fmt.Sprintf("repository '%s' added", data.Name), nil
}

func HelmRepoPatch(data HelmRepoPatchRequest) (string, error) {
	settings := NewCli()

	// Initialize the file where repositories are stored
	file := settings.RepositoryConfig

	// Load the existing repositories
	repoFile, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to load repository file: %s", err)
	}

	found := false
	for _, re := range repoFile.Repositories {
		if re.Name == data.Name {
			re.Name = data.NewName
			re.URL = data.Url
			re.Username = data.Username
			re.Password = data.Password
			re.InsecureSkipTLSverify = data.InsecureSkipTLSverify
			re.PassCredentialsAll = data.PassCredentialsAll

			chartRepo, err := repo.NewChartRepository(re, getter.All(settings))

			if err != nil {
				return "", err
			}
			if _, err := chartRepo.DownloadIndexFile(); err != nil {
				return "", err
			}
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("repository with name '%s' not found", data.Name)
	}

	if err := repoFile.WriteFile(file, 0644); err != nil {
		return "", fmt.Errorf("failed to write repositories.yaml: %w", err)
	}

	return fmt.Sprintf("repository '%s' updated", data.Name), nil
}

func HelmRepoUpdate() ([]HelmEntryStatus, error) {
	settings := NewCli()

	results := []HelmEntryStatus{}

	// Initialize the file where repositories are stored
	file := settings.RepositoryConfig

	// Load the existing repositories
	repoFile, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return results, fmt.Errorf("failed to load repository file: %s", err)
	}

	// Update the repositories
	for _, re := range repoFile.Repositories {
		chartRepo, err := repo.NewChartRepository(re, getter.All(settings))

		if err != nil {
			results = append(results, HelmEntryStatus{Entry: parseHelmEntry(re), Status: "error", Message: fmt.Sprintf("failed to create new chart repository: %s", err.Error())})
			continue
		}
		if _, err := chartRepo.DownloadIndexFile(); err != nil {
			results = append(results, HelmEntryStatus{Entry: parseHelmEntry(re), Status: "error", Message: fmt.Sprintf("failed to download index file: %s", err.Error())})
			continue
		}
		results = append(results, HelmEntryStatus{Entry: parseHelmEntry(re), Status: "success", Message: fmt.Sprintf("repository '%s' updated", re.Name)})
	}

	_ = saveRepositoryFileToValkey()

	return results, nil
}

func HelmRepoList() ([]*HelmEntryWithoutPassword, error) {
	settings := NewCli()

	// Initialize the file where repositories are stored
	file := settings.RepositoryConfig

	// Load the existing repositories
	repoFile, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return []*HelmEntryWithoutPassword{}, fmt.Errorf("failed to load repository file: %s", err)
	}

	results := []*HelmEntryWithoutPassword{}
	for _, re := range repoFile.Repositories {
		results = append(results, parseHelmEntry(re))
	}

	return results, nil
}

func HelmRepoRemove(data HelmRepoRemoveRequest) (string, error) {
	settings := NewCli()

	// Initialize the file where repositories are stored
	file := settings.RepositoryConfig

	// Load the existing repositories
	repoFile, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to load repository file: %s", err)
	}

	// Check if the repository exists
	if !repoFile.Has(data.Name) {
		return "", fmt.Errorf("repository name (%s) does not exist", data.Name)
	}

	// Remove the repository entry
	repoFile.Remove(data.Name)

	// Write the updated repository file
	if err := repoFile.WriteFile(file, 0644); err != nil {
		return "", fmt.Errorf("failed to write repository file: %s", err)
	}

	_ = saveRepositoryFileToValkey()

	return fmt.Sprintf("repository '%s' removed", data.Name), nil
}

func HelmChartSearch(data HelmChartSearchRequest) ([]HelmChartInfo, error) {
	settings := NewCli()

	repositoriesFile, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil {
		helmLogger.Error("Failed to load repositories file", "error", err.Error())
		shutdown.SendShutdownSignal(true)
		select {}
	}

	var allCharts []HelmChartInfo

	for _, repoEntry := range repositoriesFile.Repositories {
		cacheIndexFile := filepath.Join(settings.RepositoryCache, fmt.Sprintf("%s-index.yaml", repoEntry.Name))

		indexFile, err := repo.LoadIndexFile(cacheIndexFile)
		if err != nil {
			helmLogger.Debug("failed to load repo index file", "repoName", repoEntry.Name, "error", err.Error())
			continue
		}

		for _, chartVersions := range indexFile.Entries {
			for _, chartVersion := range chartVersions {
				allCharts = append(allCharts, HelmChartInfo{
					Name:        fmt.Sprintf("%s/%s", repoEntry.Name, chartVersion.Metadata.Name),
					Version:     chartVersion.Metadata.Version,
					AppVersion:  chartVersion.Metadata.AppVersion,
					Description: chartVersion.Metadata.Description,
				})
				break // only take the first version
			}
		}
	}

	filteredCharts := filterCharts(allCharts, data.Name)
	return filteredCharts, nil
}

// filterCharts filters charts based on the query
func filterCharts(charts []HelmChartInfo, query string) []HelmChartInfo {
	var result []HelmChartInfo
	query = strings.ToLower(query)
	for _, chart := range charts {
		if strings.Contains(strings.ToLower(chart.Name), query) ||
			strings.Contains(strings.ToLower(chart.Description), query) {
			result = append(result, chart)
		}
	}
	return result
}

func HelmChartShow(data HelmChartShowRequest) (string, error) {
	settings := NewCli()

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), "", ""); err != nil {
		helmLogger.Error("HelmChartShow Init", "error", err.Error())
		return "", err
	}

	// Fetch the chart
	chartPathOptions := action.ChartPathOptions{}
	chartPathOptions.Version = data.Version
	chartPath, err := chartPathOptions.LocateChart(data.Chart, settings)
	if err != nil {
		helmLogger.Error("HelmShow LocateChart", "error", err.Error())
		return "", err
	}

	// Show the chart
	show := action.NewShow(data.ShowFormat, actionConfig)
	result, err := show.Run(chartPath)
	if err != nil {
		helmLogger.Error("HelmShow Run", "error", err.Error())
		return "", err
	}
	return result, nil
}

func HelmChartVersion(data HelmChartVersionRequest) ([]HelmChartInfo, error) {
	settings := NewCli()

	repositoriesFile, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil {
		helmLogger.Error("failed to load repositories file", "error", err.Error())
		shutdown.SendShutdownSignal(true)
		select {}
	}

	var allCharts []HelmChartInfo

	split := strings.Split(data.Chart, "/")
	if len(split) != 2 {
		return nil, fmt.Errorf("invalid chart name %s", data.Chart)
	}
	repoName := split[0]
	chartName := split[1]

	for _, repoEntry := range repositoriesFile.Repositories {
		if repoEntry.Name != repoName {
			continue
		}
		cacheIndexFile := filepath.Join(settings.RepositoryCache, fmt.Sprintf("%s-index.yaml", repoEntry.Name))

		indexFile, err := repo.LoadIndexFile(cacheIndexFile)
		if err != nil {
			helmLogger.Debug("Error loading repo index file", "repoName", repoEntry.Name, "error", err.Error())
			continue
		}

		for _, chartVersions := range indexFile.Entries {
			for _, chartVersion := range chartVersions {
				if chartVersion.Metadata.Name != chartName {
					continue
				}
				if len(allCharts) > MAXCHART_VERSIONS {
					break
				}
				allCharts = append(allCharts, HelmChartInfo{
					Name:        fmt.Sprintf("%s/%s", repoEntry.Name, chartVersion.Metadata.Name),
					Version:     chartVersion.Metadata.Version,
					AppVersion:  chartVersion.Metadata.AppVersion,
					Description: chartVersion.Metadata.Description,
				})
			}
		}
	}
	return allCharts, nil
}

func HelmOciInstall(data HelmChartOciInstallUpgradeRequest) (result string, err error) {
	defer func() {
		if err != nil {
			cleanReleaseLogs(data.Namespace, data.Release)
		}
	}()

	settings := NewCli()
	settings.Debug = false
	settings.SetNamespace(data.Namespace)
	helmLogger.Info("Setting up Helm OCI installation...", "releaseName", data.Release, "namespace", data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmOCIInstall Init",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	if !registry.IsOCI(data.RegistryUrl) {
		return "", fmt.Errorf("non-OCI charts are not supported in OCI installation")
	}

	// Pull the OCI chart
	chartPath, err := pullOCIChart(data, settings)
	if err != nil {
		helmLogger.Error("HelmOCIInstall Pull",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	// Load the chart from the pulled location
	helmLogger.Info("Loading pulled OCI chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		helmLogger.Error("HelmOCIInstall Load",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	// Parse the values string into a map
	valuesMap := map[string]any{}
	if err := yaml.Unmarshal([]byte(data.Values), &valuesMap); err != nil {
		helmLogger.Error("failed to Unmarshal HelmOCIInstall Values",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	// Install the pulled chart
	helmLogger.Info("Installing OCI chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	install := action.NewInstall(actionConfig)
	if data.DryRun {
		install.DryRunStrategy = action.DryRunServer
	}
	install.ReleaseName = data.Release
	install.Namespace = data.Namespace
	install.Version = data.Version
	install.WaitStrategy = kube.StatusWatcherStrategy
	install.Timeout = 300 * time.Second

	rel, err := install.Run(chartRequested, valuesMap)
	if err != nil {
		helmLogger.Error("HelmOCIInstall Run",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	re, ok := rel.(*release.Release)
	if !ok {
		return "", errors.New("HelmOCIInstall Error: Release type assertion failed")
	}

	helmLogger.Info(installStatus(*re), "releaseName", data.Release, "namespace", data.Namespace)
	return installStatus(*re), nil
}

func pullOCIChart(data HelmChartOciInstallUpgradeRequest, settings *cli.EnvSettings) (downloadedTo string, err error) {
	chartRef := data.RegistryUrl + "/" + data.Chart
	if data.Version != "" {
		chartRef = fmt.Sprintf("%s:%s", data.Chart, data.Version)
	}

	actionConfig, err := initActionConfigList(settings, true)
	if err != nil {
		return "", fmt.Errorf("failed to init action config: %w", err)
	}

	registryClient, err := newRegistryClient(settings, false)
	if err != nil {
		return "", fmt.Errorf("failed to created registry client: %w", err)
	}
	actionConfig.RegistryClient = registryClient

	// check auth if needed
	if data.Username != "" || data.Password != "" {
		return "", fmt.Errorf("OCI AUTH currently not supported")
		// TODO: Uncomment this when OCI auth is supported
		// err = registryClient.Login(
		// 	data.RegistryUrl,
		// 	registry.LoginOptBasicAuth(data.Username, data.Password),
		// )
		// if err != nil {
		// 	helmLogger.Error("OCI registry login failed",
		// 		"releaseName", data.Release,
		// 		"namespace", data.Namespace,
		// 		"error", err.Error(),
		// 	)
		// 	return "", err
		// }
	}

	tempDir, err := os.MkdirTemp("", "helm-pull")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	pullClient := action.NewPull(action.WithConfig(actionConfig))
	pullClient.DestDir = tempDir
	pullClient.Settings = settings
	pullClient.Version = data.Version
	pullClient.Untar = true
	pullClient.Devel = false

	_, err = pullClient.Run(chartRef)
	if err != nil {
		return "", fmt.Errorf("failed to pull chart: %w", err)
	}

	return tempDir + "/" + data.Chart, nil
}

func initActionConfigList(settings *cli.EnvSettings, allNamespaces bool) (*action.Configuration, error) {

	actionConfig := new(action.Configuration)

	namespace := func() string {
		// For list action, you can pass an empty string instead of settings.Namespace() to list
		// all namespaces
		if allNamespaces {
			return ""
		}
		return settings.Namespace()
	}()

	if err := actionConfig.Init(
		settings.RESTClientGetter(),
		namespace,
		""); err != nil {
		return nil, err
	}

	return actionConfig, nil
}

func newRegistryClient(settings *cli.EnvSettings, plainHTTP bool) (*registry.Client, error) {
	opts := []registry.ClientOption{
		registry.ClientOptEnableCache(true),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	}
	if plainHTTP {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}

	// Create a new registry client
	registryClient, err := registry.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}

func HelmChartInstall(data HelmChartInstallUpgradeRequest) (result string, err error) {
	defer func() {
		if err != nil {
			cleanReleaseLogs(data.Namespace, data.Release)
		}
	}()

	settings := NewCli()
	settings.SetNamespace(data.Namespace)
	settings.Debug = true

	helmLogger.Info("Updating repos ...", "releaseName", data.Release, "namespace", data.Namespace)
	if _, err := HelmRepoUpdate(); err != nil {
		helmLogger.Error("failed to update helm repositories", "error", err.Error())
	}

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmInstall Init",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	if registry.IsOCI(data.Chart) {
		return "", fmt.Errorf("OCI charts are not supported")
	}

	install := action.NewInstall(actionConfig)
	if data.DryRun {
		install.DryRunStrategy = action.DryRunServer
	}
	install.ReleaseName = data.Release
	install.Namespace = data.Namespace
	install.Version = data.Version
	install.WaitStrategy = kube.StatusWatcherStrategy
	install.Timeout = 300 * time.Second
	install.Devel = true

	helmLogger.Info("Locating chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	chartPath, err := install.LocateChart(data.Chart, settings)
	if err != nil {
		helmLogger.Error("HelmInstall LocateChart",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	helmLogger.Info("Loading chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		helmLogger.Error("HelmInstall Load",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	// Parse the values string into a map
	valuesMap := map[string]any{}
	if err := yaml.Unmarshal([]byte(data.Values), &valuesMap); err != nil {
		helmLogger.Error("failed to Unmarshal HelmInstall Values",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	helmLogger.Info("Installing chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	re, err := install.Run(chartRequested, valuesMap)
	if err != nil {
		helmLogger.Error("HelmInstall Run",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}
	if re == nil {
		return "", fmt.Errorf("HelmInstall Error: Release not found")
	}

	helmLogger.Info(installStatus(re), "releaseName", data.Release, "namespace", data.Namespace)

	err = SaveRepoNameToValkey(data.Namespace, data.Release, data.Chart)
	if err != nil {
		helmLogger.Error("failed to SaveRepoNameToValkey",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	return installStatus(re), nil
}

func HelmReleaseUpgrade(data HelmChartInstallUpgradeRequest) (result string, err error) {
	defer func() {
		if err != nil {
			cleanReleaseLogs(data.Namespace, data.Release)
		}
	}()

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	helmLogger.Info("Updating repos ...", "releaseName", data.Release, "namespace", data.Namespace)
	if _, err := HelmRepoUpdate(); err != nil {
		helmLogger.Error("failed to update helm repositories", "error", err.Error())
	}

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmUpgrade Init", "releaseName", data.Release, "namespace", data.Namespace, "error", err.Error())
		return "", err
	}

	upgrade := action.NewUpgrade(actionConfig)
	if data.DryRun {
		upgrade.DryRunStrategy = action.DryRunServer
	}
	upgrade.WaitStrategy = kube.StatusWatcherStrategy
	upgrade.Namespace = data.Namespace
	upgrade.Version = data.Version
	upgrade.Timeout = 300 * time.Second
	upgrade.Devel = true

	helmLogger.Info("Locating chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	chartPath, err := upgrade.LocateChart(data.Chart, settings)
	if err != nil {
		helmLogger.Error("HelmUpgrade LocateChart",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	helmLogger.Info("Loading chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		helmLogger.Error("HelmUpgrade Load",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	// Parse the values string into a map
	valuesMap := map[string]any{}
	if err := yaml.Unmarshal([]byte(data.Values), &valuesMap); err != nil {
		helmLogger.Error("failed to Unmarshal HelmUpgrade Values",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	helmLogger.Info("Upgrading chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	re, err := upgrade.Run(data.Release, chartRequested, valuesMap)
	if err != nil {
		helmLogger.Error("HelmUpgrade Run failed",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
	}
	if re == nil {
		return "", fmt.Errorf("HelmUpgrade Error: Release not found")
	}

	err = SaveRepoNameToValkey(data.Namespace, data.Release, data.Chart)
	if err != nil {
		helmLogger.Error("failed to SaveRepoNameToValkey",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	return installStatus(re), nil
}

func HelmReleaseUninstall(data HelmReleaseUninstallRequest) (result string, err error) {
	defer func() {
		if err != nil {
			cleanReleaseLogs(data.Namespace, data.Release)
		}
	}()

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmUninstall Init",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	helmLogger.Info("Uninstalling chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	uninstall := action.NewUninstall(actionConfig)
	uninstall.DryRun = data.DryRun
	uninstall.WaitStrategy = kube.StatusWatcherStrategy
	_, err = uninstall.Run(data.Release)
	if err != nil {
		helmLogger.Error("HelmUninstall Run",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	_ = DeleteRepoNameFromValkey(data.Namespace, data.Release)

	return fmt.Sprintf("Release '%s' uninstalled", data.Release), nil
}

// delete release from logs (otherwise it will be kept forever)
func cleanReleaseLogs(namespace string, release string) {
	err := valkeyClient.DeleteFromSortedListWithNsAndReleaseName(namespace, release, "logs:helm")
	if err != nil {
		helmLogger.Error("failed to delete helm release logs", "releaseName", release, "namespace", namespace, "error", err.Error())
	}
}

func installStatus(rel releaser.Releaser) string {
	re, ok := rel.(*release.Release)
	if !ok {
		return "Error: unable to get release details"
	}
	result := "\n"
	result += fmt.Sprintf("%s (%s)\n", re.Name, re.Info.Status)
	result += fmt.Sprintf("%s\n", re.Info.Description)
	result += fmt.Sprintf("🗒️ Notes:\n%s\n", re.Info.Notes)
	return result
}

func HelmReleaseList(data HelmReleaseListRequest) ([]*HelmRelease, error) {
	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmReleaseList Init", "namespace", data.Namespace, "error", err.Error())
		return []*HelmRelease{}, err
	}

	list := action.NewList(actionConfig)
	list.StateMask = action.ListAll
	releases, err := list.Run()
	result := []*HelmRelease{}
	// remove unnecessary fields
	for _, aRelease := range releases {
		re, ok := aRelease.(*release.Release)
		if !ok {
			continue
		}

		re.Chart.Files = nil
		re.Chart.Templates = nil
		re.Chart.Values = nil
		re.Manifest = ""

		rep, _ := GetRepoNameFromValkey(re.Name, re.Namespace)
		repoName := ""
		if rep != nil {
			repoName = strings.Replace(rep.RepoName, "/"+re.Chart.Metadata.Name, "", 1)
		}

		result = append(result, helmReleaseFromRelease(re, repoName))
	}
	if err != nil {
		helmLogger.Error("HelmReleaseList List", "namespace", data.Namespace, "error", err.Error())
		return result, err
	}
	return result, nil
}

func HelmReleaseStatus(data HelmReleaseStatusRequest) (*HelmReleaseStatusInfo, error) {
	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmReleaseStatus Init", "releaseName", data.Release, "namespace", data.Namespace, "error", err.Error())
		return nil, err
	}

	status := action.NewStatus(actionConfig)
	rel, err := status.Run(data.Release)
	if err != nil {
		helmLogger.Error("HelmReleaseStatus List", "releaseName", data.Release, "namespace", data.Namespace, "error", err.Error())
		return nil, err
	}
	if rel == nil {
		return nil, fmt.Errorf("HelmReleaseStatus Error: Release not found")
	}

	re, ok := rel.(*release.Release)
	if !ok {
		return nil, fmt.Errorf("HelmReleaseStatus Error: unable to get release details")
	}

	helmReleaseStatusInfo := HelmReleaseStatusInfo{
		Name:         re.Name,
		LastDeployed: re.Info.LastDeployed,
		Namespace:    re.Namespace,
		Status:       re.Info.Status.String(),
		Version:      re.Version,
		Chart:        fmt.Sprintf("%s-%s", re.Chart.Metadata.Name, re.Chart.Metadata.Version),
	}

	return &helmReleaseStatusInfo, nil
}

func HelmReleaseHistory(data HelmReleaseHistoryRequest) ([]release.Release, error) {
	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmHistory Init",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return []release.Release{}, err
	}

	history := action.NewHistory(actionConfig)
	history.Max = 10
	releasers, err := history.Run(data.Release)
	if err != nil {
		helmLogger.Error("HelmHistory List",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return []release.Release{}, err
	}

	releases := []release.Release{}
	for _, rel := range releasers {
		re, ok := rel.(*release.Release)
		if !ok {
			return []release.Release{}, fmt.Errorf("HelmReleaseHistory Error: unable to get release details")
		}
		releases = append(releases, *re)
	}

	return releases, nil
}

func HelmReleaseRollback(data HelmReleaseRollbackRequest) (string, error) {
	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmRollback Init", "releaseName", data.Release, "namespace", data.Namespace, "error", err.Error())
		return "", err
	}

	rollback := action.NewRollback(actionConfig)
	rollback.Version = data.Revision
	err := rollback.Run(data.Release)
	if err != nil {
		helmLogger.Error("HelmRollback Run", "releaseName", data.Release, "namespace", data.Namespace, "error", err.Error())
		return "", err
	}

	return fmt.Sprintf("Rolled back '%s/%s' to revision %d", data.Namespace, data.Release, data.Revision), nil
}

func HelmReleaseGet(data HelmReleaseGetRequest) (string, error) {
	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmGet Init", "releaseName", data.Release, "namespace", data.Namespace, "error", err.Error())
		return "", err
	}

	get := action.NewGet(actionConfig)
	rel, err := get.Run(data.Release)
	if err != nil && err.Error() != "release: not found" {
		helmLogger.Error("HelmGet List", "releaseName", data.Release, "namespace", data.Namespace, "error", err.Error())
		return "", err
	}

	re, ok := rel.(*release.Release)
	if !ok {
		return "", fmt.Errorf("HelmGet Error: unable to get release details")
	}

	if re == nil {
		return "", err
	} else {
		switch data.GetFormat {
		case structs.HelmGetAll:
			return printAllGet(re), nil
		case structs.HelmGetHooks:
			return printHooks(re), nil
		case structs.HelmGetManifest:
			return re.Manifest, nil
		case structs.HelmGetNotes:
			return re.Info.Notes, nil
		case structs.HelmGetValues:
			return yamlString(re.Config), nil

		default:
			return "", fmt.Errorf("HelmGet Error: Unknown HelmGetEnum")
		}
	}
}

func IsManagedByHelmRelease(labels map[string]string, annotations map[string]string, releaseName string) bool {
	labelManagedByHelm := labels != nil && labels["app.kubernetes.io/managed-by"] == "Helm"
	labelInstanceRelease := labels != nil && labels["app.kubernetes.io/instance"] == releaseName
	annotationReleaseName := annotations != nil && annotations["meta.helm.sh/release-name"] == releaseName

	return annotationReleaseName || (labelManagedByHelm && (labelInstanceRelease || annotationReleaseName))
}

func HelmReleaseGetWorkloads(valkeyClient valkeyclient.ValkeyClient, data HelmReleaseGetWorkloadsRequest) ([]unstructured.Unstructured, error) {
	workloads, err := store.SearchResourceByNamespace(valkeyClient, data.Namespace, data.Whitelist)
	if err != nil {
		return nil, err
	}

	var results []unstructured.Unstructured
	appendedWorkloadUIds := make(map[types.UID]bool)
	var replicaSets []unstructured.Unstructured
	replicaSetsFetched := false

	for _, workload := range workloads {
		if appendedWorkloadUIds[workload.GetUID()] {
			continue
		}

		if workload.GetKind() == "Pod" {
			if !replicaSetsFetched {
				replicaSets, err = store.SearchResourceByKeyParts(valkeyClient, utils.ReplicaSetResource.ApiVersion, utils.ReplicaSetResource.Kind, data.Namespace)
				if errors.Is(err, store.ErrNotFound) {
					replicaSets = nil
				}
				replicaSetsFetched = true
			}

			for _, replicaset := range replicaSets {
				for _, ownerReference := range workload.GetOwnerReferences() {
					if ownerReference.UID == replicaset.GetUID() {
						if IsManagedByHelmRelease(replicaset.GetLabels(), replicaset.GetAnnotations(), data.Release) {
							results = append(results, workload)
							appendedWorkloadUIds[workload.GetUID()] = true
							break
						}

					}
				}
			}
			continue
		}

		if IsManagedByHelmRelease(workload.GetLabels(), workload.GetAnnotations(), data.Release) {
			results = append(results, workload)
			appendedWorkloadUIds[workload.GetUID()] = true
		}
	}

	return results, nil
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

func yamlString(data map[string]any) string {
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		helmLogger.Error("failed to Marshal", "error", err.Error())
		return ""
	}

	return string(yamlData)
}

func saveRepositoryFileToValkey() error {
	repoFile, err := repo.LoadFile(repositoryConfig)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load repository file: %s", err)
	}

	yamlData, err := yaml.Marshal(repoFile)
	if err != nil {
		return fmt.Errorf("failed to marshal repositories.yaml: %w", err)
	}

	err = valkeyClient.Set(string(yamlData), 0, "helm", "repositories.yaml")
	if err != nil {
		return err
	}

	return nil
}

func restoreRepositoryFileFromValkey() error {
	data, err := valkeyClient.Get("helm", "repositories.yaml")
	if err != nil {
		return fmt.Errorf("failed to get repositories.yaml from valkey: %s", err.Error())
	}
	// key does not exist in valkey (this is ok)
	if data == "" {
		return ErrorRepoFileDoesNotExist
	}

	err = os.WriteFile(repositoryConfig, []byte(data), 0644)
	if err != nil {
		return fmt.Errorf("failed to write repositories.yaml: %s", err.Error())
	}

	return nil
}

func SaveRepoNameToValkey(namespace, releaseName, repoName string) error {
	data := HelmValkeyRepoName{
		RepoName: repoName,
	}

	err := valkeyClient.SetObject(data, 0, "helm-repos", namespace, releaseName)
	if err != nil {
		return fmt.Errorf("failed to save repository to valkey: %s", err.Error())
	}

	return nil
}

func DeleteRepoNameFromValkey(namespace, releaseName string) error {
	err := valkeyClient.DeleteSingle("helm-repos", namespace, releaseName)
	if err != nil {
		return fmt.Errorf("failed to delete repository from valkey: %s", err.Error())
	}

	return nil
}

func GetRepoNameFromValkey(releaseName, namespace string) (*HelmValkeyRepoName, error) {
	data, err := valkeyclient.GetObjectForKey[HelmValkeyRepoName](valkeyClient, "helm-repos", namespace, releaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository from valkey: %s", err.Error())
	}
	return data, nil
}

func helmReleaseFromRelease(release *release.Release, repoName string) *HelmRelease {
	return &HelmRelease{
		Name:      release.Name,
		Info:      release.Info,
		Chart:     release.Chart,
		Config:    release.Config,
		Manifest:  release.Manifest,
		Hooks:     release.Hooks,
		Version:   release.Version,
		Namespace: release.Namespace,
		Labels:    release.Labels,
		RepoName:  repoName,
	}
}
