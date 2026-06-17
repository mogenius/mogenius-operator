package helm

import (
	"context"
	"encoding/json"
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
	"sort"
	"strconv"
	"strings"
	"time"

	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/metadata"

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
	"helm.sh/helm/v4/pkg/storage/driver"
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

// helmReleaseListCache caches the (expensive) full release listing so paging
// through a large release set does not re-list and re-decode every helm secret
// on every page request. Freshness is driven by invalidation, not by the TTL:
// it is flushed whenever this operator mutates releases AND whenever the Helm
// release-secret watcher observes ANY change (including out-of-band ones via
// helm CLI / other controllers). The long TTL is only a failsafe in case that
// watcher is not running.
const helmReleaseListCacheTTL = 30 * time.Minute

var helmReleaseListCache = cache.New(helmReleaseListCacheTTL, 2*helmReleaseListCacheTTL)

func invalidateReleaseListCache() {
	helmReleaseListCache.Flush()
}

// InvalidateReleaseListCache drops the cached release listing. Wire this to the
// Helm release-secret watcher so the cache is invalidated on every release
// change, regardless of whether it originated from this operator.
func InvalidateReleaseListCache() {
	invalidateReleaseListCache()
}

func Setup(logManager logging.SlogManager, configModule cfg.ConfigModule, valkey valkeyclient.ValkeyClient) {
	helmLogger = logManager.CreateLogger("helm")
	config = configModule
	valkeyClient = valkey

	// Pin Helm's server-side-apply field manager to a stable name. Without this
	// Helm v4 derives the manager from filepath.Base(os.Args[0]), which drifts
	// with how the binary is invoked (in-cluster "mogenius-operator", local
	// "dist/native/mogenius-operator", test binaries). A drifting manager name
	// leaves orphaned field ownership that later SSA applies conflict with
	// (MOG-4393).
	kube.ManagedFieldsManager = "mogenius-operator"
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

func InitBasicRepos(repos []HelmRepoAddRequest) error {
	for _, repo := range repos {
		data := HelmRepoAddRequest{
			Name: repo.Name,
			Url:  repo.Url,
		}
		if _, err := HelmRepoAdd(data); err != nil {
			if err != ErrorRepoAlreadyExists {
				helmLogger.Error("failed to add default helm repository", "repoName", data.Name, "repoUrl", data.Url, "error", err.Error())
				return err
			}
		}
	}

	if _, err := HelmRepoUpdate(); err != nil {
		helmLogger.Error("failed to update helm repositories", "error", err.Error())
		return err
	}

	return nil
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

	// Argo, when non-nil, marks this entry as an Argo-CD-managed helm chart
	// rather than a real helm release. The list view renders these alongside
	// real releases so users can still upgrade/uninstall Argo-managed charts
	// (MOG-4394). MarshalJSON emits the shape the frontend's
	// ClusterHelmReleaseDto expects for type "git-ops-argo-cd-application".
	Argo *ArgoReleaseInfo `json:"-"`
}

// ArgoReleaseInfo carries the data needed to render an Argo CD Application as a
// pseudo helm release in the release list. The platform API used to merge these
// in; pagination now happens in the operator (MOG-4367), so the operator owns
// the merge (MOG-4394).
type ArgoReleaseInfo struct {
	// Application is the full Argo Application object (unstructured). The
	// frontend reads it for the status column, the detail drawer and OCI
	// detection (spec.source.chart).
	Application map[string]any
	// ParentName / ParentNamespace identify the Application to delete (the
	// frontend's uninstall reads data.parentApplication.resourceName).
	ParentName      string
	ParentNamespace string
	// ValuesObject is the helm valuesObject rendered as YAML (the upgrade form
	// pre-fills from data.valuesObject).
	ValuesObject string
	ChartName    string
	// Version is spec.source.targetRevision (chart version, a string — note the
	// real-release HelmRelease.Version is an int revision, hence the dedicated
	// field here).
	Version string
	// AppVersion is best-effort; the operator does not fetch chart metadata over
	// the network on every list call, so it is usually empty.
	AppVersion string
	// DestNamespace is spec.destination.namespace (shown as the release namespace).
	DestNamespace string
	RepoName      string
	// CreatedAt is metadata.creationTimestamp, used for lastDeployed sorting.
	CreatedAt time.Time
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
	// OCIChartUrl expects a full OCI chart reference, e.g., "oci://registry-1.docker.io/myrepo/mychart"
	OCIChartUrl string `json:"ociChartUrl" validate:"required"`
	Namespace   string `json:"namespace" validate:"required"`
	Release     string `json:"release" validate:"required"`
	// Optional fields
	Version string `json:"version,omitempty"`
	Values  string `json:"values,omitempty"`
	DryRun  bool   `json:"dryRun,omitempty"`
	// OCI specific fields
	AuthHost string `json:"authHost,omitempty"` // e.g., "registry-1.docker.io"
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
	// Name, when set, filters the result to releases whose chart name
	// (chart.metadata.name) matches exactly. Empty means no filter.
	Name string `json:"name,omitempty"`
}

type HelmReleaseListPaginatedRequest struct {
	Namespace string `json:"namespace"`
	Filter    string `json:"filter,omitempty"` // case-insensitive substring match on release name
	Offset    int    `json:"offset"`
	Limit     int    `json:"limit"`     // 0 means "no limit"
	SortBy    string `json:"sortBy"`    // "lastDeployed" (default) | "name"
	SortOrder string `json:"sortOrder"` // "asc" | "desc"
	// WorkspaceName, when set, scopes the result to the helm releases that are
	// registered as resources of that workspace (resolved server-side from the
	// Workspace CRD). Empty means cluster-wide (all releases).
	WorkspaceName string `json:"workspaceName,omitempty"`
}

// HelmWorkspaceScope restricts a paginated listing to a workspace's helm
// releases. A non-nil scope with an empty Allowed set yields no releases (a
// workspace that has no helm resources). nil means cluster-wide (no filter).
type HelmWorkspaceScope struct {
	Allowed map[string]struct{} // keys built via WorkspaceHelmKey
}

// WorkspaceHelmKey builds the lookup key for the workspace allow-set. The NUL
// separator avoids collisions between namespace and release-name boundaries.
func WorkspaceHelmKey(namespace, releaseName string) string {
	return namespace + "\x00" + releaseName
}

type HelmReleaseListPaginatedResponse struct {
	Items      []*HelmRelease `json:"items"`
	TotalCount int            `json:"totalCount"`
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
		InsecureSkipTLSverify: entry.InsecureSkipTLSVerify,
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
			return fmt.Errorf("failed to create repository config: %w", err)
		}
		destFile.Close()
	}

	_ = restoreRepositoryFileFromValkey()

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

// normalizeRepoURL canonicalizes a Helm repository URL for equality checks:
// surrounding whitespace and trailing slashes are removed so that e.g.
// "https://charts.example.com/" and "https://charts.example.com" compare equal.
func normalizeRepoURL(url string) string {
	return strings.TrimRight(strings.TrimSpace(url), "/")
}

func HelmRepoAdd(data HelmRepoAddRequest) (string, error) {
	settings := NewCli()

	// Create a new Helm repository entry
	entry := &repo.Entry{
		Name:                  data.Name,
		URL:                   data.Url,
		Username:              data.Username,
		Password:              data.Password,
		InsecureSkipTLSVerify: data.InsecureSkipTLSverify,
		PassCredentialsAll:    data.PassCredentialsAll,
	}

	// Load the existing repositories
	repoFile, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to load repository file: %s", err)
	}

	// Check if the repository already exists
	if repoFile.Has(data.Name) {
		return fmt.Sprintf("repository '%s' already exists", data.Name), nil
	}

	// Detect the same repository URL already registered under a different
	// name. Helm resolves charts by repo name ("reponame/chart"), so silently
	// adding a duplicate under the requested name would still leave the install
	// referencing a name that does not match the existing repo - producing a
	// cryptic "chart not found" later. Surface an actionable error instead.
	// (MOG-4306)
	for _, existing := range repoFile.Repositories {
		if existing.Name != data.Name && normalizeRepoURL(existing.URL) == normalizeRepoURL(data.Url) {
			return "", fmt.Errorf("repository URL '%s' is already registered under the name '%s'; reference the chart as '%s/<chart>' or remove the existing repository", data.Url, existing.Name, existing.Name)
		}
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
			re.InsecureSkipTLSVerify = data.InsecureSkipTLSverify
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
	result := make([]HelmChartInfo, 0, len(charts))
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
	defer invalidateReleaseListCache() // the release set changed
	// Start each attempt with a clean log: drop entries from previous
	// install/upgrade attempts of the same release so the user only sees the
	// current run. Logs of a failed attempt stay visible until the next
	// attempt replaces them (previously they were wiped on error, which hid
	// the failure reason). (MOG-4306 follow-up)
	cleanReleaseLogs(data.Namespace, data.Release)

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

	if !registry.IsOCI(data.OCIChartUrl) {
		return "", fmt.Errorf("non-OCI charts are not supported in OCI installation")
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
	// See HelmReleaseUpgrade: take sole ownership on SSA conflicts (MOG-4393).
	install.ForceConflicts = true
	install.Labels = map[string]string{
		"mogenius.com/installed-via": "mogenius-operator",
		"mogenius.com/oci-chart":     "true",
	}

	// Create registry client for OCI
	registryClient, err := newRegistryClient(settings, false)
	if err != nil {
		return "", fmt.Errorf("failed to create OCI registry client: %w", err)
	}
	if (data.Username != "" || data.Password != "") && data.AuthHost != "" {
		err = registryClient.Login(
			data.AuthHost,
			registry.LoginOptBasicAuth(data.Username, data.Password),
		)
		if err != nil {
			return "", fmt.Errorf("failed to login to OCI registry: %w", err)
		}
	} else {
		helmLogger.Info("No OCI registry credentials provided, attempting anonymous access", "releaseName", data.Release, "namespace", data.Namespace)
	}
	install.SetRegistryClient(registryClient)

	chartPath, err := install.ChartPathOptions.LocateChart(data.OCIChartUrl, settings)
	if err != nil {
		return "", fmt.Errorf("failed to locate chart: %w", err)
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

	rel, err := install.Run(chartRequested, valuesMap)
	if err != nil {
		helmLogger.Error("HelmOCIInstall Run",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		logReleaseFailureDiagnostics(settings, data.Namespace, data.Release)
		return "", err
	}

	re, ok := rel.(*release.Release)
	if !ok {
		return "", errors.New("HelmOCIInstall Error: Release type assertion failed")
	}

	err = SaveRepoNameToValkey(data.Namespace, data.Release, data.OCIChartUrl)
	if err != nil {
		helmLogger.Error("failed to SaveRepoNameToValkey",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		return "", err
	}

	helmLogger.Info(installStatus(*re), "releaseName", data.Release, "namespace", data.Namespace)
	return installStatus(*re), nil
}

func newRegistryClient(settings *cli.EnvSettings, plainHTTP bool) (*registry.Client, error) {
	opts := []registry.ClientOption{
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(os.Stderr),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	}

	if plainHTTP {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}

	registryClient, err := registry.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	return registryClient, nil
}

func HelmChartInstall(data HelmChartInstallUpgradeRequest) (result string, err error) {
	defer invalidateReleaseListCache() // the release set changed
	// Start each attempt with a clean log so the user only sees the current
	// run, not entries from previous install attempts of the same release.
	// See HelmOciInstall for the rationale on no longer wiping on error.
	cleanReleaseLogs(data.Namespace, data.Release)

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
		return "", fmt.Errorf("OCI charts are not supported in the standard installation, please use the OCI installation endpoint")
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
	// See HelmReleaseUpgrade: take sole ownership on SSA conflicts (MOG-4393).
	install.ForceConflicts = true
	install.Labels = map[string]string{
		"mogenius.com/installed-via": "mogenius-operator",
		"mogenius.com/oci-chart":     "false",
	}

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

	// Ownership preflight: detect orphaned cluster-scoped resources left over
	// from a previous incomplete uninstall. Adopt them via TakeOwnership when
	// safe, abort when a foreign Helm release already owns them.
	if !data.DryRun {
		needsTakeOwnership, perr := CheckOwnershipAndLog(
			actionConfig, chartRequested, valuesMap,
			data.Release, data.Namespace, data.Version,
		)
		if perr != nil {
			helmLogger.Error("HelmInstall ownership preflight failed",
				"releaseName", data.Release,
				"namespace", data.Namespace,
				"error", perr.Error(),
			)
			return "", perr
		}
		install.TakeOwnership = needsTakeOwnership
	}

	helmLogger.Info("Installing chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	re, err := install.Run(chartRequested, valuesMap)
	if err != nil {
		helmLogger.Error("HelmInstall Run",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		logReleaseFailureDiagnostics(settings, data.Namespace, data.Release)
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
	defer invalidateReleaseListCache() // the release set changed
	// Start each attempt with a clean log so the user only sees the current
	// run, not entries from previous upgrade attempts of the same release.
	// See HelmOciInstall for the rationale on no longer wiping on error.
	cleanReleaseLogs(data.Namespace, data.Release)

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
	// Force server-side-apply conflicts so the operator becomes sole manager of
	// the platform charts it installs. Resolves the "Apply failed with conflict"
	// error when a field (e.g. argo-cd-rbac-cm .data.policy.csv) is still held by
	// a stale field-manager entry (MOG-4393).
	upgrade.ForceConflicts = true
	// ServerSideApply must be "true" (not "auto") because "auto" inherits the
	// previous release's ApplyMethod — releases originally installed via CSA would
	// resolve to SSA=false, making ForceConflicts invalid.
	upgrade.ServerSideApply = "true"

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

	// Ownership preflight: detect orphaned cluster-scoped resources left over
	// from a previous incomplete uninstall. Adopt them via TakeOwnership when
	// safe, abort when a foreign Helm release already owns them.
	if !data.DryRun {
		needsTakeOwnership, perr := CheckOwnershipAndLog(
			actionConfig, chartRequested, valuesMap,
			data.Release, data.Namespace, data.Version,
		)
		if perr != nil {
			helmLogger.Error("HelmUpgrade ownership preflight failed",
				"releaseName", data.Release,
				"namespace", data.Namespace,
				"error", perr.Error(),
			)
			return "", perr
		}
		upgrade.TakeOwnership = needsTakeOwnership
	}

	helmLogger.Info("Upgrading chart ...", "releaseName", data.Release, "namespace", data.Namespace)
	re, err := upgrade.Run(data.Release, chartRequested, valuesMap)
	if err != nil {
		helmLogger.Error("HelmUpgrade Run failed",
			"releaseName", data.Release,
			"namespace", data.Namespace,
			"error", err.Error(),
		)
		logReleaseFailureDiagnostics(settings, data.Namespace, data.Release)
		return "", err
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
	defer invalidateReleaseListCache() // the release set changed
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

	releases, err := listCurrentReleases(actionConfig)
	if err != nil {
		helmLogger.Error("HelmReleaseList List", "namespace", data.Namespace, "error", err.Error())
		return []*HelmRelease{}, err
	}

	result := []*HelmRelease{}
	// remove unnecessary fields
	for _, re := range releases {
		// optional filter by chart name (chart.metadata.name)
		if data.Name != "" {
			if re.Chart == nil || re.Chart.Metadata == nil || re.Chart.Metadata.Name != data.Name {
				continue
			}
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
	return result, nil
}

// paginateHelmReleases applies the optional workspace scope, name filter,
// stable sorting and offset/limit slicing to a unified list of entries (real
// releases and Argo-managed charts alike). It is pure (no I/O) so it can be
// unit-tested in isolation. It returns the page slice and the total count of
// entries matching the scope+filter (before slicing).
func paginateHelmReleases(items []*HelmRelease, data HelmReleaseListPaginatedRequest, scope *HelmWorkspaceScope) ([]*HelmRelease, int) {
	// Work on a copy of the slice header so the in-place sort below never
	// mutates a shared/cached slice or races with concurrent requests. The
	// HelmRelease pointers themselves are only read.
	cp := make([]*HelmRelease, len(items))
	copy(cp, items)
	items = cp

	if scope != nil {
		scoped := items[:0:0]
		for _, hr := range items {
			if _, ok := scope.Allowed[helmReleaseScopeKey(hr)]; ok {
				scoped = append(scoped, hr)
			}
		}
		items = scoped
	}

	if data.Filter != "" {
		filter := strings.ToLower(data.Filter)
		filtered := items[:0:0]
		for _, hr := range items {
			if strings.Contains(strings.ToLower(hr.Name), filter) {
				filtered = append(filtered, hr)
			}
		}
		items = filtered
	}

	switch data.SortBy {
	case "name":
		desc := data.SortOrder == "desc"
		sort.SliceStable(items, func(i, j int) bool {
			if desc {
				return items[i].Name > items[j].Name
			}
			return items[i].Name < items[j].Name
		})
	default: // "lastDeployed" (default), newest first unless asc is requested
		asc := data.SortOrder == "asc"
		sort.SliceStable(items, func(i, j int) bool {
			ti, tj := helmReleaseLastDeployed(items[i]), helmReleaseLastDeployed(items[j])
			if ti.Equal(tj) {
				return items[i].Name < items[j].Name // stable tiebreaker
			}
			if asc {
				return ti.Before(tj)
			}
			return ti.After(tj)
		})
	}

	total := len(items)
	if data.Limit > 0 {
		start := data.Offset
		if start < 0 {
			start = 0
		}
		if start > total {
			start = total
		}
		end := start + data.Limit
		if end > total {
			end = total
		}
		items = items[start:end]
	}

	return items, total
}

// helmReleaseScopeKey is the workspace allow-set key for an entry. Argo entries
// key on the Argo install namespace + release name (matching the workspace
// "argocd" resource), real releases on their install namespace + name.
func helmReleaseScopeKey(hr *HelmRelease) string {
	if hr.Argo != nil {
		return WorkspaceHelmKey(hr.Argo.ParentNamespace, hr.Name)
	}
	return WorkspaceHelmKey(hr.Namespace, hr.Name)
}

// helmReleaseLastDeployed returns the timestamp used for lastDeployed sorting:
// the Argo Application creation time for Argo entries, the release's last
// deployed time otherwise. Missing values sort as the zero time.
func helmReleaseLastDeployed(hr *HelmRelease) time.Time {
	if hr.Argo != nil {
		return hr.Argo.CreatedAt
	}
	if hr.Info == nil {
		return time.Time{}
	}
	return hr.Info.LastDeployed
}

// HelmReleaseListPaginated lists helm releases server-side filtered, sorted and
// sliced. argoItems are Argo-CD-managed charts (resolved by the caller, since
// they live outside helm) that are merged into the same sorted, paginated and
// scoped result so users can still upgrade them (MOG-4394). When scope is
// non-nil the result is restricted to that workspace's resources (resolved by
// the caller from the Workspace CRD); nil scope means cluster-wide.
func HelmReleaseListPaginated(data HelmReleaseListPaginatedRequest, scope *HelmWorkspaceScope, argoItems []*HelmRelease) (HelmReleaseListPaginatedResponse, error) {
	empty := HelmReleaseListPaginatedResponse{Items: []*HelmRelease{}, TotalCount: 0}

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmReleaseListPaginated Init", "namespace", data.Namespace, "error", err.Error())
		return empty, err
	}

	secretsDriver, ok := actionConfig.Releases.Driver.(*driver.Secrets)
	if !ok {
		// Non-secret storage driver: fall back to decoding everything.
		return helmReleaseListPaginatedFull(actionConfig, data, scope, argoItems)
	}

	// Phase 1: cheap metadata-only stub index (no gzip blob), merged with the
	// already-materialised Argo entries, then scoped/filtered/sorted/sliced.
	stubs, err := listReleaseStubsCached(data.Namespace)
	if err != nil {
		return empty, err
	}
	items := make([]*HelmRelease, 0, len(stubs)+len(argoItems))
	items = append(items, stubs...)
	items = append(items, argoItems...)

	page, total := paginateHelmReleases(items, data, scope)

	// Phase 2: decode the full release only for the real releases on this page.
	// Replace the stub pointer with a freshly materialised HelmRelease so the
	// cached stub objects are never mutated.
	for i, hr := range page {
		if hr.Argo != nil {
			continue // Argo entries are already complete and carry their repoName
		}
		re, err := decodePageRelease(secretsDriver, hr.Namespace, hr.Name, hr.Version)
		if err != nil {
			// Secret vanished between indexing and decoding (race): keep the
			// stub so the page still shows name/namespace/status.
			helmLogger.Debug("HelmReleaseListPaginated decode page release",
				"release", hr.Name, "namespace", hr.Namespace, "error", err.Error())
			continue
		}
		trimReleaseForList(re)

		repoName := ""
		if rep, _ := GetRepoNameFromValkey(re.Name, re.Namespace); rep != nil {
			chartName := ""
			if re.Chart != nil && re.Chart.Metadata != nil {
				chartName = re.Chart.Metadata.Name
			}
			repoName = strings.Replace(rep.RepoName, "/"+chartName, "", 1)
		}
		page[i] = helmReleaseFromRelease(re, repoName)
	}

	return HelmReleaseListPaginatedResponse{Items: page, TotalCount: total}, nil
}

// helmReleaseListPaginatedFull is the fallback used when the storage backend is
// not the secret driver (e.g. configmaps/memory in tests). It decodes every
// current release up front - the pre-metadata-index behaviour - then sorts,
// scopes and slices the same way as the fast path.
func helmReleaseListPaginatedFull(actionConfig *action.Configuration, data HelmReleaseListPaginatedRequest, scope *HelmWorkspaceScope, argoItems []*HelmRelease) (HelmReleaseListPaginatedResponse, error) {
	empty := HelmReleaseListPaginatedResponse{Items: []*HelmRelease{}, TotalCount: 0}

	releases, err := listCurrentReleases(actionConfig)
	if err != nil {
		helmLogger.Error("HelmReleaseListPaginated list", "namespace", data.Namespace, "error", err.Error())
		return empty, err
	}

	items := make([]*HelmRelease, 0, len(releases)+len(argoItems))
	for _, re := range releases {
		trimReleaseForList(re)
		items = append(items, helmReleaseFromRelease(re, ""))
	}
	items = append(items, argoItems...)

	page, total := paginateHelmReleases(items, data, scope)

	for _, hr := range page {
		if hr.Argo != nil {
			continue
		}
		if rep, _ := GetRepoNameFromValkey(hr.Name, hr.Namespace); rep != nil {
			chartName := ""
			if hr.Chart != nil && hr.Chart.Metadata != nil {
				chartName = hr.Chart.Metadata.Name
			}
			hr.RepoName = strings.Replace(rep.RepoName, "/"+chartName, "", 1)
		}
	}

	return HelmReleaseListPaginatedResponse{Items: page, TotalCount: total}, nil
}

// nonSupersededStatuses is every release status except "superseded". Helm
// relabels a revision to "superseded" the moment a newer revision takes over,
// so for any release exactly one revision carries a non-superseded status: the
// current one. Querying these statuses server-side returns the current revision
// of every release without fetching (and decoding) the historical superseded
// revisions, which dominate the cost at thousands of releases.
var nonSupersededStatuses = []string{
	string(releasecommon.StatusUnknown),
	string(releasecommon.StatusDeployed),
	string(releasecommon.StatusUninstalled),
	string(releasecommon.StatusFailed),
	string(releasecommon.StatusUninstalling),
	string(releasecommon.StatusPendingInstall),
	string(releasecommon.StatusPendingUpgrade),
	string(releasecommon.StatusPendingRollback),
}

// listCurrentReleases returns the current (non-superseded) revision of every
// release reachable through actionConfig. It pushes the status filter down to
// the Kubernetes API via the storage driver, so only one secret per release is
// fetched and decoded instead of every historical revision (which is what
// helm's action.List does client-side). For storage drivers other than the
// secret driver it falls back to action.List.
func listCurrentReleases(actionConfig *action.Configuration) ([]*release.Release, error) {
	secretsDriver, ok := actionConfig.Releases.Driver.(*driver.Secrets)
	if !ok {
		list := action.NewList(actionConfig)
		list.StateMask = action.ListAll
		raw, err := list.Run()
		if err != nil {
			return nil, err
		}
		out := make([]*release.Release, 0, len(raw))
		for _, r := range raw {
			if re, ok := r.(*release.Release); ok {
				out = append(out, re)
			}
		}
		return out, nil
	}

	// namespace/name -> highest-version current revision. During an in-flight
	// upgrade two non-superseded revisions can coexist (old "deployed" + new
	// "pending-upgrade"); keep the newest, matching helm's filterLatestReleases.
	latest := map[string]*release.Release{}
	for _, status := range nonSupersededStatuses {
		res, err := secretsDriver.Query(map[string]string{"owner": "helm", "status": status})
		if err != nil {
			if errors.Is(err, driver.ErrReleaseNotFound) {
				continue
			}
			return nil, err
		}
		for _, r := range res {
			re, ok := r.(*release.Release)
			if !ok {
				continue
			}
			key := re.Namespace + "/" + re.Name
			if prev, exists := latest[key]; !exists || re.Version > prev.Version {
				latest[key] = re
			}
		}
	}

	out := make([]*release.Release, 0, len(latest))
	for _, re := range latest {
		out = append(out, re)
	}
	return out, nil
}

// secretGVR is the GroupVersionResource of the Secret objects helm uses as its
// default release storage backend.
var secretGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}

// newSecretMetadataClient builds a metadata-only Kubernetes client from the same
// REST config helm uses. A metadata client lists objects as
// PartialObjectMetadata (name, namespace, labels, ...) WITHOUT their .data
// payload, so listing thousands of release secrets transfers a few KB each
// instead of the full gzipped release blob.
func newSecretMetadataClient(settings *cli.EnvSettings) (metadata.Interface, error) {
	restCfg, err := settings.RESTClientGetter().ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return metadata.NewForConfig(restCfg)
}

// listReleaseStubsCached returns one lightweight HelmRelease "stub" per release
// in the given namespace ("" = all namespaces), backed by a short-lived cache.
//
// Stubs carry only what pagination needs (name, namespace, version, lastDeployed
// proxy and status) and are built from secret LABELS via a metadata-only list -
// the expensive gzip blob is never fetched here. The caller decodes the full
// release only for the releases on the requested page. This turns a paginated
// request from O(all releases) decodes into O(page size) decodes.
//
// modifiedAt (a label helm writes on every secret write) is used as the
// lastDeployed sort key: for the current revision it is the time of the last
// deploy/upgrade/rollback, which matches release.Info.LastDeployed closely
// enough for ordering.
func listReleaseStubsCached(namespace string) ([]*HelmRelease, error) {
	cacheKey := "release-stub-index:" + namespace
	if cached, found := helmReleaseListCache.Get(cacheKey); found {
		return cached.([]*HelmRelease), nil
	}

	settings := NewCli()
	settings.SetNamespace(namespace)

	metaClient, err := newSecretMetadataClient(settings)
	if err != nil {
		helmLogger.Error("listReleaseStubsCached metadata client", "namespace", namespace, "error", err.Error())
		return nil, err
	}

	list, err := metaClient.Resource(secretGVR).Namespace(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "owner=helm,status!=" + string(releasecommon.StatusSuperseded),
	})
	if err != nil {
		helmLogger.Error("listReleaseStubsCached list", "namespace", namespace, "error", err.Error())
		return nil, err
	}

	// namespace/name -> highest-version stub. During an in-flight upgrade two
	// non-superseded revisions can coexist (old "deployed" + new
	// "pending-upgrade"); keep the newest, matching helm's filterLatestReleases.
	latest := map[string]*HelmRelease{}
	for i := range list.Items {
		item := &list.Items[i]
		lbls := item.GetLabels()
		name := lbls["name"]
		if name == "" {
			continue
		}
		version, _ := strconv.Atoi(lbls["version"])
		ns := item.GetNamespace()

		key := ns + "/" + name
		if prev, exists := latest[key]; exists && version <= prev.Version {
			continue
		}
		latest[key] = &HelmRelease{
			Name:      name,
			Namespace: ns,
			Version:   version,
			Info: &release.Info{
				LastDeployed: modifiedAtFromLabels(lbls),
				Status:       releasecommon.Status(lbls["status"]),
			},
		}
	}

	stubs := make([]*HelmRelease, 0, len(latest))
	for _, hr := range latest {
		stubs = append(stubs, hr)
	}

	helmReleaseListCache.Set(cacheKey, stubs, cache.DefaultExpiration)
	return stubs, nil
}

// modifiedAtFromLabels parses helm's "modifiedAt" label (unix seconds) into a
// time. Missing/invalid values yield the zero time, which sorts oldest.
func modifiedAtFromLabels(lbls map[string]string) time.Time {
	sec, err := strconv.ParseInt(lbls["modifiedAt"], 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// decodePageRelease fetches and decodes the single release secret identified by
// namespace/name/version through the secrets driver (which gunzips + unmarshals
// for us). It is called only for the releases on the requested page.
func decodePageRelease(secretsDriver *driver.Secrets, namespace, name string, version int) (*release.Release, error) {
	res, err := secretsDriver.Query(map[string]string{
		"owner":   "helm",
		"name":    name,
		"version": strconv.Itoa(version),
	})
	if err != nil {
		return nil, err
	}
	// In all-namespaces mode the same name+version can exist in several
	// namespaces; pick the one we indexed.
	for _, r := range res {
		if re, ok := r.(*release.Release); ok && re.Namespace == namespace {
			return re, nil
		}
	}
	return nil, driver.ErrReleaseNotFound
}

// trimReleaseForList strips everything the list view does not need so the
// response stays small. The detail view fetches the full release separately.
func trimReleaseForList(re *release.Release) {
	if re.Chart != nil {
		re.Chart.Raw = nil
		re.Chart.Files = nil
		re.Chart.Templates = nil
		re.Chart.Values = nil
		re.Chart.Schema = nil
		re.Chart.Lock = nil
	}
	re.Manifest = ""
	re.Config = nil
	re.Hooks = nil
	if re.Info != nil {
		re.Info.Notes = ""
		re.Info.Resources = nil
	}
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
		// helmLogger.Error("HelmReleaseStatus List", "releaseName", data.Release, "namespace", data.Namespace, "error", err.Error())
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
	defer invalidateReleaseListCache() // the release set changed

	settings := NewCli()
	settings.SetNamespace(data.Namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), data.Namespace, ""); err != nil {
		helmLogger.Error("HelmRollback Init", "releaseName", data.Release, "namespace", data.Namespace, "error", err.Error())
		return "", err
	}

	rollback := action.NewRollback(actionConfig)
	rollback.ServerSideApply = "auto"
	rollback.WaitStrategy = kube.StatusWatcherStrategy
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
	var result strings.Builder
	if rel.Hooks != nil {
		result.WriteString("Hooks:\n")
		for _, hook := range rel.Hooks {
			fmt.Fprintf(&result, "  Name: %s\n", hook.Name)
			fmt.Fprintf(&result, "  Kind: %s\n", hook.Kind)
			fmt.Fprintf(&result, "  Manifest: %s\n", hook.Manifest)
			fmt.Fprintf(&result, "  Events: %v\n", hook.Events)
		}
	}

	return result.String()
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

// NewArgoHelmRelease builds a pseudo helm-release entry for an Argo-CD-managed
// chart. releaseName is the helm release name inside the Argo Application
// (spec.source.helm.releaseName); it drives name sorting/filtering and maps to
// the frontend's releaseName.
func NewArgoHelmRelease(releaseName string, info *ArgoReleaseInfo) *HelmRelease {
	return &HelmRelease{
		Name:      releaseName,
		Namespace: info.DestNamespace,
		RepoName:  info.RepoName,
		Argo:      info,
	}
}

// MarshalJSON emits the default helm-release shape for real releases and the
// "git-ops-argo-cd-application" shape (matching the frontend's
// ClusterHelmReleaseDto) for Argo-managed entries.
func (h HelmRelease) MarshalJSON() ([]byte, error) {
	if h.Argo == nil {
		// alias drops the custom MarshalJSON so we get the default field-tag
		// encoding (and don't recurse).
		type alias HelmRelease
		return json.Marshal(alias(h))
	}

	a := h.Argo
	return json.Marshal(map[string]any{
		"type": "git-ops-argo-cd-application",
		"data": map[string]any{
			"application": a.Application,
			"parentApplication": map[string]any{
				"kind":         "Application",
				"plural":       "applications",
				"apiVersion":   "argoproj.io/v1alpha1",
				"namespaced":   true,
				"resourceName": a.ParentName,
				"namespace":    a.ParentNamespace,
			},
			"valuesObject": a.ValuesObject,
		},
		"namespace":   a.DestNamespace,
		"repoName":    a.RepoName,
		"chartName":   a.ChartName,
		"releaseName": h.Name,
		"name":        h.Name,
		"version":     a.Version,
		"appVersion":  a.AppVersion,
		"info":        map[string]any{},
	})
}
