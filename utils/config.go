package utils

import (
	_ "embed"
	"fmt"
	"mogenius-k8s-manager/version"
	"net/http"
	"os"
	"path/filepath"
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
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Group      string `json:"group"`
	Version    string `json:"version"`
	Namespaced bool   `json:"namespaced"`
}

type SyncResourceData struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Group      string `json:"group"`
	Version    string `json:"version"`
	Namespaced bool   `json:"namespaced"`
	YamlData   string `json:"yamlData"`
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
		UtilsLogger.Error("Error marshalling SyncResourceEntry", "error", err)
		return ""
	}
	return string(bytes)
}

func YamlStringFromSyncResource(s []SyncResourceEntry) string {
	bytes, err := yaml.Marshal(s)
	if err != nil {
		UtilsLogger.Error("Error marshalling SyncResourceEntry", "error", err)
		return ""
	}

	return string(bytes)
}

func YamlStringFromSyncResourceDescription(s []SyncResourceEntry) string {
	result := "\n"
	for _, v := range s {
		result += fmt.Sprintf("Name: %s, Group: %s, Namespaced: %t\n", v.Name, v.Group, v.Namespaced)
	}
	return result
}

func SyncResourceEntryFromYaml(str string) *SyncResourceEntry {
	var s SyncResourceEntry
	err := yaml.Unmarshal([]byte(str), &s)
	if err != nil {
		UtilsLogger.Error("Error unmarshalling SyncResourceEntry", "error", err)
		return nil
	}
	return &s
}

const CONFIGVERSION = 2

const STAGE_DEV = "dev"
const STAGE_PROD = "prod"
const STAGE_LOCAL = "local"

type Config struct {
	Config struct {
		Version int `yaml:"version" env-description:"Version of the configuration yaml."`
	} `yaml:"config"`
	Kubernetes struct {
		ApiKey                     string `yaml:"api_key" env:"api_key" env-description:"Api Key to access the server"`
		ClusterName                string `yaml:"cluster_name" env:"cluster_name" env-description:"The Name of the Kubernetes Cluster"`
		OwnNamespace               string `yaml:"own_namespace" env:"OWN_NAMESPACE" env-description:"The Namespace of mogenius platform"`
		ClusterMfaId               string `yaml:"cluster_mfa_id" env:"cluster_mfa_id" env-description:"NanoId of the Kubernetes Cluster for MFA purpose"`
		RunInCluster               bool   `yaml:"run_in_cluster" env:"run_in_cluster" env-description:"If set to true, the application will run in the cluster (using the service account token). Otherwise it will try to load your local default context." env-default:"false"`
		HelmDataPath               string `yaml:"helm_data_path" env:"helm_data_path" env-description:"Path to the Helm data."`
		GitVaultDataPath           string `yaml:"git_vault_data_path" env:"git_vault_data_path" env-description:"Path to the Git Vault data."`
		BboltDbPath                string `yaml:"bbolt_db_path" env:"bbolt_db_path" env-description:"Path to the bbolt database. This db stores build-related information."`
		BboltDbStatsPath           string `yaml:"bbolt_db_stats_path" env:"bbolt_db_stats_path" env-description:"Path to the bbolt database. This db stores stats-related information."`
		LogDataPath                string `yaml:"log_data_path" env:"log_data_path" env-description:"Path to the log data."`
		LocalContainerRegistryHost string `yaml:"local_registry_host" env:"local_registry_host" env-description:"Local container registry inside the cluster" env-default:"mocr.local.mogenius.io"`
	} `yaml:"kubernetes"`
	ApiServer struct {
		Http_Server string `yaml:"http_server" env:"api_http_server" env-description:"Server host" env-default:"https://platform-api.mogenius.com"`
		Ws_Server   string `yaml:"ws_server" env:"api_ws_server" env-description:"Server host" env-default:"127.0.0.1:8080"`
		Ws_Scheme   string `yaml:"ws_server_scheme" env:"api_ws_scheme" env-description:"Server host scheme. (ws/wss)" env-default:"wss"`
		WS_Path     string `yaml:"ws_path" env:"api_ws_path" env-description:"Server Path" env-default:"/ws"`
	} `yaml:"api_server"`
	EventServer struct {
		Server string `yaml:"server" env:"event_server" env-description:"Server host" env-default:"127.0.0.1:8080"`
		Scheme string `yaml:"scheme" env:"event_scheme" env-description:"Server host scheme. (ws/wss)" env-default:"wss"`
		Path   string `yaml:"path" env:"event_path" env-description:"Server Path" env-default:"/ws-event"`
	} `yaml:"event_server"`
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
		Stage                     string   `yaml:"stage" env:"stage" env-description:"mogenius k8s-manager stage" env-default:"prod"`
		LogFormat                 string   `yaml:"log_format" env:"log_format" env-description:"Setup the log format. Available are: json | text" env-default:"json"`
		LogLevel                  string   `yaml:"log_level" env:"log_level" env-description:"Setup the log level. Available are: error, warn, info, debug" env-default:"info"`
		LogIncomingStats          bool     `yaml:"log_incoming_stats" env:"log_incoming_stats" env-description:"Scraper data input will be logged visibly when set to true." env-default:"false"`
		Debug                     bool     `yaml:"debug" env:"debug" env-description:"If set to true, debug features will be enabled." env-default:"false"`
		DebugLogCaller            bool     `yaml:"debug_log_caller" env:"debug_log_caller" env-description:"If set to true, the calling function will be logged." env-default:"false"`
		LogKubernetesEvents       bool     `yaml:"log_kubernetes_events" env:"log_kubernetes_events" env-description:"If set to true, all kubernetes events will be logged to std-out." env-default:"false"`
		DefaultMountPath          string   `yaml:"default_mount_path" env:"default_mount_path" env-description:"All containers will have access to this mount point"`
		IgnoreNamespaces          []string `yaml:"ignore_namespaces" env:"ignore_namespaces" env-description:"List of all ignored namespaces." env-default:""`
		LogRotationSizeInBytes    int      `yaml:"log_rotation_size_in_bytes" env:"log_rotation_size_in_bytes" env-description:"Size of the logfile when it is rotated." env-default:"5242880"`
		LogRotationMaxSizeInBytes int      `yaml:"log_rotation_max_size_in_bytes" env:"log_rotation_max_size_in_bytes" env-description:"Size of the max logfile when it is rotated." env-default:"314572800"`
		LogRetentionDays          int      `yaml:"log_retention_days" env:"log_retention_days" env-description:"Number of days to keep log files." env-default:"7"`
		AutoMountNfs              bool     `yaml:"auto_mount_nfs" env:"auto_mount_nfs" env-description:"If set to true, nfs pvc will automatically be mounted." env-default:"true"`
		IgnoreResourcesBackup     []string `yaml:"ignore_resources_backup" env:"ignore_resources_backup" env-description:"List of all ignored resources while backup." env-default:""`
		CheckForUpdates           int      `yaml:"check_for_updates" env:"check_for_updates" env-description:"Time interval between update checks." env-default:"86400"`
		HelmIndex                 string   `yaml:"helm_index" env:"helm_index" env-description:"URL of the helm index file." env-default:"https://helm.mogenius.com/public/index.yaml"`
		NfsPodPrefix              string   `yaml:"nfs_pod_prefix" env:"nfs_pod_prefix" env-description:"A prefix for the nfs-server pod. This will always be applied in order to detect the pod."`
		ExternalSecretsEnabled    bool     `yaml:"external_secrets_enabled" env:"external_secrets_enabled" env-description:"If set to true, external secrets will be enabled." env-default:"false"`
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
			UtilsLogger.Error("failed to write default 'dev' config file", "path", path, "error", err)
		}
	case STAGE_LOCAL:
		err := os.WriteFile(path, []byte(DefaultConfigLocalFile), 0755)
		if err != nil {
			UtilsLogger.Error("failed to write default 'local' config file", "path", path, "error", err)
		}
	case STAGE_PROD:
		err := os.WriteFile(path, []byte(DefaultConfigClusterFileProd), 0755)
		if err != nil {
			UtilsLogger.Error("failed to write default config file", "path", path, "error", err)
		}
	}
	err := cleanenv.ReadConfig(path, &CONFIG)
	if err != nil {
		UtilsLogger.Error("failed to read config file", "path", path, "error", err)
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
			UtilsLogger.Error("Config file is corrupted. Creating a new one by using -r flag.")
		}
		UtilsLogger.Error("Error reading config", "path", ConfigPath, "error", err)
	}

	if CONFIG.Kubernetes.RunInCluster {
		ConfigPath = "RUNS_IN_CLUSTER_NO_CONFIG_NEEDED"
	}

	// SET DEFAULTS if missing
	dirPath, _ := os.Getwd()

	if !CONFIG.Kubernetes.RunInCluster {
		dirPath, err := os.MkdirTemp("", "mo_*")
		if err != nil {
			UtilsLogger.Error("failed to create temp dir", "error", err)
			panic(1)
		}
		UtilsLogger.Info("TempDir created", "path", dirPath)
	}

	if CONFIG.Kubernetes.BboltDbPath == "" {
		CONFIG.Kubernetes.BboltDbPath = filepath.Join(dirPath, "mogenius.db")
	}
	if CONFIG.Kubernetes.HelmDataPath == "" {
		CONFIG.Kubernetes.HelmDataPath = filepath.Join(dirPath, "helm-data")
	}
	if CONFIG.Kubernetes.GitVaultDataPath == "" {
		CONFIG.Kubernetes.GitVaultDataPath = filepath.Join(dirPath, "git-vault-data")
	}
	if CONFIG.Kubernetes.BboltDbStatsPath == "" {
		CONFIG.Kubernetes.BboltDbStatsPath = filepath.Join(dirPath, "mogenius-stats.db")
	}
	if CONFIG.Misc.DefaultMountPath == "" {
		CONFIG.Misc.DefaultMountPath = filepath.Join(dirPath, "mo-data")
	}
	if CONFIG.Kubernetes.LogDataPath == "" {
		CONFIG.Kubernetes.LogDataPath = filepath.Join(dirPath, "logs")
	}

	// CHECKS FOR CLUSTER
	if CONFIG.Kubernetes.RunInCluster {
		if CONFIG.Kubernetes.ClusterName == "your-cluster-name" || CONFIG.Kubernetes.ClusterName == "" {
			UtilsLogger.Error("Environment Variable 'cluster_name' not setup. TERMINATING.")
			panic(1)
		}
		if CONFIG.Kubernetes.ApiKey == "YOUR_API_KEY" || CONFIG.Kubernetes.ApiKey == "" {
			UtilsLogger.Error("Environment Variable 'api_key' not setup or default value not overwritten. TERMINATING.")
			panic(1)
		}
	}

	if CONFIGVERSION > CONFIG.Config.Version {
		UtilsLogger.Error("Config version is outdated. Please delete your config file and restart the application.", "ConfigPath", ConfigPath, "version", CONFIG.Config.Version, "neededVersion", CONFIGVERSION)
		panic(1)
	}

	// SET LOGGING
	// setupLogging()

	if CONFIG.Misc.Debug {
		UtilsLogger.Info("Starting service for pprof in localhost:6060")
		go func() {
			err := http.ListenAndServe("localhost:6060", nil)
			if err != nil {
				panic(err)
			}
			UtilsLogger.Info("1. Portforward mogenius-k8s-manager to 6060")
			UtilsLogger.Info("2. wget http://localhost:6060/debug/pprof/profile?seconds=60 -O cpu.pprof")
			UtilsLogger.Info("3. wget http://localhost:6060/debug/pprof/heap -O mem.pprof")
			UtilsLogger.Info("4. go tool pprof -http=localhost:8081 cpu.pprof")
			UtilsLogger.Info("5. go tool pprof -http=localhost:8081 mem.pprof")
			UtilsLogger.Info("OR: go tool pprof mem.pprof -> Then type in commands like top, top --cum, list")
			UtilsLogger.Info("http://localhost:6060/debug/pprof/ This is the index page that lists all available profiles.")
			UtilsLogger.Info("http://localhost:6060/debug/pprof/profile This serves a CPU profile. You can set the profiling duration through the seconds parameter. For example, ?seconds=30 would profile your CPU for 30 seconds.")
			UtilsLogger.Info("http://localhost:6060/debug/pprof/heap This serves a snapshot of the current heap memory usage.")
			UtilsLogger.Info("http://localhost:6060/debug/pprof/goroutine This serves a snapshot of the current goroutines stack traces.")
			UtilsLogger.Info("http://localhost:6060/debug/pprof/block This serves a snapshot of stack traces that led to blocking on synchronization primitives.")
			UtilsLogger.Info("http://localhost:6060/debug/pprof/threadcreate This serves a snapshot of all OS thread creation stack traces.")
			UtilsLogger.Info("http://localhost:6060/debug/pprof/cmdline This returns the command line invocation of the current program.")
			UtilsLogger.Info("http://localhost:6060/debug/pprof/symbol This is used to look up the program counters listed in a pprof profile.")
			UtilsLogger.Info("http://localhost:6060/debug/pprof/trace This serves a trace of execution of the current program. You can set the trace duration through the seconds parameter.")
		}()
	}
}

//func setupLogging() {
//	// Create a log file
//	err := os.MkdirAll(CONFIG.Kubernetes.LogDataPath, os.ModePerm)
//	if err != nil {
//		log.Fatalf("Failed to create parent directories: %v", err)
//	}
//	file, err := os.OpenFile(MainLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
//	if err != nil {
//		log.Fatalf("Failed to open log file: %v", err)
//	}
//
//	mw := io.MultiWriter(os.Stdout, file)
//
//	log.SetOutput(mw)
//	log.SetLevel(log.TraceLevel)
//
//	log.AddHook(&SecretRedactionHook{})
//	log.AddHook(&LogRotationHook{})
//
//	log.SetFormatter(&log.TextFormatter{
//		ForceColors:      true,
//		DisableTimestamp: false,
//		DisableQuote:     true,
//	})
//
//	log.SetReportCaller(CONFIG.Misc.DebugLogCaller)
//	logLevel, err := log.ParseLevel(CONFIG.Misc.LogLevel)
//	if err != nil {
//		logLevel = log.InfoLevel
//		log.Error("Error parsing log level. Using default log level: info")
//	}
//	log.SetLevel(logLevel)
//
//	if strings.ToLower(CONFIG.Misc.LogFormat) == "json" {
//		log.SetFormatter(&log.JSONFormatter{})
//	} else if strings.ToLower(CONFIG.Misc.LogFormat) == "text" {
//		log.SetFormatter(&log.TextFormatter{
//			ForceColors:      true,
//			DisableTimestamp: false,
//			DisableQuote:     true,
//		})
//	} else {
//		log.SetFormatter(&log.TextFormatter{})
//	}
//}

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
	configCopy.Kubernetes.HelmDataPath = ""
	configCopy.Kubernetes.GitVaultDataPath = ""
	configCopy.Kubernetes.BboltDbPath = ""
	configCopy.Kubernetes.BboltDbStatsPath = ""
	configCopy.Kubernetes.LogDataPath = ""
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
		CONFIG.Kubernetes.ClusterMfaId = clusterSecret.ClusterMfaId
		CONFIG.Kubernetes.ApiKey = clusterSecret.ApiKey
		CONFIG.Kubernetes.ClusterName = clusterSecret.ClusterName
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
	UtilsLogger.Info("PrintSettings",
		"ConfigPath", ConfigPath,
		"Version", CONFIG.Config.Version,
		"Kubernetes.OwnNamespace", CONFIG.Kubernetes.OwnNamespace,
		"Kubernetes.ClusterName", CONFIG.Kubernetes.ClusterName,
		"Kubernetes.ClusterMfaId", CONFIG.Kubernetes.ClusterMfaId,
		"Kubernetes.RunInCluster", CONFIG.Kubernetes.RunInCluster,
		"Kubernetes.ApiKey", CONFIG.Kubernetes.ApiKey,
		"Kubernetes.HelmDataPath", CONFIG.Kubernetes.HelmDataPath,
		"Kubernetes.GitVaultDataPath", CONFIG.Kubernetes.GitVaultDataPath,
		"Kubernetes.BboltDbPath", CONFIG.Kubernetes.BboltDbPath,
		"Kubernetes.BboltDbStatsPath", CONFIG.Kubernetes.BboltDbStatsPath,
		"Kubernetes.LogDataPath", CONFIG.Kubernetes.LogDataPath,
		"Kubernetes.LocalContainerRegistryHost", CONFIG.Kubernetes.LocalContainerRegistryHost,
		"ApiServer.Http_Server", CONFIG.ApiServer.Http_Server,
		"ApiServer.Ws_Server", CONFIG.ApiServer.Ws_Server,
		"ApiServer.Ws_Scheme", CONFIG.ApiServer.Ws_Scheme,
		"ApiServer.WS_Path", CONFIG.ApiServer.WS_Path,
		"EventServer.Server", CONFIG.EventServer.Server,
		"EventServer.Scheme", CONFIG.EventServer.Scheme,
		"EventServer.Path", CONFIG.EventServer.Path,
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
		"Misc.Stage", CONFIG.Misc.Stage,
		"Misc.LogFormat", CONFIG.Misc.LogFormat,
		"Misc.LogIncomingStats", CONFIG.Misc.LogIncomingStats,
		"Misc.Debug", CONFIG.Misc.Debug,
		"Misc.DebugLogCaller", CONFIG.Misc.DebugLogCaller,
		"Misc.AutoMountNfs", CONFIG.Misc.AutoMountNfs,
		"Misc.LogKubernetesEvents", CONFIG.Misc.LogKubernetesEvents,
		"Misc.DefaultMountPath", CONFIG.Misc.DefaultMountPath,
		"Misc.IgnoreResourcesBackup", CONFIG.Misc.IgnoreResourcesBackup,
		"Misc.IgnoreNamespaces", CONFIG.Misc.IgnoreNamespaces,
		"Misc.LogRotationSizeInBytes", CONFIG.Misc.LogRotationSizeInBytes,
		"Misc.LogRotationMaxSizeInBytes", CONFIG.Misc.LogRotationMaxSizeInBytes,
		"Misc.LogRetentionDays", CONFIG.Misc.LogRetentionDays,
		"Misc.CheckForUpdates", CONFIG.Misc.CheckForUpdates,
		"Misc.HelmIndex", CONFIG.Misc.HelmIndex,
		"Misc.NfsPodPrefix", CONFIG.Misc.NfsPodPrefix,
		"Builder.BuildTimeout", CONFIG.Builder.BuildTimeout,
		"Builder.ScanTimeout", CONFIG.Builder.ScanTimeout,
		"Builder.MaxConcurrentBuilds", CONFIG.Builder.MaxConcurrentBuilds,
		"Git.GitUserEmail", CONFIG.Git.GitUserEmail,
		"Git.GitUserName", CONFIG.Git.GitUserName,
		"Stats.MaxDataPoints", CONFIG.Stats.MaxDataPoints,
	)
}

func PrintVersionInfo() {
	UtilsLogger.Info(
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
		UtilsLogger.Error("Error retrieving user homedir", "error", err)
	}

	if customConfigPath != "" {
		if _, err := os.Stat(configPath); err == nil || os.IsExist(err) {
			configPath = customConfigPath
			configDir = filepath.Dir(customConfigPath)
		} else {
			UtilsLogger.Error("Custom config not found.", "customConfigPath", customConfigPath)
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
		UtilsLogger.Error("failed to delete config file", "error", err)
	} else {
		UtilsLogger.Info("succesfully deleted config file", "configPath", configPath)
	}
}

func writeDefaultConfig(stage string) {
	configDir, configPath := GetDirectories("")

	// write it to default location
	err := os.Mkdir(configDir, 0755)
	if err != nil && err.Error() != "mkdir "+configDir+": file exists" {
		UtilsLogger.Warn("failed to create directory", "path", configDir, "error", err)
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
		UtilsLogger.Warn("No stage set. Using local config.")
		err = os.WriteFile(configPath, []byte(DefaultConfigLocalFile), 0755)
	}
	if err != nil {
		UtilsLogger.Error("Error writing "+configPath+" file", "configPath", configPath, "error", err)
		panic(1)
	}
}
