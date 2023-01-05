package utils

import (
	"io/ioutil"
	"mogenius-k8s-manager/logger"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Kubernetes struct {
		ClusterName  string `yaml:"cluster_name" env-description:"The Name of the Kubernetes Cluster"`
		RunInCluster bool   `yaml:"run_in_cluster" env-description:"If set to true, the application will run in the cluster (using the service account token). Otherwise it will try to load your local default context." env-default:"false"`
	} `yaml:"kubernetes"`
	ApiServer struct {
		WebsocketServer string `yaml:"websocket_server" env-description:"Server host" env-default:"127.0.0.1"`
		WebsocketPort   int    `yaml:"websocket_port" env-description:"Server port" env-default:"8080"`
		ApiKey          string `yaml:"api_key" env-description:"Api Key to access the server"`
	} `yaml:"api_server"`
}

var DefaultConfigFile string
var CONFIG Config

func InitConfigYaml() {

	homeDirName, homeDirErr := os.UserHomeDir()
	if homeDirErr != nil {
		logger.Log.Error(homeDirErr)
	}

	configDir := homeDirName + "/.mogenius-k8s-manager/"
	configPath := configDir + "config.yaml"

	if _, err := os.Stat(configPath); err == nil || os.IsExist(err) {
		// file exists
		if err := cleanenv.ReadConfig(configPath, &CONFIG); err != nil {
			logger.Log.Fatal(err)
			os.Exit(2)
		}
	} else {
		// write it to default location
		folderErr := os.Mkdir(configDir, 0644)
		if folderErr != nil && folderErr.Error() != "mkdir "+configDir+": file exists" {
			logger.Log.Warning("Error creating folder " + configDir)
			logger.Log.Warning(folderErr)
		}

		errWrite := ioutil.WriteFile(configPath, []byte(DefaultConfigFile), 0644)
		if errWrite != nil {
			logger.Log.Fatal("Error writing " + configPath + " file")
			logger.Log.Fatal(err)
		}

		// read configuration from the file and environment variables
		if err := cleanenv.ReadConfig(configPath, &CONFIG); err != nil {
			logger.Log.Fatal(err)
			os.Exit(2)
		}
	}

	logger.Log.Infof("ClusterName: \t%s", CONFIG.Kubernetes.ClusterName)
	logger.Log.Infof("RunInCluster: \t%t", CONFIG.Kubernetes.RunInCluster)
	logger.Log.Infof("WebsocketServer: %s", CONFIG.ApiServer.WebsocketServer)
	logger.Log.Infof("WebsocketPort: \t%d", CONFIG.ApiServer.WebsocketPort)
	logger.Log.Infof("ApiKey: \t\t%s", CONFIG.ApiServer.ApiKey)
}
