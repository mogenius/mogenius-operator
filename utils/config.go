package utils

import (
	"mogenius-k8s-manager/logger"
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Kubernetes struct {
		ClusterName              string `yaml:"cluster_name" env-description:"The Name of the Kubernetes Cluster"`
		RunInCluster             bool   `yaml:"run_in_cluster" env-description:"If set to true, the application will run in the cluster (using the service account token). Otherwise it will try to load your local default context." env-default:"false"`
		DefaultContainerRegistry string `yaml:"default_container_registry" env-description:"Default Container Image Registry"`
	} `yaml:"kubernetes"`
	ApiServer struct {
		WebsocketServer string `yaml:"websocket_server" env-description:"Server host" env-default:"127.0.0.1"`
		WebsocketPort   int    `yaml:"websocket_port" env-description:"Server port" env-default:"8080"`
		WebsocketPath   string `yaml:"websocket_path" env-description:"Server Path" env-default:"/ws"`
		ApiKey          string `yaml:"api_key" env-description:"Api Key to access the server"`
	} `yaml:"api_server"`
	Misc struct {
		Debug                 bool     `yaml:"debug" env-description:"If set to true, debug features will be enabled." env-default:"false"`
		StorageAccount        string   `yaml:"storage_account" env-description:"Azure Storage Account"`
		DefaultMountPath      string   `yaml:"default_mount_path" env-description:"All containers will have access to this mount point"`
		ConcurrentConnections int      `yaml:"concurrent_connections" env-description:"Concurrent connections to API server." env-default:"3"`
		IgnoreNamespaces      []string `yaml:"ignore_namespaces" env-description:"List of all ignored namespaces." env-default:""`
		CheckForUpdates       int      `yaml:"check_for_updates" env-description:"Time interval between update checks." env-default:"3600"`
		HelmIndex             string   `yaml:"helm_index" env-description:"URL of the helm index file." env-default:"https://helm.mogenius.com/public/index.yaml"`
	} `yaml:"misc"`
}

var DefaultConfigFile string
var CONFIG Config

func InitConfigYaml(showDebug bool, customConfigName *string, overrideClusterName *string) {
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
		WriteDefaultConfig()

		// read configuration from the file and environment variables
		if err := cleanenv.ReadConfig(configPath, &CONFIG); err != nil {
			logger.Log.Fatal(err)
		}
	}

	if *overrideClusterName != "" {
		CONFIG.Kubernetes.ClusterName = *overrideClusterName
	}

	if showDebug {
		PrintSettings()
	}
}

func PrintSettings() {
	logger.Log.Infof("ClusterName: \t\t\t%s", CONFIG.Kubernetes.ClusterName)
	logger.Log.Infof("RunInCluster: \t\t\t%t", CONFIG.Kubernetes.RunInCluster)
	logger.Log.Infof("DefaultContainerRegistry: \t%s", CONFIG.Kubernetes.DefaultContainerRegistry)

	logger.Log.Infof("WebsocketServer:\t\t\t%s", CONFIG.ApiServer.WebsocketServer)
	logger.Log.Infof("WebsocketPort: \t\t\t%d", CONFIG.ApiServer.WebsocketPort)
	logger.Log.Infof("WebsocketPath: \t\t\t%s", CONFIG.ApiServer.WebsocketPath)
	logger.Log.Infof("ApiKey: \t\t\t\t%s", CONFIG.ApiServer.ApiKey)

	logger.Log.Infof("Debug: \t\t\t\t%t", CONFIG.Misc.Debug)
	logger.Log.Infof("StorageAccount: \t\t\t%s", CONFIG.Misc.StorageAccount)
	logger.Log.Infof("DefaultMountPath: \t\t%s", CONFIG.Misc.DefaultMountPath)
	logger.Log.Infof("ConcurrentConnections: \t\t%d", CONFIG.Misc.ConcurrentConnections)
	logger.Log.Infof("IgnoreNamespaces: \t\t%s", strings.Join(CONFIG.Misc.IgnoreNamespaces, ","))
	logger.Log.Infof("CheckForUpdates: \t\t%d", CONFIG.Misc.CheckForUpdates)
	logger.Log.Infof("HelmIndex: \t\t%d", CONFIG.Misc.HelmIndex)
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

func WriteDefaultConfig() {
	configDir, configPath := GetDirectories(nil)

	// write it to default location
	err := os.Mkdir(configDir, 0755)
	if err != nil && err.Error() != "mkdir "+configDir+": file exists" {
		logger.Log.Warning("Error creating folder " + configDir)
		logger.Log.Warning(err)
	}

	err = os.WriteFile(configPath, []byte(DefaultConfigFile), 0755)
	if err != nil {
		logger.Log.Error("Error writing " + configPath + " file")
		logger.Log.Fatal(err.Error())
	}
}
