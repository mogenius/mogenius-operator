package utils

import (
	_ "embed"
	"fmt"
	"io"
	"mogenius-k8s-manager/version"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	punqDtos "github.com/mogenius/punq/dtos"
	"gopkg.in/yaml.v2"

	"github.com/ilyakaznacheev/cleanenv"
	log "github.com/sirupsen/logrus"
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
	SyncWorkloads      []string
	IgnoredNamespaces  []string
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
		BboltDbPath                string `yaml:"bbolt_db_path" env:"bbolt_db_path" env-description:"Path to the bbolt database. This db stores build-related information."`
		BboltDbStatsPath           string `yaml:"bbolt_db_stats_path" env:"bbolt_db_stats_path" env-description:"Path to the bbolt database. This db stores stats-related information."`
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
		RepoUrl            string   `yaml:"repo_url" env:"sync_repo_url" env-description:"Sync repo url." env-default:""`
		RepoPat            string   `yaml:"repo_pat" env:"sync_repo_pat" env-description:"Sync repo pat." env-default:""`
		RepoBranch         string   `yaml:"repo_pat_branch" env:"sync_repo_branch" env-description:"Sync repo branch." env-default:"main"`
		SyncFrequencyInSec int      `yaml:"sync_requency_secs" env:"sync_requency_secs" env-description:"Polling interval for sync in seconds." env-default:"10"`
		AllowPush          bool     `yaml:"allow_push" env:"sync_allow_push" env-description:"Allow IAC manager to push data to repo." env-default:"true"`
		AllowPull          bool     `yaml:"allow_pull" env:"sync_allow_pull" env-description:"Allow IAC manager to pull data from repo." env-default:"true"`
		SyncWorkloads      []string `yaml:"sync_workloads" env:"sync_workloads" env-description:"List of all workloads to sync." env-default:""`
		ShowDiffInLog      bool     `yaml:"show_diff_in_log" env:"sync_show_diff_in_log" env-description:"Show all changes of resources as diff in operator log."`
		IgnoredNamespaces  []string `yaml:"ignored_namespaces" env:"sync_ignored_namespaces" env-description:"List of all ignored namespaces." env-default:""`
		LogChanges         bool     `yaml:"log_changes" env:"sync_log_changes" env-description:"Resource changes in kubernetes will create a log entry." env-default:"true"`
	} `yaml:"iac"`
	Misc struct {
		Stage                  string   `yaml:"stage" env:"stage" env-description:"mogenius k8s-manager stage" env-default:"prod"`
		LogFormat              string   `yaml:"log_format" env:"log_format" env-description:"Setup the log format. Available are: json | text" env-default:"json"`
		LogLevel               string   `yaml:"log_level" env:"log_level" env-description:"Setup the log level. Available are: panic, fatal, error, warn, info, debug, trace" env-default:"info"`
		LogIncomingStats       bool     `yaml:"log_incoming_stats" env:"log_incoming_stats" env-description:"Scraper data input will be logged visibly when set to true." env-default:"false"`
		Debug                  bool     `yaml:"debug" env:"debug" env-description:"If set to true, debug features will be enabled." env-default:"false"`
		DebugLogCaller         bool     `yaml:"debug_log_caller" env:"debug_log_caller" env-description:"If set to true, the calling function will be logged." env-default:"false"`
		LogKubernetesEvents    bool     `yaml:"log_kubernetes_events" env:"log_kubernetes_events" env-description:"If set to true, all kubernetes events will be logged to std-out." env-default:"false"`
		DefaultMountPath       string   `yaml:"default_mount_path" env:"default_mount_path" env-description:"All containers will have access to this mount point"`
		IgnoreNamespaces       []string `yaml:"ignore_namespaces" env:"ignore_namespaces" env-description:"List of all ignored namespaces." env-default:""`
		LogRotationSizeInBytes int      `yaml:"log_rotation_size_in_bytes" env:"log_rotation_size_in_bytes" env-description:"Size of the logfile when it is rotated." env-default:"5242880"`
		LogRetentionDays       int      `yaml:"log_retention_days" env:"log_retention_days" env-description:"Number of days to keep log files." env-default:"7"`
		AutoMountNfs           bool     `yaml:"auto_mount_nfs" env:"auto_mount_nfs" env-description:"If set to true, nfs pvc will automatically be mounted." env-default:"true"`
		IgnoreResourcesBackup  []string `yaml:"ignore_resources_backup" env:"ignore_resources_backup" env-description:"List of all ignored resources while backup." env-default:""`
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
		GitUserEmail      string `yaml:"git_user_email" env:"git_user_email" env-description:"Email address which is used when interacting with git." env-default:"git@mogenius.com"`
		GitUserName       string `yaml:"git_user_name" env:"git_user_name" env-description:"User name which is used when interacting with git." env-default:"mogenius git-user"`
		GitDefaultBranch  string `yaml:"git_default_branch" env:"git_default_branch" env-description:"Default branch name which is used when creating a repository." env-default:"main"`
		GitAddIgnoredFile string `yaml:"git_add_ignored_file" env:"git_add_ignored_file" env-description:"Gits behaviour when adding ignored files." env-default:"false"`
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
			log.Error("Config file is corrupted. Creating a new one by using -r flag.")
		}
		log.Errorf("Error reading config: %s", ConfigPath)
		log.Errorf("Error reading config: %s", err.Error())
	}

	if CONFIG.Kubernetes.RunInCluster {
		ConfigPath = "RUNS_IN_CLUSTER_NO_CONFIG_NEEDED"
	}

	// SET DEFAULTS if missing
	pwd, _ := os.Getwd()
	if CONFIG.Kubernetes.BboltDbPath == "" {
		CONFIG.Kubernetes.BboltDbPath = pwd + "/mogenius.db"
	}
	if CONFIG.Kubernetes.BboltDbStatsPath == "" {
		CONFIG.Kubernetes.BboltDbStatsPath = pwd + "/mogenius-stats.db"
	}
	if CONFIG.Misc.DefaultMountPath == "" {
		CONFIG.Misc.DefaultMountPath = pwd
	}

	// CHECKS FOR CLUSTER
	if CONFIG.Kubernetes.RunInCluster {
		if CONFIG.Kubernetes.ClusterName == "your-cluster-name" || CONFIG.Kubernetes.ClusterName == "" {
			if !showDebug {
				PrintSettings()
			}
			log.Fatalf("Environment Variable 'cluster_name' not setup. TERMINATING.")
		}
		if CONFIG.Kubernetes.ApiKey == "YOUR_API_KEY" || CONFIG.Kubernetes.ApiKey == "" {
			if !showDebug {
				PrintSettings()
			}
			log.Fatalf("Environment Variable 'api_key' not setup or default value not overwritten. TERMINATING.")
		}
	}

	if showDebug || CONFIG.Kubernetes.RunInCluster {
		PrintSettings()
	}

	if CONFIGVERSION > CONFIG.Config.Version {
		log.Fatalf("Config version is outdated. Please delete your config file %s and restart the application. (Your Config version: %d, Needed: %d)", ConfigPath, CONFIG.Config.Version, CONFIGVERSION)
	}

	// SET LOGGING
	setupLogging()

	if CONFIG.Misc.Debug {
		log.Info("Starting serice for pprof in localhost:6060")
		go func() {
			log.Info(http.ListenAndServe("localhost:6060", nil))
			log.Info("1. Portforward mogenius-k8s-manager to 6060")
			log.Info("2. wget http://localhost:6060/debug/pprof/profile?seconds=60 -O cpu.pprof")
			log.Info("3. wget http://localhost:6060/debug/pprof/heap -O mem.pprof")
			log.Info("4. go tool pprof -http=localhost:8081 cpu.pprof")
			log.Info("5. go tool pprof -http=localhost:8081 mem.pprof")
			log.Info("OR: go tool pprof mem.pprof -> Then type in commands like top, top --cum, list")
			log.Info("http://localhost:6060/debug/pprof/ This is the index page that lists all available profiles.")
			log.Info("http://localhost:6060/debug/pprof/profile This serves a CPU profile. You can set the profiling duration through the seconds parameter. For example, ?seconds=30 would profile your CPU for 30 seconds.")
			log.Info("http://localhost:6060/debug/pprof/heap This serves a snapshot of the current heap memory usage.")
			log.Info("http://localhost:6060/debug/pprof/goroutine This serves a snapshot of the current goroutines stack traces.")
			log.Info("http://localhost:6060/debug/pprof/block This serves a snapshot of stack traces that led to blocking on synchronization primitives.")
			log.Info("http://localhost:6060/debug/pprof/threadcreate This serves a snapshot of all OS thread creation stack traces.")
			log.Info("http://localhost:6060/debug/pprof/cmdline This returns the command line invocation of the current program.")
			log.Info("http://localhost:6060/debug/pprof/symbol This is used to look up the program counters listed in a pprof profile.")
			log.Info("http://localhost:6060/debug/pprof/trace This serves a trace of execution of the current program. You can set the trace duration through the seconds parameter.")
		}()
	}
}

func setupLogging() {
	// Create a log file
	err := os.MkdirAll(MainLogFolder(), os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create parent directories: %v", err)
	}
	file, err := os.OpenFile(MainLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	mw := io.MultiWriter(os.Stdout, file)

	log.SetOutput(mw)
	log.SetLevel(log.TraceLevel)

	log.AddHook(&SecretRedactionHook{})
	log.AddHook(&LogRotationHook{})

	log.SetFormatter(&log.TextFormatter{
		ForceColors:      true,
		DisableTimestamp: false,
		DisableQuote:     true,
	})

	log.SetReportCaller(CONFIG.Misc.DebugLogCaller)
	logLevel, err := log.ParseLevel(CONFIG.Misc.LogLevel)
	if err != nil {
		logLevel = log.InfoLevel
		log.Error("Error parsing log level. Using default log level: info")
	}
	log.SetLevel(logLevel)

	if strings.ToLower(CONFIG.Misc.LogFormat) == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else if strings.ToLower(CONFIG.Misc.LogFormat) == "text" {
		log.SetFormatter(&log.TextFormatter{
			ForceColors:      true,
			DisableTimestamp: false,
			DisableQuote:     true,
		})
	} else {
		log.SetFormatter(&log.TextFormatter{})
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
	configCopy.Kubernetes.BboltDbPath = ""
	configCopy.Kubernetes.BboltDbStatsPath = ""
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
		CONFIG.Iac.SyncWorkloads = clusterSecret.SyncWorkloads
		CONFIG.Iac.IgnoredNamespaces = clusterSecret.IgnoredNamespaces

		if clusterSecret.SyncFrequencyInSec <= 5 {
			clusterSecret.SyncFrequencyInSec = 5
		} else {
			CONFIG.Iac.SyncFrequencyInSec = clusterSecret.SyncFrequencyInSec
		}
	}
}

func PrintSettings() {
	log.Infof("Config\n")
	log.Infof("Version:                   %d\n\n", CONFIG.Config.Version)

	log.Infof("KUBERNETES")
	log.Infof("OwnNamespace:              %s", CONFIG.Kubernetes.OwnNamespace)
	log.Infof("ClusterName:               %s", CONFIG.Kubernetes.ClusterName)
	log.Infof("ClusterMfaId:              %s", CONFIG.Kubernetes.ClusterMfaId)
	log.Infof("RunInCluster:              %t", CONFIG.Kubernetes.RunInCluster)
	log.Infof("ApiKey:                    %s", CONFIG.Kubernetes.ApiKey)
	log.Infof("BboltDbPath:               %s", CONFIG.Kubernetes.BboltDbPath)
	log.Infof("BboltDbStatsPath:          %s", CONFIG.Kubernetes.BboltDbStatsPath)
	log.Infof("LocalContainerRegistry:    %s\n\n", CONFIG.Kubernetes.LocalContainerRegistryHost)

	log.Infof("API")
	log.Infof("HttpServer:                %s", CONFIG.ApiServer.Http_Server)
	log.Infof("WsServer:                  %s", CONFIG.ApiServer.Ws_Server)
	log.Infof("WsScheme:                  %s", CONFIG.ApiServer.Ws_Scheme)
	log.Infof("WsPath:                    %s\n\n", CONFIG.ApiServer.WS_Path)

	log.Infof("EVENTS")
	log.Infof("EventServer:               %s", CONFIG.EventServer.Server)
	log.Infof("EventScheme:               %s", CONFIG.EventServer.Scheme)
	log.Infof("EventPath:                 %s\n\n", CONFIG.EventServer.Path)

	log.Infof("IAC")
	log.Infof("RepoUrl:                   %s", CONFIG.Iac.RepoUrl)
	log.Infof("RepoPat:                   %s", CONFIG.Iac.RepoPat)
	log.Infof("RepoBranch:                %s", CONFIG.Iac.RepoBranch)
	log.Infof("PollingIntervalSecs:       %d", CONFIG.Iac.SyncFrequencyInSec)
	log.Infof("AllowPull:                 %t", CONFIG.Iac.AllowPull)
	log.Infof("AllowPush:                 %t", CONFIG.Iac.AllowPush)
	log.Infof("SyncWorkloads:             %s", strings.Join(CONFIG.Iac.SyncWorkloads, ","))
	log.Infof("IgnoredNamespaces:         %s", strings.Join(CONFIG.Iac.IgnoredNamespaces, ","))
	log.Infof("LogChanges:                %t", CONFIG.Iac.LogChanges)
	log.Infof("ShowDiffInLog:             %t\n\n", CONFIG.Iac.ShowDiffInLog)

	log.Infof("MISC")
	log.Infof("Stage:                     %s", CONFIG.Misc.Stage)
	log.Infof("LogFormat:                 %s", CONFIG.Misc.LogFormat)
	log.Infof("LogIncomingStats:          %t", CONFIG.Misc.LogIncomingStats)
	log.Infof("Debug:                     %t", CONFIG.Misc.Debug)
	log.Infof("DebugLogCaller:            %t", CONFIG.Misc.DebugLogCaller)
	log.Infof("AutoMountNfs:              %t", CONFIG.Misc.AutoMountNfs)
	log.Infof("LogKubernetesEvents:       %t", CONFIG.Misc.LogKubernetesEvents)
	log.Infof("DefaultMountPath:          %s", CONFIG.Misc.DefaultMountPath)
	log.Infof("IgnoreResourcesBackup:     %s", strings.Join(CONFIG.Misc.IgnoreResourcesBackup, ","))
	log.Infof("IgnoreNamespaces:          %s", strings.Join(CONFIG.Misc.IgnoreNamespaces, ","))
	log.Infof("LogRotationSizeInBytes:    %d", CONFIG.Misc.LogRotationSizeInBytes)
	log.Infof("LogRetentionDays:          %d", CONFIG.Misc.LogRetentionDays)
	log.Infof("CheckForUpdates:           %d", CONFIG.Misc.CheckForUpdates)
	log.Infof("HelmIndex:                 %s", CONFIG.Misc.HelmIndex)
	log.Infof("NfsPodPrefix:              %s\n\n", CONFIG.Misc.NfsPodPrefix)

	log.Infof("BUILDER")
	log.Infof("BuildTimeout:              %d", CONFIG.Builder.BuildTimeout)
	log.Infof("ScanTimeout:               %d", CONFIG.Builder.ScanTimeout)
	log.Infof("MaxConcurrentBuilds:       %d\n\n", CONFIG.Builder.MaxConcurrentBuilds)

	log.Infof("GIT")
	log.Infof("GitUserEmail:              %s", CONFIG.Git.GitUserEmail)
	log.Infof("GitUserName:               %s", CONFIG.Git.GitUserName)
	log.Infof("GitDefaultBranch:          %s", CONFIG.Git.GitDefaultBranch)
	log.Infof("GitAddIgnoredFile:         %s\n\n", CONFIG.Git.GitAddIgnoredFile)

	log.Infof("STATS")
	log.Infof("MaxDataPoints:             %d\n\n", CONFIG.Stats.MaxDataPoints)

	log.Infof("Config:                    %s\n\n", ConfigPath)
}

func PrintVersionInfo() {
	log.Infof("\nVersion:     %s", version.Ver)
	log.Infof("Branch:      %s", version.Branch)
	log.Infof("Hash:        %s", version.GitCommitHash)
	log.Infof("BuildAt:     %s", version.BuildTimestamp)
}

func GetDirectories(customConfigPath string) (configDir string, configPath string) {
	homeDirName, err := os.UserHomeDir()
	if err != nil {
		log.Error(err)
	}

	if customConfigPath != "" {
		if _, err := os.Stat(configPath); err == nil || os.IsExist(err) {
			configPath = customConfigPath
			configDir = filepath.Dir(customConfigPath)
		} else {
			log.Errorf("Custom config not found '%s'.", customConfigPath)
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
		log.Errorf("Error removing config file. '%s'.", err.Error())
	} else {
		log.Infof("%s succesfuly deleted.", configPath)
	}
}

func writeDefaultConfig(stage string) {
	configDir, configPath := GetDirectories("")

	// write it to default location
	err := os.Mkdir(configDir, 0755)
	if err != nil && err.Error() != "mkdir "+configDir+": file exists" {
		log.Warning("Error creating folder " + configDir)
		log.Warning(err)
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
		log.Warnf("No stage set. Using local config.")
		err = os.WriteFile(configPath, []byte(DefaultConfigLocalFile), 0755)
	}
	if err != nil {
		log.Error("Error writing " + configPath + " file")
		log.Fatal(err.Error())
	}
}
