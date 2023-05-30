package utils

import (
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/version"
	"net/http"
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

// This object will initially created in secrets when the software is installed into the cluster for the first time (resource: secret -> mogenius/mogenius)
type ClusterSecret struct {
	ApiKey       string
	ClusterMfaId string
	ClusterName  string
}

type Config struct {
	Kubernetes struct {
		ApiKey                   string `yaml:"api_key" env:"api_key" env-description:"Api Key to access the server"`
		ClusterName              string `yaml:"cluster_name" env:"cluster_name" env-description:"The Name of the Kubernetes Cluster"`
		OwnNamespace             string `yaml:"own_namespace" env:"OWN_NAMESPACE" env-description:"The Namespace of mogenius platform"`
		ClusterMfaId             string `yaml:"cluster_mfa_id" env:"cluster_mfa_id" env-description:"UUID of the Kubernetes Cluster for MFA purpose"`
		RunInCluster             bool   `yaml:"run_in_cluster" env:"run_in_cluster" env-description:"If set to true, the application will run in the cluster (using the service account token). Otherwise it will try to load your local default context." env-default:"false"`
		DefaultContainerRegistry string `yaml:"default_container_registry" env:"default_container_registry" env-description:"Default Container Image Registry"`
	} `yaml:"kubernetes"`
	ApiServer struct {
		Ws_Server string `yaml:"ws_server" env:"api_ws_server" env-description:"Server host" env-default:"127.0.0.1:8080"`
		WS_Path   string `yaml:"ws_path" env:"api_ws_path" env-description:"Server Path" env-default:"/ws"`
	} `yaml:"api_server"`
	EventServer struct {
		Server string `yaml:"server" env:"event_server" env-description:"Server host" env-default:"127.0.0.1:8080"`
		Path   string `yaml:"path" env:"event_path" env-description:"Server Path" env-default:"/ws"`
	} `yaml:"event_server"`
	Misc struct {
		Stage                 string   `yaml:"stage" env:"stage" env-description:"mogenius k8s-manager stage" env-default:"prod"`
		Debug                 bool     `yaml:"debug" env:"debug" env-description:"If set to true, debug features will be enabled." env-default:"false"`
		LogKubernetesEvents   bool     `yaml:"log_kubernetes_events" env:"log-kubernetes-events" env-description:"If set to true, all kubernetes events will be logged to std-out." env-default:"false"`
		DefaultMountPath      string   `yaml:"default_mount_path" env:"default_mount_path" env-description:"All containers will have access to this mount point"`
		IgnoreNamespaces      []string `yaml:"ignore_namespaces" env:"ignore_namespaces" env-description:"List of all ignored namespaces." env-default:""`
		IgnoreResourcesBackup []string `yaml:"ignore_resources_backup" env:"ignore_resources_backup" env-description:"List of all ignored resources while backup." env-default:""`
		CheckForUpdates       int      `yaml:"check_for_updates" env:"check_for_updates" env-description:"Time interval between update checks." env-default:"86400"`
		HelmIndex             string   `yaml:"helm_index" env:"helm_index" env-description:"URL of the helm index file." env-default:"https://helm.mogenius.com/public/index.yaml"`
	} `yaml:"misc"`
}

var DefaultConfigLocalFile string
var DefaultConfigClusterFileDev string
var DefaultConfigClusterFileProd string
var CONFIG Config

func InitConfigYaml(showDebug bool, customConfigName *string, clusterSecret ClusterSecret, loadClusterConfig bool) {
	_, configPath := GetDirectories(customConfigName)

	if _, err := os.Stat(configPath); err == nil || os.IsExist(err) {
		// file exists
		if err := cleanenv.ReadConfig(configPath, &CONFIG); err != nil {
			if strings.HasPrefix(err.Error(), "config file parsing error:") {
				logger.Log.Notice("Config file is corrupted. Creating a new one by using -r flag.")
			}
			logger.Log.Fatal(err)
		}
	} else {
		WriteDefaultConfig(loadClusterConfig)

		// read configuration from the file and environment variables
		if err := cleanenv.ReadConfig(configPath, &CONFIG); err != nil {
			logger.Log.Fatal(err)
		}
	}

	// LOCAL ALWAYS WINS
	if clusterSecret.ClusterMfaId != "" && CONFIG.Kubernetes.RunInCluster {
		CONFIG.Kubernetes.ClusterMfaId = clusterSecret.ClusterMfaId
	}
	if clusterSecret.ApiKey != "" && CONFIG.Kubernetes.RunInCluster {
		CONFIG.Kubernetes.ApiKey = clusterSecret.ApiKey
	}
	if clusterSecret.ClusterName != "" && CONFIG.Kubernetes.RunInCluster {
		CONFIG.Kubernetes.ClusterName = clusterSecret.ClusterName
	}

	if showDebug {
		PrintSettings()
	}

	// CHECKS FOR CLUSTER
	if loadClusterConfig {
		if CONFIG.Kubernetes.ClusterName == "your-cluster-name" || CONFIG.Kubernetes.ClusterName == "" {
			if !showDebug {
				PrintSettings()
			}
			logger.Log.Fatalf("Environment Variable 'cluster_name' not setup. TERMINATING.")
		}
		if CONFIG.Kubernetes.ApiKey == "YOUR_API_KEY" || CONFIG.Kubernetes.ApiKey == "" {
			if !showDebug {
				PrintSettings()
			}
			logger.Log.Fatalf("Environment Variable 'api_key' not setup or default value not overwritten. TERMINATING.")
		}
	}

	if CONFIG.Misc.Debug {
		logger.Log.Warning("Starting serice for pprof in localhost:6060")
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}
}

func PrintSettings() {
	logger.Log.Infof("OwnNamespace:             %s", CONFIG.Kubernetes.OwnNamespace)
	logger.Log.Infof("ClusterName:              %s", CONFIG.Kubernetes.ClusterName)
	logger.Log.Infof("ClusterMfaId:             %s", CONFIG.Kubernetes.ClusterMfaId)
	logger.Log.Infof("RunInCluster:             %t", CONFIG.Kubernetes.RunInCluster)
	logger.Log.Infof("DefaultContainerRegistry: %s", CONFIG.Kubernetes.DefaultContainerRegistry)

	logger.Log.Infof("ApiKey:                   %s", CONFIG.Kubernetes.ApiKey)

	logger.Log.Infof("WsServer:                 %s", CONFIG.ApiServer.Ws_Server)
	logger.Log.Infof("WsPath:                   %s", CONFIG.ApiServer.WS_Path)

	logger.Log.Infof("EventServer:              %s", CONFIG.EventServer.Server)
	logger.Log.Infof("EventPath:                %s", CONFIG.EventServer.Path)

	logger.Log.Infof("Stage:                    %s", CONFIG.Misc.Stage)
	logger.Log.Infof("Debug:                    %t", CONFIG.Misc.Debug)
	logger.Log.Infof("LogKubernetesEvents:      %t", CONFIG.Misc.LogKubernetesEvents)
	logger.Log.Infof("DefaultMountPath:         %s", CONFIG.Misc.DefaultMountPath)
	logger.Log.Infof("IgnoreResourcesBackup:    %s", strings.Join(CONFIG.Misc.IgnoreResourcesBackup, ","))
	logger.Log.Infof("IgnoreNamespaces:         %s", strings.Join(CONFIG.Misc.IgnoreNamespaces, ","))
	logger.Log.Infof("CheckForUpdates:          %d", CONFIG.Misc.CheckForUpdates)
	logger.Log.Infof("HelmIndex:                %s", CONFIG.Misc.HelmIndex)
}

func PrintVersionInfo() {
	logger.Log.Infof("Version:     %s", version.Ver)
	logger.Log.Infof("Branch:      %s", version.Branch)
	logger.Log.Infof("Hash:        %s", version.GitCommitHash)
	logger.Log.Infof("BuildAt:     %s", version.BuildTimestamp)
}

func GetDirectories(customConfigName *string) (configDir string, configPath string) {
	homeDirName, err := os.UserHomeDir()
	if err != nil {
		logger.Log.Error(err)
	}

	configDir = homeDirName + "/.mogenius-k8s-manager/"
	if customConfigName != nil {
		newConfigName := *customConfigName
		if newConfigName != "" {
			configPath = configDir + newConfigName
		}
	} else {
		configPath = configDir + "config.yaml"
	}

	return configDir, configPath
}

func WriteDefaultConfig(loadClusterConfig bool) {
	configDir, configPath := GetDirectories(nil)

	// write it to default location
	err := os.Mkdir(configDir, 0755)
	if err != nil && err.Error() != "mkdir "+configDir+": file exists" {
		logger.Log.Warning("Error creating folder " + configDir)
		logger.Log.Warning(err)
	}

	stage := os.Getenv("STAGE")

	if loadClusterConfig {
		if stage == "prod" {
			err = os.WriteFile(configPath, []byte(DefaultConfigClusterFileProd), 0755)
		} else {
			err = os.WriteFile(configPath, []byte(DefaultConfigClusterFileDev), 0755)
		}
	} else {
		err = os.WriteFile(configPath, []byte(DefaultConfigLocalFile), 0755)
	}
	if err != nil {
		logger.Log.Error("Error writing " + configPath + " file")
		logger.Log.Fatal(err.Error())
	}
}
