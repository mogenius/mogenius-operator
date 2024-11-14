package utils

import (
	_ "embed"
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/version"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	punqDtos "github.com/mogenius/punq/dtos"
	"gopkg.in/yaml.v3"

	"github.com/ilyakaznacheev/cleanenv"
)

// This object will initially created in secrets when the software is installed into the cluster for the first time (resource: secret -> mogenius/mogenius)
type ClusterSecret struct {
	ApiKey             string
	ClusterMfaId       string
	ClusterName        string
	SyncRepoUrl        string
	SyncRepoPat        string
	SyncRepoBranch     string
	SyncAllowPull      bool
	SyncAllowPush      bool
	SyncFrequencyInSec int
}

type ClusterConfigmap struct {
	SyncWorkloads      []SyncResourceEntry
	AvailableWorkloads []SyncResourceEntry
	IgnoredNamespaces  []string
	IgnoredNames       []string
}

type SyncResourceEntry struct {
	Kind      string  `json:"kind"`
	Name      string  `json:"name"`
	Group     string  `json:"group"`
	Version   string  `json:"version"`
	Namespace *string `json:"namespace"`
}

type SyncResourceData struct {
	Kind      string  `json:"kind"`
	Name      string  `json:"name"`
	Group     string  `json:"group"`
	Version   string  `json:"version"`
	Namespace *string `json:"namespace"`
	YamlData  string  `json:"yamlData"`
}

type SyncResourceItem struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Group        string `json:"group"`
	Version      string `json:"version"`
	ResourceName string `json:"resourceName"`
	Namespace    string `json:"namespace"`
}

func (s *SyncResourceEntry) YamlString() string {
	bytes, err := yaml.Marshal(s)
	if err != nil {
		utilsLogger.Error("Error marshalling SyncResourceEntry", "error", err)
		return ""
	}
	return string(bytes)
}

func ToYaml(data interface{}) (string, error) {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func YamlStringFromSyncResourceDescription(s []SyncResourceEntry) string {
	result := "\n"
	for _, v := range s {
		result += fmt.Sprintf("Name: %s, Group: %s, Namespace: %v\n", v.Name, v.Group, v.Namespace)
	}
	return result
}

func SyncResourceEntryFromYaml(str string) *SyncResourceEntry {
	var s SyncResourceEntry
	err := yaml.Unmarshal([]byte(str), &s)
	if err != nil {
		utilsLogger.Error("Error unmarshalling SyncResourceEntry", "error", err)
		return nil
	}
	return &s
}

const CONFIGVERSION = 2

const STAGE_DEV = "dev"
const STAGE_PROD = "prod"
const STAGE_LOCAL = "local"

type Config struct {
	Kubernetes struct {
		RunInCluster               bool   `yaml:"run_in_cluster" env:"run_in_cluster" env-description:"If set to true, the application will run in the cluster (using the service account token). Otherwise it will try to load your local default context." env-default:"false"`
		LocalContainerRegistryHost string `yaml:"local_registry_host" env:"local_registry_host" env-description:"Local container registry inside the cluster" env-default:"mocr.local.mogenius.io"`
	} `yaml:"kubernetes"`
	Iac struct {
		RepoUrl            string              `yaml:"repo_url" env:"sync_repo_url" env-description:"Sync repo url."`
		RepoPat            string              `yaml:"repo_pat" env:"sync_repo_pat" env-description:"Sync repo pat."`
		RepoBranch         string              `yaml:"repo_pat_branch" env:"sync_repo_branch" env-description:"Sync repo branch."`
		SyncFrequencyInSec int                 `yaml:"sync_requency_secs" env:"sync_requency_secs" env-description:"Polling interval for sync in seconds." env-default:"10"`
		AllowPush          bool                `yaml:"allow_push" env:"sync_allow_push" env-description:"Allow IAC manager to push data to repo."`
		AllowPull          bool                `yaml:"allow_pull" env:"sync_allow_pull" env-description:"Allow IAC manager to pull data from repo."`
		ShowDiffInLog      bool                `yaml:"show_diff_in_log" env:"sync_show_diff_in_log" env-description:"Show all changes of resources as diff in operator log."`
		AvailableWorkloads []SyncResourceEntry `yaml:"sync_available_workloads" env:"sync_available_workloads" env-description:"List of all workloads to sync."`
		SyncWorkloads      []SyncResourceEntry `yaml:"sync_workloads" env:"sync_workloads" env-description:"List of all workloads to sync. Default is one entry with * which means all."`
		IgnoredNamespaces  []string            `yaml:"ignored_namespaces" env:"sync_ignored_namespaces" env-description:"List of all ignored namespaces."`
		IgnoredNames       []string            `yaml:"ignored_names" env:"sync_ignored_names" env-description:"List of strings which are ignored when for sync. This list may include regex."`
		LogChanges         bool                `yaml:"log_changes" env:"sync_log_changes" env-description:"Resource changes in kubernetes will create a log entry."`
	} `yaml:"iac"`
	Misc struct {
		Stage                  string   `yaml:"stage" env:"stage" env-description:"mogenius k8s-manager stage" env-default:"prod"`
		LogIncomingStats       bool     `yaml:"log_incoming_stats" env:"log_incoming_stats" env-description:"Scraper data input will be logged visibly when set to true." env-default:"false"`
		DefaultMountPath       string   `yaml:"default_mount_path" env:"default_mount_path" env-description:"All containers will have access to this mount point"`
		IgnoreNamespaces       []string `yaml:"ignore_namespaces" env:"ignore_namespaces" env-description:"List of all ignored namespaces." env-default:""`
		AutoMountNfs           bool     `yaml:"auto_mount_nfs" env:"auto_mount_nfs" env-description:"If set to true, nfs pvc will automatically be mounted." env-default:"true"`
		CheckForUpdates        int      `yaml:"check_for_updates" env:"check_for_updates" env-description:"Time interval between update checks." env-default:"86400"`
		HelmIndex              string   `yaml:"helm_index" env:"helm_index" env-description:"URL of the helm index file." env-default:"https://helm.mogenius.com/public/index.yaml"`
		NfsPodPrefix           string   `yaml:"nfs_pod_prefix" env:"nfs_pod_prefix" env-description:"A prefix for the nfs-server pod. This will always be applied in order to detect the pod."`
		ExternalSecretsEnabled bool     `yaml:"external_secrets_enabled" env:"external_secrets_enabled" env-description:"If set to true, external secrets will be enabled." env-default:"false"`
	} `yaml:"misc"`
	Builder struct {
		BuildTimeout        int `yaml:"max_build_time" env:"max_build_time" env-description:"Seconds until the build will be canceled." env-default:"3600"`
		ScanTimeout         int `yaml:"max_scan_time" env:"max_build_time" env-description:"Seconds until the vulnerability scan will be canceled." env-default:"200"`
		MaxConcurrentBuilds int `yaml:"max_concurrent_builds" env:"max_concurrent_builds" env-description:"Number of concurrent builds." env-default:"1"`
	} `yaml:"builder"`
	Git struct {
		GitUserEmail string `yaml:"git_user_email" env:"git_user_email" env-description:"Email address which is used when interacting with git." env-default:"git@mogenius.com"`
		GitUserName  string `yaml:"git_user_name" env:"git_user_name" env-description:"User name which is used when interacting with git." env-default:"mogenius git-user"`
	} `yaml:"git"`
	Stats struct {
		MaxDataPoints int `yaml:"max_data_points" env:"max_data_points" env-description:"After x data points in bucket will be overwritten LIFO principle." env-default:"6000"`
	} `yaml:"stats"`
}

//go:embed config/config-local.yaml
var DefaultConfigLocalFile string

//go:embed config/config-cluster-dev.yaml
var DefaultConfigClusterFileDev string

//go:embed config/config-cluster-prod.yaml
var DefaultConfigClusterFileProd string

var CONFIG Config
var ConfigPath string
var ClusterProviderCached punqDtos.KubernetesProvider = punqDtos.UNKNOWN

// preconfigure with dtos
var IacWorkloadConfigMap map[string]bool

func InitConfigSimple(stage string) {
	path := os.TempDir() + "/config.yaml"

	switch stage {
	case STAGE_DEV:
		err := os.WriteFile(path, []byte(DefaultConfigClusterFileDev), 0755)
		if err != nil {
			utilsLogger.Error("failed to write default 'dev' config file", "path", path, "error", err)
		}
	case STAGE_LOCAL:
		err := os.WriteFile(path, []byte(DefaultConfigLocalFile), 0755)
		if err != nil {
			utilsLogger.Error("failed to write default 'local' config file", "path", path, "error", err)
		}
	case STAGE_PROD:
		err := os.WriteFile(path, []byte(DefaultConfigClusterFileProd), 0755)
		if err != nil {
			utilsLogger.Error("failed to write default config file", "path", path, "error", err)
		}
	}
	err := cleanenv.ReadConfig(path, &CONFIG)
	if err != nil {
		utilsLogger.Error("failed to read config file", "path", path, "error", err)
	}
}

func InitConfigYaml(showDebug bool, customConfigName string, stage string) {
	// try to load stage if not set
	if stage == "" {
		stage = strings.ToLower(os.Getenv("STAGE"))
	}
	if stage == "" {
		stage = strings.ToLower(os.Getenv("stage"))
	}

	_, ConfigPath = GetDirectories(customConfigName)

	// create default config if not exists
	// if stage is set, then we overwrite the config
	if stage == "" {
		if _, err := os.Stat(ConfigPath); err == nil || os.IsExist(err) {
			// do nothing, file exists
		} else {
			writeDefaultConfig(stage)
		}
	} else {
		writeDefaultConfig(stage)
	}

	// read configuration from the file and environment variables
	if err := cleanenv.ReadConfig(ConfigPath, &CONFIG); err != nil {
		if strings.HasPrefix(err.Error(), "config file parsing error:") {
			utilsLogger.Error("Config file is corrupted. Creating a new one by using -r flag.")
		}
		utilsLogger.Error("Error reading config", "path", ConfigPath, "error", err)
	}

	if CONFIG.Kubernetes.RunInCluster {
		ConfigPath = "RUNS_IN_CLUSTER_NO_CONFIG_NEEDED"
	}

	// SET DEFAULTS if missing
	dirPath, _ := os.Getwd()

	if !CONFIG.Kubernetes.RunInCluster {
		dirPath, err := os.MkdirTemp("", "mo_*")
		if err != nil {
			utilsLogger.Error("failed to create temp dir", "error", err)
			shutdown.SendShutdownSignal(true)
			select {}
		}
		utilsLogger.Info("TempDir created", "path", dirPath)
	}

	if CONFIG.Misc.DefaultMountPath == "" {
		CONFIG.Misc.DefaultMountPath = filepath.Join(dirPath, "mo-data")
	}

	// CHECKS FOR CLUSTER
	if CONFIG.Kubernetes.RunInCluster {
		clusterName := config.Get("MO_CLUSTER_NAME")
		if clusterName == "your-cluster-name" || clusterName == "" {
			utilsLogger.Error("Environment Variable 'cluster_name' not setup. TERMINATING.")
			shutdown.SendShutdownSignal(true)
			select {}
		}
		apiKey := config.Get("MO_API_KEY")
		if apiKey == "YOUR_API_KEY" || apiKey == "" {
			utilsLogger.Error("Environment Variable 'api_key' not setup or default value not overwritten. TERMINATING.")
			shutdown.SendShutdownSignal(true)
			select {}
		}
	}

	// SET LOGGING
	// setupLogging()

	moDebug, err := strconv.ParseBool(config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	if moDebug {
		utilsLogger.Info("Starting service for pprof in localhost:6060")
		go func() {
			err := http.ListenAndServe("localhost:6060", nil)
			if err != nil {
				utilsLogger.Error("failed to start debug service", "error", err)
				shutdown.SendShutdownSignal(true)
				select {}
			}
			utilsLogger.Info("1. Portforward mogenius-k8s-manager to 6060")
			utilsLogger.Info("2. wget http://localhost:6060/debug/pprof/profile?seconds=60 -O cpu.pprof")
			utilsLogger.Info("3. wget http://localhost:6060/debug/pprof/heap -O mem.pprof")
			utilsLogger.Info("4. go tool pprof -http=localhost:8081 cpu.pprof")
			utilsLogger.Info("5. go tool pprof -http=localhost:8081 mem.pprof")
			utilsLogger.Info("OR: go tool pprof mem.pprof -> Then type in commands like top, top --cum, list")
			utilsLogger.Info("http://localhost:6060/debug/pprof/ This is the index page that lists all available profiles.")
			utilsLogger.Info("http://localhost:6060/debug/pprof/profile This serves a CPU profile. You can set the profiling duration through the seconds parameter. For example, ?seconds=30 would profile your CPU for 30 seconds.")
			utilsLogger.Info("http://localhost:6060/debug/pprof/heap This serves a snapshot of the current heap memory usage.")
			utilsLogger.Info("http://localhost:6060/debug/pprof/goroutine This serves a snapshot of the current goroutines stack traces.")
			utilsLogger.Info("http://localhost:6060/debug/pprof/block This serves a snapshot of stack traces that led to blocking on synchronization primitives.")
			utilsLogger.Info("http://localhost:6060/debug/pprof/threadcreate This serves a snapshot of all OS thread creation stack traces.")
			utilsLogger.Info("http://localhost:6060/debug/pprof/cmdline This returns the command line invocation of the current program.")
			utilsLogger.Info("http://localhost:6060/debug/pprof/symbol This is used to look up the program counters listed in a pprof profile.")
			utilsLogger.Info("http://localhost:6060/debug/pprof/trace This serves a trace of execution of the current program. You can set the trace duration through the seconds parameter.")
		}()
	}
}

func PrintCurrentCONFIG() (string, error) {
	// create a deep copy of the Config instance
	var configCopy Config
	yamlData, err := yaml.Marshal(&CONFIG)
	if err != nil {
		return "", err
	}
	err = yaml.Unmarshal(yamlData, &configCopy)
	if err != nil {
		return "", err
	}

	// reset data for local usage
	configCopy.Misc.DefaultMountPath = ""
	configCopy.Kubernetes.RunInCluster = false

	// marshal the copy to yaml
	yamlData, err = yaml.Marshal(&configCopy)
	if err != nil {
		fmt.Printf("Error marshalling to YAML: %v\n", err)
		return "", err
	}
	return string(yamlData), nil
}

func SetupClusterSecret(clusterSecret ClusterSecret) {
	if clusterSecret.ClusterMfaId != "" {
		err := config.TrySet("MO_API_KEY", clusterSecret.ApiKey)
		if err != nil {
			utilsLogger.Debug("failed to set MO_API_KEY", "error", err)
		}
		err = config.TrySet("MO_CLUSTER_NAME", clusterSecret.ClusterName)
		if err != nil {
			utilsLogger.Debug("failed to set MO_CLUSTER_NAME", "error", err)
		}
		err = config.TrySet("MO_CLUSTER_MFA_ID", clusterSecret.ClusterMfaId)
		if err != nil {
			utilsLogger.Debug("failed to set MO_CLUSTER_MFA_ID", "error", err)
		}
		CONFIG.Iac.RepoUrl = clusterSecret.SyncRepoUrl
		CONFIG.Iac.RepoPat = clusterSecret.SyncRepoPat
		CONFIG.Iac.RepoBranch = clusterSecret.SyncRepoBranch
		CONFIG.Iac.AllowPull = clusterSecret.SyncAllowPull
		CONFIG.Iac.AllowPush = clusterSecret.SyncAllowPush

		if clusterSecret.SyncFrequencyInSec <= 5 {
			clusterSecret.SyncFrequencyInSec = 5
		} else {
			CONFIG.Iac.SyncFrequencyInSec = clusterSecret.SyncFrequencyInSec
		}
	}
}

func SetupClusterConfigmap(clusterConfigmap ClusterConfigmap) {
	CONFIG.Iac.SyncWorkloads = clusterConfigmap.SyncWorkloads
	CONFIG.Iac.IgnoredNamespaces = clusterConfigmap.IgnoredNamespaces
	CONFIG.Iac.AvailableWorkloads = clusterConfigmap.AvailableWorkloads
	CONFIG.Iac.IgnoredNames = clusterConfigmap.IgnoredNames
}

func PrintSettings() {
	utilsLogger.Info("PrintSettings",
		"ConfigPath", ConfigPath,
		"Kubernetes.OwnNamespace", config.Get("MO_OWN_NAMESPACE"),
		"Kubernetes.ClusterName", config.Get("MO_CLUSTER_NAME"),
		"Kubernetes.ClusterMfaId", config.Get("MO_CLUSTER_MFA_ID"),
		"Kubernetes.RunInCluster", CONFIG.Kubernetes.RunInCluster,
		"Kubernetes.ApiKey", config.Get("MO_API_KEY"),
		"Kubernetes.HelmDataPath", config.Get("MO_HELM_DATA_PATH"),
		"Kubernetes.GitVaultDataPath", config.Get("MO_GIT_VAULT_DATA_PATH"),
		"Kubernetes.BboltDbPath", config.Get("MO_BBOLT_DB_PATH"),
		"Kubernetes.BboltDbStatsPath", config.Get("MO_BBOLT_DB_STATS_PATH"),
		"Kubernetes.LogDataPath", config.Get("MO_LOG_DIR"),
		"Kubernetes.LocalContainerRegistryHost", CONFIG.Kubernetes.LocalContainerRegistryHost,
		"Iac.RepoUrl", CONFIG.Iac.RepoUrl,
		"Iac.RepoPat", CONFIG.Iac.RepoPat,
		"Iac.RepoBranch", CONFIG.Iac.RepoBranch,
		"Iac.SyncFrequencyInSec", CONFIG.Iac.SyncFrequencyInSec,
		"Iac.AllowPull", CONFIG.Iac.AllowPull,
		"Iac.AllowPush", CONFIG.Iac.AllowPush,
		"Iac.SyncWorkloads", CONFIG.Iac.SyncWorkloads,
		"Iac.AvailableWorkloads", CONFIG.Iac.AvailableWorkloads,
		"Iac.IgnoredNamespaces", CONFIG.Iac.IgnoredNamespaces,
		"Iac.IgnoredNames", CONFIG.Iac.IgnoredNames,
		"Iac.LogChanges", CONFIG.Iac.LogChanges,
		"Iac.ShowDiffInLog", CONFIG.Iac.ShowDiffInLog,
		"Misc.Stage", config.Get("MO_STAGE"),
		"Misc.LogIncomingStats", CONFIG.Misc.LogIncomingStats,
		"Misc.Debug", config.Get("MO_DEBUG"),
		"Misc.AutoMountNfs", CONFIG.Misc.AutoMountNfs,
		"Misc.DefaultMountPath", CONFIG.Misc.DefaultMountPath,
		"Misc.IgnoreNamespaces", CONFIG.Misc.IgnoreNamespaces,
		"Misc.CheckForUpdates", CONFIG.Misc.CheckForUpdates,
		"Misc.HelmIndex", CONFIG.Misc.HelmIndex,
		"Misc.NfsPodPrefix", CONFIG.Misc.NfsPodPrefix,
		"Builder.BuildTimeout", CONFIG.Builder.BuildTimeout,
		"Builder.ScanTimeout", CONFIG.Builder.ScanTimeout,
		"Builder.MaxConcurrentBuilds", CONFIG.Builder.MaxConcurrentBuilds,
		"Git.GitUserEmail", config.Get("MO_GIT_USER_EMAIL"),
		"Git.GitUserName", config.Get("MO_GIT_USER_NAME"),
		"Stats.MaxDataPoints", CONFIG.Stats.MaxDataPoints,
	)
}

func PrintVersionInfo() {
	utilsLogger.Info(
		"mogenius-k8s-manager",
		"Version", version.Ver,
		"Branch", version.Branch,
		"GitCommitHash", version.GitCommitHash,
		"BuildTimestamp", version.BuildTimestamp,
	)
}

func GetDirectories(customConfigPath string) (configDir string, configPath string) {
	homeDirName, err := os.UserHomeDir()
	if err != nil {
		utilsLogger.Error("Error retrieving user homedir", "error", err)
	}

	if customConfigPath != "" {
		if _, err := os.Stat(configPath); err == nil || os.IsExist(err) {
			configPath = customConfigPath
			configDir = filepath.Dir(customConfigPath)
		} else {
			utilsLogger.Error("Custom config not found.", "customConfigPath", customConfigPath)
		}
	} else {
		configDir = homeDirName + "/.mogenius-k8s-manager/"
		configPath = configDir + "config.yaml"
	}

	return configDir, configPath
}

func DeleteCurrentConfig() {
	_, configPath := GetDirectories("")
	err := os.Remove(configPath)
	if err != nil {
		utilsLogger.Error("failed to delete config file", "error", err)
	} else {
		utilsLogger.Info("succesfully deleted config file", "configPath", configPath)
	}
}

func writeDefaultConfig(stage string) {
	configDir, configPath := GetDirectories("")

	// write it to default location
	err := os.Mkdir(configDir, 0755)
	if err != nil && err.Error() != "mkdir "+configDir+": file exists" {
		utilsLogger.Warn("failed to create directory", "path", configDir, "error", err)
	}

	// check if stage is set via env variable
	envVarStage := strings.ToLower(os.Getenv("stage"))
	if envVarStage != "" {
		stage = envVarStage
	} else {
		// default stage is prod
		if stage == "" {
			stage = STAGE_PROD
		}
	}

	if stage == STAGE_DEV {
		err = os.WriteFile(configPath, []byte(DefaultConfigClusterFileDev), 0755)
	} else if stage == STAGE_PROD {
		err = os.WriteFile(configPath, []byte(DefaultConfigClusterFileProd), 0755)
	} else if stage == STAGE_LOCAL {
		err = os.WriteFile(configPath, []byte(DefaultConfigLocalFile), 0755)
	} else {
		utilsLogger.Warn("No stage set. Using local config.")
		err = os.WriteFile(configPath, []byte(DefaultConfigLocalFile), 0755)
	}
	if err != nil {
		utilsLogger.Error("Error writing "+configPath+" file", "configPath", configPath, "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}
}
