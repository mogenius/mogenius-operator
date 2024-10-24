package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/patrickmn/go-cache"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
)

const (
	HELM_DATA_HOME   = "helm"
	HELM_CONFIG_HOME = "helm"

	HELM_REGISTRY_CONFIG_FILE = "helm/config.json"

	HELM_CACHE_HOME              = "helm/cache"
	HELM_REPOSITORY_CACHE_FOLDER = "helm/cache/repository"

	HELM_REPOSITORY_CONFIG_FILE = "helm/repositories.yaml"

	HELM_PLUGINS = "helm/plugins"

	MAXCHART_VERSIONS = 50
)

var (
	registryConfig   string
	repositoryConfig string
	repositoryCache  string
)

var helmCache = cache.New(2*time.Hour, 30*time.Minute) // cache with default expiration time of 2 hours and cleanup interval of 30 minutes

var HelmLogger = log.WithField("component", structs.ComponentHelm)

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

type HelmRepoRemoveRequest struct {
	Name string `json:"name" validate:"required"`
}

type HelmChartSearchRequest struct {
	Name string `json:"name,omitempty"`
}

type HelmChartInstallRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Chart     string `json:"chart" validate:"required"`
	Release   string `json:"release" validate:"required"`
	// Optional fields
	Version string `json:"version,omitempty"`
	Values  string `json:"values,omitempty"`
	DryRun  bool   `json:"dryRun,omitempty"`
}

type HelmChartShowRequest struct {
	Chart      string                  `json:"chart" validate:"required"`
	ShowFormat action.ShowOutputFormat `json:"format" validate:"required"` // "all" "chart" "values" "readme" "crds"
}

type HelmChartVersionRequest struct {
	Chart string `json:"chart" validate:"required"`
}

type HelmReleaseUpgradeRequest struct {
	Namespace string `json:"namespace" validate:"required"`
	Chart     string `json:"chart" validate:"required"`
	Release   string `json:"release" validate:"required"`
	// Optional fields
	Version string `json:"version,omitempty"`
	Values  string `json:"values,omitempty"`
	DryRun  bool   `json:"dryRun,omitempty"`
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

type HelmReleaseGetRequest struct {
	Namespace string              `json:"namespace" validate:"required"`
	Release   string              `json:"release" validate:"required"`
	GetFormat structs.HelmGetEnum `json:"getFormat" validate:"required"` // "all" "hooks" "manifest" "notes" "values"
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
func CreateHelmChart(helmReleaseName string, helmRepoName string, helmRepoUrl string, helmChartName string, helmValues string) (output string, err error) {
	data := HelmChartInstallRequest{
		Namespace: utils.CONFIG.Kubernetes.OwnNamespace,
		Chart:     helmChartName,
		Release:   helmReleaseName,
		Values:    helmValues,
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

//func CheckHelmRepoExists(repoURL string, username string, password string) error {
//	indexURL := fmt.Sprintf("%s/index.yaml", repoURL)
//
//	client := http.Client{
//		Timeout: 10 * time.Second,
//	}
//
//	req, err := http.NewRequest("GET", indexURL, nil)
//	if err != nil {
//		return fmt.Errorf("failed to create request: %w", err)
//	}
//
//	if username != "" && password != "" {
//		auth := fmt.Sprintf("%s:%s", username, password)
//		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
//		req.Header.Add("Authorization", "Basic "+encodedAuth)
//	}
//
//	resp, err := client.Do(req)
//	if err != nil {
//		return fmt.Errorf("failed to fetch index.yaml: %w", err)
//	}
//	defer resp.Body.Close()
//
//	if resp.StatusCode != http.StatusOK {
//		return fmt.Errorf("repository index.yaml not found, status code: %d", resp.StatusCode)
//	}
//
//	body, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		return fmt.Errorf("failed to read response body: %w", err)
//	}
//
//	var indexFile IndexFile
//	if err := yaml.Unmarshal(body, &indexFile); err != nil {
//		return fmt.Errorf("invalid YAML format in index.yaml: %w", err)
//	}
//
//	if indexFile.APIVersion == "" || indexFile.Entries == nil {
//		return fmt.Errorf("invalid Helm repository index format")
//	}
//
//	return nil
//}

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
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), HelmLogger.Infof); err != nil {
		HelmLogger.Errorf("HelmStatus Init Error: %s", err.Error())
		helmCache.Set(cacheKey, release.StatusUnknown, cacheTime)
		return release.StatusUnknown
	}

	get := action.NewGet(actionConfig)
	chart, err := get.Run(chartname)
	if err != nil && err.Error() != "release: not found" {
		HelmLogger.Errorf("HelmStatus List Error: %s", err.Error())
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

type IndexFile struct {
	APIVersion string                 `yaml:"apiVersion"`
	Entries    map[string]interface{} `yaml:"entries"`
}

func parseHelmEntry(entry *repo.Entry) *HelmEntryWithoutPassword {
	return &HelmEntryWithoutPassword{
		Name:                  entry.Name,
		URL:                   entry.URL,
		InsecureSkipTLSverify: entry.InsecureSkipTLSverify,
		PassCredentialsAll:    entry.PassCredentialsAll,
	}
}

func InitHelmConfig() error {
	// Set the registryConfig, repositoryConfig and repositoryCache variables
	registryConfig = fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.HelmDataPath, HELM_REGISTRY_CONFIG_FILE)
	repositoryConfig = fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.HelmDataPath, HELM_REPOSITORY_CONFIG_FILE)
	repositoryCache = fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.HelmDataPath, HELM_REPOSITORY_CACHE_FOLDER)

	// Set the HELM_HOME environment variable
	folder := fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.HelmDataPath, HELM_DATA_HOME)

	// create helm home directory if it does not exist
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		err := os.MkdirAll(folder, 0755)
		if err != nil {
			HelmLogger.Errorf("Error creating directory: %s", err.Error())
			return err
		}
		HelmLogger.Infof("Helm home directory created successfully: %s", folder)

		// create cache directory if it does not exist
		if _, err := os.Stat(repositoryCache); os.IsNotExist(err) {
			err := os.MkdirAll(repositoryCache, 0755)
			if err != nil {
				HelmLogger.Errorf("Error creating directory: %s", err.Error())
				return err
			}
			HelmLogger.Infof("Helm cache directory created successfully: %s", repositoryCache)
		}

		// create plugins directory if it does not exist
		pluginsFolder := fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.HelmDataPath, HELM_PLUGINS)
		if _, err := os.Stat(pluginsFolder); os.IsNotExist(err) {
			err := os.MkdirAll(pluginsFolder, 0755)
			if err != nil {
				HelmLogger.Errorf("Error creating directory: %s", err.Error())
				return err
			}
			HelmLogger.Infof("Helm plugins directory created successfully: %s", pluginsFolder)
		}
	}
	os.Setenv("HELM_CACHE_HOME", fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.HelmDataPath, HELM_CACHE_HOME))
	os.Setenv("HELM_CONFIG_HOME", fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.HelmDataPath, HELM_CONFIG_HOME))
	os.Setenv("HELM_DATA_HOME", fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.HelmDataPath, HELM_DATA_HOME))
	os.Setenv("HELM_PLUGINS", fmt.Sprintf("%s/%s", utils.CONFIG.Kubernetes.HelmDataPath, HELM_PLUGINS))
	os.Setenv("HELM_REGISTRY_CONFIG", registryConfig)
	os.Setenv("HELM_REPOSITORY_CACHE", repositoryCache)
	os.Setenv("HELM_REPOSITORY_CONFIG", repositoryConfig)
	os.Setenv("HELM_LOG_LEVEL", "trace")

	K8sLogger.Infof("HELM_CACHE_HOME: %s", os.Getenv("HELM_CACHE_HOME"))
	K8sLogger.Infof("HELM_CONFIG_HOME: %s", os.Getenv("HELM_CONFIG_HOME"))
	K8sLogger.Infof("HELM_DATA_HOME: %s", os.Getenv("HELM_DATA_HOME"))
	K8sLogger.Infof("HELM_PLUGINS: %s", os.Getenv("HELM_PLUGINS"))
	K8sLogger.Infof("HELM_REGISTRY_CONFIG: %s", os.Getenv("HELM_REGISTRY_CONFIG"))
	K8sLogger.Infof("HELM_REPOSITORY_CACHE: %s", os.Getenv("HELM_REPOSITORY_CACHE"))
	K8sLogger.Infof("HELM_REPOSITORY_CONFIG: %s", os.Getenv("HELM_REPOSITORY_CONFIG"))

	if _, err := os.Stat(repositoryConfig); os.IsNotExist(err) {
		destFile, err := os.Create(repositoryConfig)
		if err != nil {
			HelmLogger.Errorf("Error creating repository config: %s", err.Error())
		}
		defer destFile.Close()

		// add default repository
		data := HelmRepoAddRequest{
			Name: "mogenius",
			Url:  "https://helm.mogenius.com/public",
		}
		if _, err := HelmRepoAdd(data); err != nil {
			HelmLogger.Errorf("Failed to add default repository: %s", err.Error())
		}
	}

	if _, err := HelmRepoUpdate(); err != nil {
		HelmLogger.Errorf("Failed to update repositories: %s", err.Error())
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

func NewCliConfArgs() []string {
	return []string{
		"--registry-config",
		registryConfig,
		"--repository-config",
		repositoryConfig,
		"--repository-cache",
		repositoryCache,
	}
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

	// Initialize the file where repositories are stored
	file := settings.RepositoryConfig

	// Load the existing repositories
	repoFile, err := repo.LoadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to load repository file: %s", err)
	}

	// Check if the repository already exists
	if repoFile.Has(data.Name) {
		return "", fmt.Errorf("repository name (%s) already exists", data.Name)
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

	return fmt.Sprintf("repository '%s' removed", data.Name), nil
}

func HelmChartSearch(data HelmChartSearchRequest) ([]HelmChartInfo, error) {
	settings := cli.New()

	repositoriesFile, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil {
		HelmLogger.Fatalf("Failed to load repositories file: %v", err)
	}

	var allCharts []HelmChartInfo

	for _, repoEntry := range repositoriesFile.Repositories {
		cacheIndexFile := filepath.Join(settings.RepositoryCache, fmt.Sprintf("%s-index.yaml", repoEntry.Name))

		indexFile, err := repo.LoadIndexFile(cacheIndexFile)
		if err != nil {
			HelmLogger.Printf("Error loading index file for repo %s: %v", repoEntry.Name, err)
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
	if err := actionConfig.Init(settings.RESTClientGetter(), "", os.Getenv("HELM_DRIVER"), HelmLogger.Infof); err != nil {
		HelmLogger.Errorf("HelmChartShow Init Error: %s", err.Error())
		return "", err
	}

	// Fetch the chart
	chartPathOptions := action.ChartPathOptions{}
	chartPath, err := chartPathOptions.LocateChart(data.Chart, settings)
	if err != nil {
		HelmLogger.Errorf("HelmShow LocateChart Error: %s\n", err.Error())
		return "", err
	}

	// Show the chart
	show := action.NewShowWithConfig(data.ShowFormat, actionConfig)
	result, err := show.Run(chartPath)
	if err != nil {
		HelmLogger.Errorf("HelmShow Run Error: %s", err.Error())
		return "", err
	}

	return result, nil
}

func HelmChartVersion(data HelmChartVersionRequest) ([]HelmChartInfo, error) {
	settings := cli.New()

	repositoriesFile, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil {
		HelmLogger.Fatalf("Failed to load repositories file: %v", err)
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
			HelmLogger.Printf("Error loading index file for repo %s: %v", repoEntry.Name, err)
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

func HelmChartInstall(data HelmChartInstallRequest) (string, error) {
	settings := NewCli()
	settings.SetNamespace(data.Namespace)
	settings.Debug = true

	var HelmInstallLogger = HelmLogger.WithFields(log.Fields{
		"component":   structs.ComponentHelm,
		"releaseName": data.Release,
		"namespace":   data.Namespace,
	})

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, os.Getenv("HELM_DRIVER"), HelmInstallLogger.Infof); err != nil {
		HelmInstallLogger.Errorf("HelmInstall Init Error: %s\n", err.Error())
		return "", err
	}

	if registry.IsOCI(data.Chart) {
		return "", fmt.Errorf("OCI charts are not supported")
	}

	install := action.NewInstall(actionConfig)
	install.DryRun = data.DryRun
	install.ReleaseName = data.Release
	install.Namespace = data.Namespace
	install.Version = data.Version
	install.Wait = false
	install.Timeout = 300 * time.Second

	chartPath, err := install.LocateChart(data.Chart, settings)
	if err != nil {
		HelmInstallLogger.Errorf("HelmInstall LocateChart Error: %s\n", err.Error())
		return "", err
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		HelmInstallLogger.Errorf("HelmInstall Load Error: %s\n", err.Error())
		return "", err
	}

	// Parse the values string into a map
	valuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(data.Values), &valuesMap); err != nil {
		HelmInstallLogger.Errorf("HelmInstall Values Unmarshal Error: %s\n", err.Error())
		return "", err
	}

	re, err := install.Run(chartRequested, valuesMap)
	if err != nil {
		HelmInstallLogger.Errorf("HelmInstall Run Error: %s\n", err.Error())
		return "", err
	}
	if re == nil {
		return "", fmt.Errorf("HelmInstall Error: Release not found")
	}

	HelmInstallLogger.Info(installStatus(*re))

	return installStatus(*re), nil
}

func HelmReleaseUpgrade(data HelmReleaseUpgradeRequest) (string, error) {
	var HelmReleaseUpgradeLogger = HelmLogger.WithFields(log.Fields{
		"component":   structs.ComponentHelm,
		"releaseName": data.Release,
		"namespace":   data.Namespace,
	})

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, os.Getenv("HELM_DRIVER"), HelmReleaseUpgradeLogger.Infof); err != nil {
		HelmReleaseUpgradeLogger.Errorf("HelmUpgrade Init Error: %s\n", err.Error())
		return "", err
	}

	upgrade := action.NewUpgrade(actionConfig)
	upgrade.DryRun = data.DryRun
	upgrade.Wait = false
	upgrade.Namespace = data.Namespace
	upgrade.Version = data.Version
	upgrade.Timeout = 300 * time.Second

	chartPath, err := upgrade.LocateChart(data.Chart, settings)
	if err != nil {
		HelmReleaseUpgradeLogger.Errorf("HelmUpgrade LocateChart Error: %s\n", err.Error())
		return "", err
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		HelmReleaseUpgradeLogger.Errorf("HelmUpgrade Load Error: %s\n", err.Error())
		return "", err
	}

	// Parse the values string into a map
	valuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(data.Values), &valuesMap); err != nil {
		HelmReleaseUpgradeLogger.Errorf("HelmUpgrade Values Unmarshal Error: %s\n", err.Error())
		return "", err
	}

	re, err := upgrade.Run(data.Release, chartRequested, valuesMap)
	if err != nil {
		HelmReleaseUpgradeLogger.Errorf("HelmUpgrade Run Error: %s\n", err.Error())
		return "", err
	}
	if re == nil {
		return "", fmt.Errorf("HelmUpgrade Error: Release not found")
	}

	return installStatus(*re), nil
}

func HelmReleaseUninstall(data HelmReleaseUninstallRequest) (string, error) {
	var HelmReleaseUninstallLogger = HelmLogger.WithFields(log.Fields{
		"component":   structs.ComponentHelm,
		"releaseName": data.Release,
		"namespace":   data.Namespace,
	})

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, os.Getenv("HELM_DRIVER"), HelmReleaseUninstallLogger.Infof); err != nil {
		HelmReleaseUninstallLogger.Errorf("HelmUninstall Init Error: %s\n", err.Error())
		return "", err
	}

	uninstall := action.NewUninstall(actionConfig)
	uninstall.DryRun = data.DryRun
	uninstall.Wait = false
	_, err := uninstall.Run(data.Release)
	if err != nil {
		HelmReleaseUninstallLogger.Errorf("HelmUninstall Run Error: %s\n", err.Error())
		return "", err
	}

	return fmt.Sprintf("Release '%s' uninstalled", data.Release), nil
}

func installStatus(rel release.Release) string {
	result := ""
	result += fmt.Sprintf("%s (%s)\n", rel.Name, rel.Info.Status)
	result += fmt.Sprintf("%s\n", rel.Info.Description)
	result += fmt.Sprintf("🗒️ Notes:\n%s\n", rel.Info.Notes)
	return result
}

func HelmReleaseList(data HelmReleaseListRequest) ([]*release.Release, error) {
	var HelmReleaseListLogger = HelmLogger.WithFields(log.Fields{
		"component": structs.ComponentHelm,
		"namespace": data.Namespace,
	})

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, os.Getenv("HELM_DRIVER"), HelmReleaseListLogger.Infof); err != nil {
		HelmReleaseListLogger.Errorf("HelmReleaseList Init Error: %s", err.Error())
		return []*release.Release{}, err
	}

	list := action.NewList(actionConfig)
	list.StateMask = action.ListAll
	releases, err := list.Run()
	// remove unnecessary fields
	for _, release := range releases {
		release.Chart.Files = nil
		release.Chart.Templates = nil
		release.Chart.Values = nil
		release.Manifest = ""
	}
	if err != nil {
		HelmReleaseListLogger.Errorf("HelmReleaseList List Error: %s", err.Error())
		return releases, err
	}
	return releases, nil
}

func HelmReleaseStatus(data HelmReleaseStatusRequest) (*HelmReleaseStatusInfo, error) {
	var HelmReleaseStatusLogger = HelmLogger.WithFields(log.Fields{
		"component":   structs.ComponentHelm,
		"releaseName": data.Release,
		"namespace":   data.Namespace,
	})

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, os.Getenv("HELM_DRIVER"), HelmReleaseStatusLogger.Infof); err != nil {
		HelmReleaseStatusLogger.Errorf("HelmReleaseStatus Init Error: %s", err.Error())
		return nil, err
	}

	status := action.NewStatus(actionConfig)
	re, err := status.Run(data.Release)
	if err != nil {
		HelmReleaseStatusLogger.Errorf("HelmReleaseStatus List Error: %s", err.Error())
		return nil, err
	}
	if re == nil {
		return nil, fmt.Errorf("HelmReleaseStatus Error: Release not found")
	}

	helmReleaseStatusInfo := HelmReleaseStatusInfo{
		Name:         re.Name,
		LastDeployed: re.Info.LastDeployed.Time,
		Namespace:    re.Namespace,
		Status:       re.Info.Status.String(),
		Version:      re.Version,
		Chart:        fmt.Sprintf("%s-%s", re.Chart.Metadata.Name, re.Chart.Metadata.Version),
	}

	return &helmReleaseStatusInfo, nil
}

func HelmReleaseHistory(data HelmReleaseHistoryRequest) ([]*release.Release, error) {
	var HelmReleaseHistoryLogger = HelmLogger.WithFields(log.Fields{
		"component":   structs.ComponentHelm,
		"releaseName": data.Release,
		"namespace":   data.Namespace,
	})

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, os.Getenv("HELM_DRIVER"), HelmReleaseHistoryLogger.Infof); err != nil {
		HelmReleaseHistoryLogger.Errorf("HelmHistory Init Error: %s", err.Error())
		return []*release.Release{}, err
	}

	history := action.NewHistory(actionConfig)
	history.Max = 10
	releases, err := history.Run(data.Release)
	if err != nil {
		HelmReleaseHistoryLogger.Errorf("HelmHistory List Error: %s", err.Error())
		return releases, err
	}
	return releases, nil
}

func HelmReleaseRollback(data HelmReleaseRollbackRequest) (string, error) {
	var HelmReleaseRollbackLogger = HelmLogger.WithFields(log.Fields{
		"component":   structs.ComponentHelm,
		"releaseName": data.Release,
		"namespace":   data.Namespace,
	})

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, os.Getenv("HELM_DRIVER"), HelmReleaseRollbackLogger.Infof); err != nil {
		HelmReleaseRollbackLogger.Errorf("HelmRollback Init Error: %s", err.Error())
		return "", err
	}

	rollback := action.NewRollback(actionConfig)
	rollback.Version = data.Revision
	err := rollback.Run(data.Release)
	if err != nil {
		HelmReleaseRollbackLogger.Errorf("HelmRollback Run Error: %s", err.Error())
		return "", err
	}

	return fmt.Sprintf("Rolled back '%s/%s' to revision %d", data.Namespace, data.Release, data.Revision), nil
}

func HelmReleaseGet(data HelmReleaseGetRequest) (string, error) {
	var HelmReleaseGetLogger = HelmLogger.WithFields(log.Fields{
		"component":   structs.ComponentHelm,
		"releaseName": data.Release,
		"namespace":   data.Namespace,
	})

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, os.Getenv("HELM_DRIVER"), HelmReleaseGetLogger.Infof); err != nil {
		HelmReleaseGetLogger.Errorf("HelmGet Init Error: %s", err.Error())
		return "", err
	}

	get := action.NewGet(actionConfig)
	re, err := get.Run(data.Release)
	if err != nil && err.Error() != "release: not found" {
		HelmReleaseGetLogger.Errorf("HelmGet List Error: %s", err.Error())
		return "", err
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
		HelmLogger.Errorf("Error while marshaling. %v", err)
		return ""
	}

	return string(yamlData)
}
