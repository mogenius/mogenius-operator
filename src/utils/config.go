package utils

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

const HELM_INDEX string = "https://helm.mogenius.com/public/index.yaml"
const NFS_POD_PREFIX string = "nfs-server"

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
}

var CONFIG Config
var ConfigPath string
var ClusterProviderCached KubernetesProvider = UNKNOWN

// preconfigure with dtos
var IacWorkloadConfigMap map[string]bool

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
