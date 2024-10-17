package services

import (
	"context"
	"fmt"
	"io"
	"mogenius-k8s-manager/controllers"
	"mogenius-k8s-manager/db"
	dbstats "mogenius-k8s-manager/db-stats"
	"mogenius-k8s-manager/dtos"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/xterm"
	"os"
	"os/exec"
	"strings"
	"time"

	"mogenius-k8s-manager/structs"
	"net/url"

	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"
	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

var ServiceLogger = log.WithField("component", structs.ComponentServices)

type MessageResponseStatus string

const (
	StatusSuccess MessageResponseStatus = "success"
	StatusError   MessageResponseStatus = "error"
)

type MessageResponse struct {
	Status  MessageResponseStatus `json:"status"` // success, error
	Message string                `json:"message,omitempty"`
	Data    interface{}           `json:"data,omitempty"`
}

func NewMessageResponse(result interface{}, err error) MessageResponse {
	if err != nil {
		return MessageResponse{
			Status:  StatusError,
			Message: err.Error(),
		}
	}
	if str, ok := result.(string); ok {
		return MessageResponse{
			Status:  StatusSuccess,
			Message: str,
		}
	}
	return MessageResponse{
		Status: StatusSuccess,
		Data:   result,
	}
}

func ExecuteCommandRequest(datagram structs.Datagram) interface{} {
	switch datagram.Pattern {
	case structs.PAT_K8SNOTIFICATION:
		return K8sNotification(datagram)
	case structs.PAT_CLUSTERSTATUS:
		return punq.ClusterStatus(nil)
	case structs.PAT_CLUSTERRESOURCEINFO:
		nodeStats := punq.GetNodeStats(nil)
		loadBalancerExternalIps := punq.GetClusterExternalIps(nil)
		country, _ := punqUtils.GuessClusterCountry()
		result := punqDtos.ClusterResourceInfoDto{
			NodeStats:               nodeStats,
			LoadBalancerExternalIps: loadBalancerExternalIps,
			Country:                 country,
			Provider:                string(utils.ClusterProviderCached),
		}
		return result
	case structs.PAT_UPGRADEK8SMANAGER:
		data := K8sManagerUpgradeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return UpgradeK8sManager(data)

	case structs.PAT_CLUSTER_FORCE_RECONNECT:
		time.Sleep(1 * time.Second)
		return kubernetes.ClusterForceReconnect()

	case structs.PAT_CLUSTER_FORCE_DISCONNECT:
		time.Sleep(1 * time.Second)
		return kubernetes.ClusterForceDisconnect()

	case structs.PAT_SYSTEM_CHECK:
		return SystemCheck()
	case structs.PAT_CLUSTER_RESTART:
		ServiceLogger.Infof("😵😵😵 Received RESTART COMMAND. Restarting now ...")
		time.Sleep(1 * time.Second)
		os.Exit(0)
		return nil
	case structs.PAT_SYSTEM_PRINT_CURRENT_CONFIG:
		conf, err := utils.PrintCurrentCONFIG()
		if err != nil {
			return err
		}
		return conf

	case structs.PAT_IAC_FORCE_SYNC:
		return NewMessageResponse(nil, iacmanager.SyncChanges())
	case structs.PAT_IAC_GET_STATUS:
		return NewMessageResponse(iacmanager.GetDataModel(), nil)
	case structs.PAT_IAC_RESET_LOCAL_REPO:
		return NewMessageResponse(nil, kubernetes.ResetLocalRepo())
	case structs.PAT_IAC_RESET_FILE:
		data := dtos.ResetFileRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(nil, iacmanager.ResetFile(data.FilePath, data.CommitHash))

	case structs.PAT_ENERGY_CONSUMPTION:
		return EnergyConsumption()

	case structs.PAT_CLUSTER_SYNC_INFO:
		result, err := kubernetes.GetSyncRepoData()
		if err != nil {
			return err
		}
		return result

	case structs.PAT_CLUSTER_SYNC_UPDATE:
		data := dtos.SyncRepoData{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.AddSecretsToRedaction()
		err := kubernetes.UpdateSynRepoData(&data)
		if err != nil {
			return err
		}
		err = iacmanager.CheckRepoAccess()
		if err != nil {
			return err
		}
		return err

	case structs.PAT_INSTALL_LOCAL_DEV_COMPONENTS:
		data := ClusterIssuerInstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.AddSecretsToRedaction()
		return InstallAllLocalDevComponents(data.Email)
	case structs.PAT_INSTALL_TRAFFIC_COLLECTOR:
		return InstallTrafficCollector()
	case structs.PAT_INSTALL_POD_STATS_COLLECTOR:
		return InstallPodStatsCollector()
	case structs.PAT_INSTALL_METRICS_SERVER:
		return InstallMetricsServer()
	case structs.PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK:
		return InstallIngressControllerTreafik()
	case structs.PAT_INSTALL_CERT_MANAGER:
		return InstallCertManager()
	case structs.PAT_INSTALL_CLUSTER_ISSUER:
		data := ClusterIssuerInstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.AddSecretsToRedaction()
		return InstallClusterIssuer(data.Email, 0)
	case structs.PAT_INSTALL_CONTAINER_REGISTRY:
		return InstallContainerRegistry()
	case structs.PAT_INSTALL_EXTERNAL_SECRETS:
		return InstallExternalSecrets()
	case structs.PAT_INSTALL_METALLB:
		return InstallMetalLb()
	case structs.PAT_INSTALL_KEPLER:
		return InstallKepler()
	case structs.PAT_UNINSTALL_TRAFFIC_COLLECTOR:
		return UninstallTrafficCollector()
	case structs.PAT_UNINSTALL_POD_STATS_COLLECTOR:
		return UninstallPodStatsCollector()
	case structs.PAT_UNINSTALL_METRICS_SERVER:
		return UninstallMetricsServer()
	case structs.PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK:
		return UninstallIngressControllerTreafik()
	case structs.PAT_UNINSTALL_CERT_MANAGER:
		return UninstallCertManager()
	case structs.PAT_UNINSTALL_CLUSTER_ISSUER:
		return UninstallClusterIssuer()
	case structs.PAT_UNINSTALL_CONTAINER_REGISTRY:
		return UninstallContainerRegistry()
	case structs.PAT_UNINSTALL_EXTERNAL_SECRETS:
		return UninstallExternalSecrets()
	case structs.PAT_UNINSTALL_METALLB:
		return UninstallMetalLb()
	case structs.PAT_UNINSTALL_KEPLER:
		return UninstallKepler()
	case structs.PAT_UPGRADE_TRAFFIC_COLLECTOR:
		return UpgradeTrafficCollector()
	case structs.PAT_UPGRADE_PODSTATS_COLLECTOR:
		return UpgradePodStatsCollector()
	case structs.PAT_UPGRADE_METRICS_SERVER:
		return UpgradeMetricsServer()
	case structs.PAT_UPGRADE_INGRESS_CONTROLLER_TREAFIK:
		return UpgradeIngressControllerTreafik()
	case structs.PAT_UPGRADE_CERT_MANAGER:
		return UpgradeCertManager()
	case structs.PAT_UPGRADE_CONTAINER_REGISTRY:
		return UpgradeContainerRegistry()
	case structs.PAT_UPGRADE_METALLB:
		return UpgradeMetalLb()
	case structs.PAT_UPGRADE_KEPLER:
		return UpgradeKepler()

	case structs.PAT_STATS_PODSTAT_FOR_POD_ALL:
		data := StatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
		if ctrl == nil {
			return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
		}
		return dbstats.GetPodStatsEntriesForController(*ctrl)
	case structs.PAT_STATS_PODSTAT_FOR_POD_LAST:
		data := StatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
		if ctrl == nil {
			return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
		}
		return dbstats.GetLastPodStatsEntryForController(*ctrl)

	case structs.PAT_STATS_PODSTAT_FOR_CONTROLLER_ALL:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return dbstats.GetPodStatsEntriesForController(data)
	case structs.PAT_STATS_PODSTAT_FOR_CONTROLLER_LAST:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return dbstats.GetLastPodStatsEntryForController(data)
	case structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_ALL:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return dbstats.GetTrafficStatsEntriesForController(data)
	case structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_SUM, structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_LAST:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return dbstats.GetTrafficStatsEntrySumForController(data, false)

	case structs.PAT_STATS_TRAFFIC_FOR_POD_ALL:
		data := StatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
		if ctrl == nil {
			return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
		}
		return dbstats.GetTrafficStatsEntriesForController(*ctrl)
	case structs.PAT_STATS_TRAFFIC_FOR_POD_SUM, structs.PAT_STATS_TRAFFIC_FOR_POD_LAST:
		data := StatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		ctrl := kubernetes.ControllerForPod(data.Namespace, data.PodName)
		if ctrl == nil {
			return fmt.Errorf("could not find controller for pod %s in namespace %s", data.PodName, data.Namespace)
		}
		return dbstats.GetTrafficStatsEntrySumForController(*ctrl, false)

	case structs.PAT_STATS_PODSTAT_FOR_NAMESPACE_ALL:
		data := NsStatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return dbstats.GetPodStatsEntriesForNamespace(data.Namespace)
	case structs.PAT_STATS_PODSTAT_FOR_NAMESPACE_LAST:
		data := NsStatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return dbstats.GetLastPodStatsEntriesForNamespace(data.Namespace)
	case structs.PAT_STATS_TRAFFIC_FOR_NAMESPACE_ALL:
		data := NsStatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return dbstats.GetTrafficStatsEntriesForNamespace(data.Namespace)
	case structs.PAT_STATS_TRAFFIC_FOR_NAMESPACE_SUM, structs.PAT_STATS_TRAFFIC_FOR_NAMESPACE_LAST:
		data := NsStatsDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return dbstats.GetTrafficStatsEntriesSumForNamespace(data.Namespace)

	case structs.PAT_STATS_CHART_FOR_POD:
		data := ChartPodDataRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return RenderPodNetworkTreePageJson(data.Namespace, data.PodName)

	case structs.PAT_METRICS_DEPLOYMENT_AVG_UTILIZATION:
		data := kubernetes.K8sController{}
		data.Kind = "Deployment"

		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetAverageUtilizationForDeployment(data)
	case structs.PAT_FILES_LIST:
		data := FilesListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return List(data)
	case structs.PAT_FILES_CREATE_FOLDER:
		data := FilesCreateFolderRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return CreateFolder(data)
	case structs.PAT_FILES_RENAME:
		data := FilesRenameRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return Rename(data)
	case structs.PAT_FILES_CHOWN:
		data := FilesChownRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return Chown(data)
	case structs.PAT_FILES_CHMOD:
		data := FilesChmodRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return Chmod(data)
	case structs.PAT_FILES_DELETE:
		data := FilesDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return Delete(data)
	case structs.PAT_FILES_DOWNLOAD:
		data := FilesDownloadRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return Download(data)
	case structs.PAT_FILES_INFO:
		data := dtos.PersistentFileRequestDto{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return Info(data)

	case structs.PAT_CLUSTER_EXECUTE_HELM_CHART_TASK:
		data := ClusterHelmRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return InstallHelmChart(data)
	case structs.PAT_CLUSTER_UNINSTALL_HELM_CHART:
		data := ClusterHelmUninstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return DeleteHelmChart(data)
	case structs.PAT_CLUSTER_TCP_UDP_CONFIGURATION:
		return TcpUdpClusterConfiguration()
	case structs.PAT_CLUSTER_BACKUP:
		result, err := kubernetes.BackupNamespace("")
		if err != nil {
			return err.Error()
		}
		return result
	case structs.PAT_CLUSTER_READ_CONFIGMAP:
		data := ClusterGetConfigMap{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetConfigMapWR(data.Namespace, data.Name)
	case structs.PAT_CLUSTER_WRITE_CONFIGMAP:
		data := ClusterWriteConfigMap{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.WriteConfigMap(data.Namespace, data.Name, data.Data, data.Labels)
	case structs.PAT_CLUSTER_LIST_CONFIGMAPS:
		data := ClusterListWorkloads{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.ListConfigMapWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
	case structs.PAT_CLUSTER_READ_DEPLOYMENT:
		data := ClusterGetDeployment{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetDeploymentResult(data.Namespace, data.Name)
	// TODO
	// case structs.PAT_CLUSTER_WRITE_DEPLOYMENT:
	// 	data := ClusterWriteDeployment{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	return kubernetes.WriteConfigMap(data.Namespace, data.Name, data.Data, data.Labels)
	case structs.PAT_CLUSTER_LIST_DEPLOYMENTS:
		data := ClusterListWorkloads{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.ListDeploymentsWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)
	case structs.PAT_CLUSTER_READ_PERSISTENT_VOLUME_CLAIM:
		data := ClusterGetPersistentVolume{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.GetPersistentVolumeClaim(data.Namespace, data.Name)
	// TODO
	// case structs.PAT_CLUSTER_WRITE_PERSISTENT_VOLUME_CLAIM:
	// 	data := ClusterWritePersistentVolume{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	return kubernetes.WritePersistentVolume(data.Namespace, data.Name, data.Data, data.Labels)
	case structs.PAT_CLUSTER_LIST_PERSISTENT_VOLUME_CLAIMS:
		data := ClusterListWorkloads{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		// AllPersistentVolumes
		return kubernetes.ListPersistentVolumeClaimsWithFieldSelector(data.Namespace, data.LabelSelector, data.Prefix)

	case structs.PAT_CLUSTER_UPDATE_LOCAL_TLS_SECRET:
		data := ClusterUpdateLocalTlsSecret{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.CreateMogeniusContainerRegistryTlsSecret(data.LocalTlsCrt, data.LocalTlsKey)

	case structs.PAT_PROJECT_CREATE:
		data := ProjectCreateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return CreateProject(data)
	case structs.PAT_PROJECT_UPDATE:
		data := ProjectUpdateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return UpdateProject(data)
	case structs.PAT_PROJECT_DELETE:
		data := ProjectDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return DeleteProject(data)
	case structs.PAT_PROJECT_LIST:
		return ListProject()
	case structs.PAT_PROJECT_COUNT:
		return CountProject()
	case structs.PAT_NAMESPACE_CREATE:
		data := NamespaceCreateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Project.AddSecretsToRedaction()
		return CreateNamespace(data)
	case structs.PAT_NAMESPACE_DELETE:
		data := NamespaceDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return DeleteNamespace(data)
	case structs.PAT_NAMESPACE_SHUTDOWN:
		data := NamespaceShutdownRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		return ShutdownNamespace(data)
	case structs.PAT_NAMESPACE_POD_IDS:
		data := NamespacePodIdsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return PodIds(data)
	case structs.PAT_NAMESPACE_VALIDATE_CLUSTER_PODS:
		data := NamespaceValidateClusterPodsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return ValidateClusterPods(data)
	case structs.PAT_NAMESPACE_VALIDATE_PORTS:
		data := NamespaceValidatePortsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return ValidateClusterPorts(data)
	case structs.PAT_NAMESPACE_LIST_ALL:
		return ListAllNamespaces()
	case structs.PAT_NAMESPACE_GATHER_ALL_RESOURCES:
		data := NamespaceGatherAllResourcesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return ListAllResourcesForNamespace(data)
	case structs.PAT_NAMESPACE_BACKUP:
		data := NamespaceBackupRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		result, err := kubernetes.BackupNamespace(data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case structs.PAT_NAMESPACE_RESTORE:
		data := NamespaceRestoreRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		result, err := kubernetes.RestoreNamespace(data.YamlData, data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case structs.PAT_NAMESPACE_RESOURCE_YAML:
		data := NamespaceResourceYamlRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		result, err := punq.AllResourcesFromToCombinedYaml(data.NamespaceName, data.Resources, nil)
		if err != nil {
			return err.Error()
		}
		return result

	case structs.PAT_CLUSTER_HELM_REPO_ADD:
		data := kubernetes.HelmRepoAddRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmRepoAdd(data))
	case structs.PAT_CLUSTER_HELM_REPO_PATCH:
		data := kubernetes.HelmRepoPatchRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmRepoPatch(data))
	case structs.PAT_CLUSTER_HELM_REPO_UPDATE:
		return NewMessageResponse(kubernetes.HelmRepoUpdate())
	case structs.PAT_CLUSTER_HELM_REPO_LIST:
		return NewMessageResponse(kubernetes.HelmRepoList())
	case structs.PAT_CLUSTER_HELM_REPO_REMOVE:
		data := kubernetes.HelmRepoRemoveRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmRepoRemove(data))
	case structs.PAT_CLUSTER_HELM_CHART_SEARCH:
		data := kubernetes.HelmChartSearchRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmChartSearch(data))
	case structs.PAT_CLUSTER_HELM_CHART_INSTALL:
		data := kubernetes.HelmChartInstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmChartInstall(data))
	case structs.PAT_CLUSTER_HELM_CHART_SHOW:
		data := kubernetes.HelmChartShowRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmChartShow(data))
	case structs.PAT_CLUSTER_HELM_CHART_VERSIONS:
		data := kubernetes.HelmChartVersionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmChartVersion(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_UPGRADE:
		data := kubernetes.HelmReleaseUpgradeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmReleaseUpgrade(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_UNINSTALL:
		data := kubernetes.HelmReleaseUninstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmReleaseUninstall(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_LIST:
		data := kubernetes.HelmReleaseListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmReleaseList(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_STATUS:
		data := kubernetes.HelmReleaseStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmReleaseStatus(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_HISTORY:
		data := kubernetes.HelmReleaseHistoryRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmReleaseHistory(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_ROLLBACK:
		data := kubernetes.HelmReleaseRollbackRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmReleaseRollback(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_GET:
		data := kubernetes.HelmReleaseGetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(kubernetes.HelmReleaseGet(data))

	case structs.PAT_SERVICE_CREATE:
		data := ServiceUpdateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		data.Project.AddSecretsToRedaction()
		return UpdateService(data)
	case structs.PAT_SERVICE_DELETE:
		data := ServiceDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		data.Project.AddSecretsToRedaction()
		return DeleteService(data)
	case structs.PAT_SERVICE_POD_IDS:
		data := ServiceGetPodIdsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return ServicePodIds(data)
	case structs.PAT_SERVICE_POD_EXISTS:
		data := ServicePodExistsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return ServicePodExists(data)
	case structs.PAT_SERVICE_PODS:
		data := ServicePodsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return ServicePodStatus(data)
	// case structs.PAT_SERVICE_SET_IMAGE:
	// 	data := ServiceSetImageRequest{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	return SetImage(data)
	case structs.PAT_SERVICE_LOG:
		data := ServiceGetLogRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return PodLog(data)
	case structs.PAT_SERVICE_LOG_ERROR:
		data := ServiceGetLogRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return PodLogError(data)
	case structs.PAT_SERVICE_RESOURCE_STATUS:
		data := ServiceResourceStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return PodStatus(data)
	case structs.PAT_SERVICE_RESTART:
		data := ServiceRestartRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		return Restart(data)
	case structs.PAT_SERVICE_STOP:
		data := ServiceStopRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		return StopService(data)
	case structs.PAT_SERVICE_START:
		data := ServiceStartRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Service.AddSecretsToRedaction()
		return StartService(data)
	case structs.PAT_SERVICE_UPDATE_SERVICE:
		data := ServiceUpdateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Project.AddSecretsToRedaction()
		data.Service.AddSecretsToRedaction()
		return UpdateService(data)
	case structs.PAT_SERVICE_TRIGGER_JOB:
		data := ServiceTriggerJobRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return TriggerJobService(data)
	case structs.PAT_SERVICE_STATUS:
		data := ServiceStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return StatusServiceDebounced(data)

	case structs.PAT_SERVICE_LOG_STREAM:
		data := ServiceLogStreamRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return logStream(data, datagram)

	case structs.PAT_SERVICE_EXEC_SH_CONNECTION_REQUEST:
		data := xterm.PodCmdConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go execShConnection(data)
		return nil

	case structs.PAT_SERVICE_LOG_STREAM_CONNECTION_REQUEST:
		data := xterm.PodCmdConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go logStreamConnection(data)
		return nil
	case structs.PAT_SERVICE_BUILD_LOG_STREAM_CONNECTION_REQUEST:
		data := xterm.BuildLogConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go buildLogStreamConnection(data)
		return nil
	case structs.PAT_SERVICE_OPERATOR_LOG_STREAM_CONNECTION_REQUEST:
		data := xterm.OperatorLogConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go operatorLogStreamConnection(data)
		return nil
	case structs.PAT_CLUSTER_COMPONENT_LOG_STREAM_CONNECTION_REQUEST:
		data := xterm.ComponentLogConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go componentLogStreamConnection(data)
		return nil
	case structs.PAT_SERVICE_POD_EVENT_STREAM_CONNECTION_REQUEST:
		data := xterm.PodEventConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go podEventStreamConnection(data)
		return nil
	case structs.PAT_SERVICE_SCAN_IMAGE_LOG_STREAM_CONNECTION_REQUEST:
		data := xterm.ScanImageLogConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.AddSecretsToRedaction()
		go scanImageLogStreamConnection(data)
		return nil
	case structs.PAT_SERVICE_CLUSTER_TOOL_STREAM_CONNECTION_REQUEST:
		data := xterm.ClusterToolConnectionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		go XTermClusterToolStreamConnection(data)
		return nil

	case structs.PAT_LIST_CREATE_TEMPLATES:
		return punq.ListCreateTemplates()

	case structs.PAT_LIST_NAMESPACES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.ListK8sNamespaces(data.NamespaceName, nil)
	case structs.PAT_LIST_DEPLOYMENTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sDeployments(data.NamespaceName, nil)
	case structs.PAT_LIST_SERVICES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sServices(data.NamespaceName, nil)
	case structs.PAT_LIST_PODS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sPods(data.NamespaceName, nil)
	case structs.PAT_LIST_INGRESSES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sIngresses(data.NamespaceName, nil)
	case structs.PAT_LIST_CONFIGMAPS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sConfigmaps(data.NamespaceName, nil)
	case structs.PAT_LIST_SECRETS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sSecrets(data.NamespaceName, nil)
	case structs.PAT_LIST_NODES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.ListK8sNodes(nil)
	case structs.PAT_LIST_DAEMONSETS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sDaemonsets(data.NamespaceName, nil)
	case structs.PAT_LIST_STATEFULSETS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sStatefulSets(data.NamespaceName, nil)
	case structs.PAT_LIST_JOBS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sJobs(data.NamespaceName, nil)
	case structs.PAT_LIST_CRONJOBS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sCronjobs(data.NamespaceName, nil)
	case structs.PAT_LIST_REPLICASETS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sReplicasets(data.NamespaceName, nil)
	case structs.PAT_LIST_PERSISTENT_VOLUMES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sPersistentVolumes(nil)
	case structs.PAT_LIST_PERSISTENT_VOLUME_CLAIMS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sPersistentVolumeClaims(data.NamespaceName, nil)
	case structs.PAT_LIST_HORIZONTAL_POD_AUTOSCALERS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sHpas(data.NamespaceName, nil)
	case structs.PAT_LIST_EVENTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sEvents(data.NamespaceName, nil)
	case structs.PAT_LIST_CERTIFICATES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sCertificates(data.NamespaceName, nil)
	case structs.PAT_LIST_CERTIFICATEREQUESTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sCertificateSigningRequests(data.NamespaceName, nil)
	case structs.PAT_LIST_ORDERS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllOrders(data.NamespaceName, nil)
	case structs.PAT_LIST_ISSUERS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sIssuer(data.NamespaceName, nil)
	case structs.PAT_LIST_CLUSTERISSUERS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sClusterIssuers(nil)
	case structs.PAT_LIST_SERVICE_ACCOUNT:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sServiceAccounts(data.NamespaceName, nil)
	case structs.PAT_LIST_ROLE:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sRoles(data.NamespaceName, nil)
	case structs.PAT_LIST_ROLE_BINDING:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sRoleBindings(data.NamespaceName, nil)
	case structs.PAT_LIST_CLUSTER_ROLE:
		return punq.AllK8sClusterRoles(nil)
	case structs.PAT_LIST_CLUSTER_ROLE_BINDING:
		return punq.AllK8sClusterRoleBindings(nil)
	case structs.PAT_LIST_VOLUME_ATTACHMENT:
		return punq.AllK8sVolumeAttachments(nil)
	case structs.PAT_LIST_NETWORK_POLICY:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sNetworkPolicies(data.NamespaceName, nil)
	case structs.PAT_LIST_STORAGE_CLASS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sStorageClasses(nil)
	case structs.PAT_LIST_CUSTOM_RESOURCE_DEFINITIONS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		// TODO: sdk not giving crds. on hold
		return nil
	case structs.PAT_LIST_ENDPOINTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sEndpoints(data.NamespaceName, nil)
	case structs.PAT_LIST_LEASES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sLeases(data.NamespaceName, nil)
	case structs.PAT_LIST_PRIORITYCLASSES:
		return punq.AllPriorityClasses(nil)
	case structs.PAT_LIST_VOLUMESNAPSHOTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		// TODO: sdk not giving crds. on hold
		return nil
	case structs.PAT_LIST_RESOURCEQUOTAS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.AllK8sResourceQuotas(data.NamespaceName, nil)

	case structs.PAT_DESCRIBE_NAMESPACE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sNamespace(data.ResourceName, nil)
	case structs.PAT_DESCRIBE_DEPLOYMENT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sDeployment(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_SERVICE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sService(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_POD:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sPod(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_INGRESS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sIngress(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_CONFIGMAP:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sConfigmap(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_SECRET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sSecret(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_NODE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sNode(data.ResourceName, nil)
	case structs.PAT_DESCRIBE_DAEMONSET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sDaemonSet(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_STATEFULSET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sStatefulset(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_JOB:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sJob(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_CRONJOB:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sCronJob(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_REPLICASET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sReplicaset(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_PERSISTENT_VOLUME:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sPersistentVolume(data.ResourceName, nil)
	case structs.PAT_DESCRIBE_PERSISTENT_VOLUME_CLAIM:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sPersistentVolumeClaim(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_HORIZONTAL_POD_AUTOSCALER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sHpa(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_EVENT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sEvent(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_CERTIFICATE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sCertificate(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_CERTIFICATEREQUEST:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sCertificateSigningRequest(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_ORDER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sOrder(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_ISSUER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sIssuer(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_CLUSTERISSUER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sClusterIssuer(data.ResourceName, nil)
	case structs.PAT_DESCRIBE_SERVICE_ACCOUNT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sServiceAccount(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_ROLE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sRole(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_ROLE_BINDING:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sRoleBinding(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_CLUSTER_ROLE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sClusterRole(data.ResourceName, nil)
	case structs.PAT_DESCRIBE_CLUSTER_ROLE_BINDING:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sClusterRoleBinding(data.ResourceName, nil)
	case structs.PAT_DESCRIBE_VOLUME_ATTACHMENT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sVolumeAttachment(data.ResourceName, nil)
	case structs.PAT_DESCRIBE_NETWORK_POLICY:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sNetworkPolicy(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_STORAGE_CLASS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sStorageClass(data.ResourceName, nil)
	case structs.PAT_DESCRIBE_CUSTOM_RESOURCE_DEFINITIONS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sCustomResourceDefinition(data.ResourceName, nil)
	case structs.PAT_DESCRIBE_ENDPOINTS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sEndpoint(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_LEASES:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sLease(data.NamespaceName, data.ResourceName, nil)
	case structs.PAT_DESCRIBE_PRIORITYCLASSES:
		return punq.AllPriorityClasses(nil)
	case structs.PAT_DESCRIBE_VOLUMESNAPSHOTS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		// TODO: sdk not giving crds. on hold
		return nil
	case structs.PAT_DESCRIBE_RESOURCEQUOTAS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DescribeK8sResourceQuota(data.NamespaceName, data.ResourceName, nil)

	case structs.PAT_UPDATE_NAMESPACE:
		data := K8sUpdateNamespaceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		if datagram.Err != "" {
			fmt.Println("Error in request: ", datagram.Err)
			return datagram.Err
		}
		return K8sUpdateNamespace(data)
	case structs.PAT_UPDATE_DEPLOYMENT:
		data := K8sUpdateDeploymentRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		if datagram.Err != "" {
			fmt.Println("Error in request: ", datagram.Err)
			return datagram.Err
		}
		return K8sUpdateDeployment(data)
	case structs.PAT_UPDATE_SERVICE:
		data := K8sUpdateServiceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdateService(data)
	case structs.PAT_UPDATE_POD:
		data := K8sUpdatePodRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdatePod(data)
	case structs.PAT_UPDATE_INGRESS:
		data := K8sUpdateIngressRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdateIngress(data)
	case structs.PAT_UPDATE_CONFIGMAP:
		data := K8sUpdateConfigmapRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdateConfigMap(data)
	case structs.PAT_UPDATE_SECRET:
		data := K8sUpdateSecretRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdateSecret(data)
	case structs.PAT_UPDATE_DAEMONSET:
		data := K8sUpdateDaemonSetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdateDaemonSet(data)
	case structs.PAT_UPDATE_STATEFULSET:
		data := K8sUpdateStatefulSetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdateStatefulset(data)
	case structs.PAT_UPDATE_JOB:
		data := K8sUpdateJobRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdateJob(data)
	case structs.PAT_UPDATE_CRONJOB:
		data := K8sUpdateCronJobRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdateCronJob(data)
	case structs.PAT_UPDATE_REPLICASET:
		data := K8sUpdateReplicaSetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sUpdateReplicaSet(data)
	case structs.PAT_UPDATE_PERSISTENT_VOLUME:
		data := K8sUpdatePersistentVolumeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sPersistentVolume(*data.Data, nil)
	case structs.PAT_UPDATE_PERSISTENT_VOLUME_CLAIM:
		data := K8sUpdatePersistentVolumeClaimRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sPersistentVolumeClaim(*data.Data, nil)
	case structs.PAT_UPDATE_HORIZONTAL_POD_AUTOSCALERS:
		data := K8sUpdateHPARequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sHpa(*data.Data, nil)
	case structs.PAT_UPDATE_CERTIFICATES:
		data := K8sUpdateCertificateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sCertificate(*data.Data, nil)
	case structs.PAT_UPDATE_CERTIFICATEREQUESTS:
		data := K8sUpdateCertificateRequestRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sCertificateSigningRequest(*data.Data, nil)
	case structs.PAT_UPDATE_ORDERS:
		data := K8sUpdateOrderRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sOrder(*data.Data, nil)
	case structs.PAT_UPDATE_ISSUERS:
		data := K8sUpdateIssuerRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sIssuer(*data.Data, nil)
	case structs.PAT_UPDATE_CLUSTERISSUERS:
		data := K8sUpdateClusterIssuerRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sClusterIssuer(*data.Data, nil)
	case structs.PAT_UPDATE_SERVICE_ACCOUNT:
		data := K8sUpdateServiceAccountRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sServiceAccount(*data.Data, nil)
	case structs.PAT_UPDATE_ROLE:
		data := K8sUpdateRoleRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sRole(*data.Data, nil)
	case structs.PAT_UPDATE_ROLE_BINDING:
		data := K8sUpdateRoleBindingRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sRoleBinding(*data.Data, nil)
	case structs.PAT_UPDATE_CLUSTER_ROLE:
		data := K8sUpdateClusterRoleRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sClusterRole(*data.Data, nil)
	case structs.PAT_UPDATE_CLUSTER_ROLE_BINDING:
		data := K8sUpdateClusterRoleBindingRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sClusterRoleBinding(*data.Data, nil)
	case structs.PAT_UPDATE_VOLUME_ATTACHMENT:
		data := K8sUpdateVolumeAttachmentRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sVolumeAttachment(*data.Data, nil)
	case structs.PAT_UPDATE_NETWORK_POLICY:
		data := K8sUpdateNetworkPolicyRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sNetworkPolicy(*data.Data, nil)
	case structs.PAT_UPDATE_STORAGE_CLASS:
		data := K8sUpdateStorageClassRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sStorageClass(*data.Data, nil)
	case structs.PAT_UPDATE_CUSTOM_RESOURCE_DEFINITIONS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		// TODO: sdk not giving crds. on hold
		return nil
	case structs.PAT_UPDATE_ENDPOINTS:
		data := K8sUpdateEndpointRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sEndpoint(*data.Data, nil)
	case structs.PAT_UPDATE_LEASES:
		data := K8sUpdateLeaseRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sLease(*data.Data, nil)
	case structs.PAT_UPDATE_PRIORITYCLASSES:
		data := K8sUpdatePriorityClassRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sPriorityClass(*data.Data, nil)
	case structs.PAT_UPDATE_VOLUMESNAPSHOTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		// TODO: sdk not giving crds. on hold
		return nil
	case structs.PAT_UPDATE_RESOURCEQUOTAS:
		data := K8sUpdateResourceQuotaRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.UpdateK8sResourceQuota(*data.Data, nil)

	case structs.PAT_DELETE_NAMESPACE:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteNamespace(data)
	case structs.PAT_DELETE_DEPLOYMENT:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteDeployment(data)
	case structs.PAT_DELETE_SERVICE:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteService(data)
	case structs.PAT_DELETE_POD:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeletePod(data)
	case structs.PAT_DELETE_INGRESS:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteIngress(data)
	case structs.PAT_DELETE_CONFIGMAP:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteConfigMap(data)
	case structs.PAT_DELETE_SECRET:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteSecret(data)
	case structs.PAT_DELETE_DAEMONSET:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteDaemonSet(data)
	case structs.PAT_DELETE_STATEFULSET:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteStatefulset(data)
	case structs.PAT_DELETE_JOB:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteJob(data)
	case structs.PAT_DELETE_CRONJOB:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteCronJob(data)
	case structs.PAT_DELETE_REPLICASET:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return K8sDeleteReplicaSet(data)
	case structs.PAT_DELETE_PERSISTENT_VOLUME:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sPersistentVolumeBy(data.Name, nil)
	case structs.PAT_DELETE_PERSISTENT_VOLUME_CLAIM:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sPersistentVolumeClaimBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_HORIZONTAL_POD_AUTOSCALERS:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sHpaBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_CERTIFICATES:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sCertificateBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_CERTIFICATEREQUESTS:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sCertificateSigningRequestBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_ORDERS:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sOrderBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_ISSUERS:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sIssuerBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_CLUSTERISSUERS:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sClusterIssuerBy(data.Name, nil)
	case structs.PAT_DELETE_SERVICE_ACCOUNT:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sServiceAccountBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_ROLE:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sRoleBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_ROLE_BINDING:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sRoleBindingBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_CLUSTER_ROLE:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sClusterRoleBy(data.Name, nil)
	case structs.PAT_DELETE_CLUSTER_ROLE_BINDING:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sClusterRoleBindingBy(data.Name, nil)
	case structs.PAT_DELETE_VOLUME_ATTACHMENT:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sVolumeAttachmentBy(data.Name, nil)
	case structs.PAT_DELETE_NETWORK_POLICY:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sNetworkPolicyBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_STORAGE_CLASS:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sStorageClassBy(data.Name, nil)
	case structs.PAT_DELETE_CUSTOM_RESOURCE_DEFINITIONS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		// TODO: sdk not giving crds. on hold
		return nil
	case structs.PAT_DELETE_ENDPOINTS:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sEndpointBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_LEASES:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sLeaseBy(data.Namespace, data.Name, nil)
	case structs.PAT_DELETE_PRIORITYCLASSES:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sPriorityClassBy(data.Name, nil)
	case structs.PAT_DELETE_VOLUMESNAPSHOTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		// TODO: sdk not giving crds. on hold
		return nil
	case structs.PAT_DELETE_RESOURCEQUOTAS:
		data := K8sDeleteResourceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return punq.DeleteK8sResourceQuotaBy(data.Namespace, data.Name, nil)

	case structs.PAT_GET_NAMESPACE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetNamespace(data.NamespaceName, nil)
		return getResult
	case structs.PAT_GET_DEPLOYMENT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetK8sDeployment(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_SERVICE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetService(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_POD:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult := punq.GetPod(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_INGRESS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetK8sIngress(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_CONFIGMAP:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetK8sConfigmap(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_SECRET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetSecret(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_NODE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetK8sNode(data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_DAEMONSET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetK8sDaemonset(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_STATEFULSET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetStatefulSet(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_JOB:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetJob(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_CRONJOB:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetCronjob(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_REPLICASET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetReplicaset(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_PERSISTENT_VOLUME:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetPersistentVolume(data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_PERSISTENT_VOLUME_CLAIM:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetPersistentVolumeClaim(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_HORIZONTAL_POD_AUTOSCALER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetHpa(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_EVENT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetEvent(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_CERTIFICATE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetCertificate(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_CERTIFICATEREQUEST:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetCertificateSigningRequest(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_ORDER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetOrder(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_ISSUER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetIssuer(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_CLUSTERISSUER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetClusterIssuer(data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_SERVICE_ACCOUNT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetServiceAccount(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_ROLE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetRole(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_ROLE_BINDING:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetRoleBinding(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_CLUSTER_ROLE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetClusterRole(data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_CLUSTER_ROLE_BINDING:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetClusterRoleBinding(data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_VOLUME_ATTACHMENT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetVolumeAttachment(data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_NETWORK_POLICY:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetNetworkPolicy(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_STORAGE_CLASS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetStorageClass(data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_ENDPOINTS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetEndpoint(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_LEASES:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetLeas(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_PRIORITYCLASSES:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetPriorityClass(data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_VOLUMESNAPSHOTS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetVolumeSnapshot(data.NamespaceName, data.ResourceName, nil)
		return getResult
	case structs.PAT_GET_RESOURCEQUOTAS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		getResult, _ := punq.GetResourceQuota(data.NamespaceName, data.ResourceName, nil)
		return getResult

	case structs.PAT_BUILDER_STATUS:
		return BuilderStatus()
	case structs.PAT_BUILD_INFOS:
		data := structs.BuildJobStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return BuildJobInfos(data.BuildId)
	case structs.PAT_BUILD_LAST_INFOS:
		data := structs.BuildTaskRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return LastBuildInfos(data)
	case structs.PAT_BUILD_LIST_ALL:
		return ListAll()
	case structs.PAT_BUILD_LIST_BY_PROJECT:
		data := structs.ListBuildByProjectIdRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return ListByProjectId(data.ProjectId)
	case structs.PAT_BUILD_ADD:
		data := structs.BuildJob{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Project.AddSecretsToRedaction()
		data.Service.AddSecretsToRedaction()
		return AddBuildJob(data)
	case structs.PAT_BUILD_CANCEL:
		data := structs.BuildJob{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.Project.AddSecretsToRedaction()
		data.Service.AddSecretsToRedaction()
		return Cancel(data.BuildId)
	case structs.PAT_BUILD_DELETE:
		data := structs.BuildJobStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return DeleteBuild(data.BuildId)
	case structs.PAT_BUILD_LAST_JOB_OF_SERVICES:
		data := structs.BuildTaskListOfServicesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return LastBuildInfosOfServices(data)
	case structs.PAT_BUILD_JOB_LIST_OF_SERVICE:
		data := structs.BuildTaskRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return BuildInfosList(data)
	case structs.PAT_BUILD_DELETE_ALL_OF_SERVICE:
		data := structs.BuildTaskRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		DeleteAllBuildData(data)
		return nil
	//case structs.PAT_BUILD_LAST_JOB_INFO_OF_SERVICE:
	//	data := structs.BuildServiceRequest{}
	//	structs.MarshalUnmarshal(&datagram, &data)
	//	if err := utils.ValidateJSON(data); err != nil {
	//		return err
	//	}
	//	return LastBuildForService(data.ServiceId)

	case structs.PAT_STORAGE_CREATE_VOLUME:
		data := NfsVolumeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return CreateMogeniusNfsVolume(data)
	case structs.PAT_STORAGE_DELETE_VOLUME:
		data := NfsVolumeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return DeleteMogeniusNfsVolume(data)
	// case structs.PAT_STORAGE_BACKUP_VOLUME:
	// 	data := NfsVolumeBackupRequest{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	data.AddSecretsToRedaction()
	// 	return BackupMogeniusNfsVolume(data)
	// case structs.PAT_STORAGE_RESTORE_VOLUME:
	// 	data := NfsVolumeRestoreRequest{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	data.AddSecretsToRedaction()
	// 	return RestoreMogeniusNfsVolume(data)
	case structs.PAT_STORAGE_STATS:
		data := NfsVolumeStatsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return StatsMogeniusNfsVolume(data)
	case structs.PAT_STORAGE_NAMESPACE_STATS:
		data := NfsNamespaceStatsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return StatsMogeniusNfsNamespace(data)
	case structs.PAT_STORAGE_STATUS:
		data := NfsStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return StatusMogeniusNfs(data)
	case structs.PAT_POPEYE_CONSOLE:
		return PopeyeConsole()
	case structs.PAT_LOG_LIST_ALL:
		return db.ListLogFromDb()
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// External Secrets
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	case structs.PAT_EXTERNAL_SECRET_STORE_CREATE:
		data := controllers.CreateSecretsStoreRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return controllers.CreateExternalSecretStore(data)
	case structs.PAT_EXTERNAL_SECRET_STORE_LIST:
		data := controllers.ListSecretStoresRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return controllers.ListExternalSecretsStores(data)
	case structs.PAT_EXTERNAL_SECRET_LIST_AVAILABLE_SECRETS:
		data := controllers.ListSecretsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return controllers.ListAvailableExternalSecrets(data)
	case structs.PAT_EXTERNAL_SECRET_STORE_DELETE:
		data := controllers.DeleteSecretsStoreRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return controllers.DeleteExternalSecretsStore(data)
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Labeled Network Policies
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	case structs.PAT_ATTACH_LABELED_NETWORK_POLICY:
		data := controllers.AttachLabeledNetworkPolicyRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.AttachLabeledNetworkPolicy(data))
	case structs.PAT_DETACH_LABELED_NETWORK_POLICY:
		data := controllers.DetachLabeledNetworkPolicyRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.DetachLabeledNetworkPolicy(data))
	case structs.PAT_LIST_LABELED_NETWORK_POLICY_PORTS:
		return controllers.ListLabeledNetworkPolicyPorts()
	case structs.PAT_LIST_CONFLICTING_NETWORK_POLICIES:
		data := controllers.RemoveConflictingNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.ListAllConflictingNetworkPolicies(data))
	case structs.PAT_REMOVE_CONFLICTING_NETWORK_POLICIES:
		data := controllers.RemoveConflictingNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.RemoveConflictingNetworkPolicies(data))
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	// Cronjobs
	// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	case structs.PAT_LIST_CRONJOB_JOBS:
		data := ListCronjobJobsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return kubernetes.ListCronjobJobs(data.ControllerName, data.NamespaceName, data.ProjectId)
	}
	datagram.Err = "Pattern not found"
	return datagram
}

func logStream(data ServiceLogStreamRequest, datagram structs.Datagram) ServiceLogStreamResult {
	result := ServiceLogStreamResult{}

	url, err := url.Parse(data.PostTo)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		ServiceLogger.Error(result.Error)
		return result
	}

	pod := punq.PodStatus(data.Namespace, data.PodId, false, nil)
	terminatedState := punq.LastTerminatedStateIfAny(pod)

	var previousResReq *rest.Request
	if terminatedState != nil {
		tmpPreviousResReq, err := PreviousPodLogStream(data.Namespace, data.PodId)
		if err != nil {
			ServiceLogger.Error(err.Error())
		} else {
			previousResReq = tmpPreviousResReq
		}
	}

	restReq, err := PodLogStream(data)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		ServiceLogger.Error(result.Error)
		return result
	}

	if terminatedState != nil {
		ServiceLogger.Infof("Logger try multiStreamData")
		go multiStreamData(previousResReq, restReq, terminatedState, url.String())
	} else {
		ServiceLogger.Infof("Logger try streamData")
		go streamData(restReq, url.String())
	}

	result.Success = true

	return result
}

func streamData(restReq *rest.Request, toServerUrl string) {
	ctx := context.Background()
	cancelCtx, endGofunc := context.WithCancel(ctx)
	stream, err := restReq.Stream(cancelCtx)
	if err != nil {
		ServiceLogger.Error(err.Error())
	} else {
		structs.SendDataWs(toServerUrl, stream)
	}
	endGofunc()
}

func multiStreamData(previousRestReq *rest.Request, restReq *rest.Request, terminatedState *v1.ContainerStateTerminated, toServerUrl string) {
	ctx := context.Background()
	cancelCtx, endGofunc := context.WithCancel(ctx)

	lastState := punq.LastTerminatedStateToString(terminatedState)

	var previousStream io.ReadCloser
	if previousRestReq != nil {
		tmpPreviousStream, err := previousRestReq.Stream(cancelCtx)
		if err != nil {
			ServiceLogger.Error(err.Error())
			previousStream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
		} else {
			previousStream = tmpPreviousStream
		}
	}

	stream, err := restReq.Stream(cancelCtx)
	if err != nil {
		ServiceLogger.Error(err.Error())
		stream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
	}

	nl := strings.NewReader("\n")
	previousState := strings.NewReader(lastState)
	headlineLastLog := strings.NewReader("Last Log:\n")
	headlineCurrentLog := strings.NewReader("Current Log:\n")

	mergedStream := io.MultiReader(previousState, nl, headlineLastLog, nl, previousStream, nl, headlineCurrentLog, nl, stream)

	structs.SendDataWs(toServerUrl, io.NopCloser(mergedStream))
	endGofunc()
}

func PopeyeConsole() string {
	return punqStructs.ExecuteShellCommandWithResponse("Generate popeye report", "popeye --force-exit-zero")
}

func ExecuteBinaryRequestUpload(datagram structs.Datagram) *FilesUploadRequest {
	data := FilesUploadRequest{}
	structs.MarshalUnmarshal(&datagram, &data)
	return &data
}

func K8sNotification(d structs.Datagram) interface{} {
	ServiceLogger.Infof("Received '%s'.", d.Pattern)
	return nil
}

func execShConnection(podCmdConnectionRequest xterm.PodCmdConnectionRequest) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("kubectl exec -it -c %s -n %s %s -- sh -c \"clear; echo -e '\033[97;104m Connected to %s/%s/%s (using $0): \033[0m'; (type bash >/dev/null 2>&1 && exec bash || type ash >/dev/null 2>&1 && exec ash || type sh >/dev/null 2>&1 && exec sh || type ksh >/dev/null 2>&1 && exec ksh || type csh >/dev/null 2>&1 && exec csh || type zsh >/dev/null 2>&1 && exec zsh)\"", podCmdConnectionRequest.Container, podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod, podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod, podCmdConnectionRequest.Container))

	xterm.XTermCommandStreamConnection(
		"exec-sh",
		podCmdConnectionRequest.WsConnection,
		podCmdConnectionRequest.Namespace,
		podCmdConnectionRequest.Controller,
		podCmdConnectionRequest.Pod,
		podCmdConnectionRequest.Container,
		cmd,
		nil,
	)
}

func GetPreviousLogContent(podCmdConnectionRequest xterm.PodCmdConnectionRequest) io.Reader {
	ctx := context.Background()
	cancelCtx, endGofunc := context.WithCancel(ctx)
	defer endGofunc()

	pod := punq.PodStatus(podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod, false, nil)
	terminatedState := punq.LastTerminatedStateIfAny(pod)

	var previousRestReq *rest.Request
	if terminatedState != nil {
		tmpPreviousResReq, err := PreviousPodLogStream(podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod)
		if err != nil {
			ServiceLogger.Error(err.Error())
		} else {
			previousRestReq = tmpPreviousResReq
		}
	}

	if previousRestReq == nil {
		return nil
	}

	var previousStream io.ReadCloser
	tmpPreviousStream, err := previousRestReq.Stream(cancelCtx)
	if err != nil {
		ServiceLogger.Error(err.Error())
		previousStream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
	} else {
		previousStream = tmpPreviousStream
	}

	data, err := io.ReadAll(previousStream)
	if err != nil {
		ServiceLogger.Errorf("failed to read data: %v", err)
	}

	lastState := punq.LastTerminatedStateToString(terminatedState)

	nl := strings.NewReader("\r\n")
	previousState := strings.NewReader(lastState)
	headlineLastLog := strings.NewReader("Last Log:\r\n")
	headlineCurrentLog := strings.NewReader("\r\nCurrent Log:\r\n")

	return io.MultiReader(previousState, nl, headlineLastLog, strings.NewReader(string(data)), nl, headlineCurrentLog)
}

func logStreamConnection(podCmdConnectionRequest xterm.PodCmdConnectionRequest) {
	if podCmdConnectionRequest.LogTail == "" {
		podCmdConnectionRequest.LogTail = "1000"
	}
	cmd := exec.Command("kubectl", "logs", "-f", podCmdConnectionRequest.Pod, fmt.Sprintf("--tail=%s", podCmdConnectionRequest.LogTail), "-c", podCmdConnectionRequest.Container, "-n", podCmdConnectionRequest.Namespace)
	xterm.XTermCommandStreamConnection(
		"log",
		podCmdConnectionRequest.WsConnection,
		podCmdConnectionRequest.Namespace,
		podCmdConnectionRequest.Controller,
		podCmdConnectionRequest.Pod,
		podCmdConnectionRequest.Container,
		cmd,
		GetPreviousLogContent(podCmdConnectionRequest),
	)
}

func buildLogStreamConnection(buildLogConnectionRequest xterm.BuildLogConnectionRequest) {
	xterm.XTermBuildLogStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.Namespace,
		buildLogConnectionRequest.Controller,
		buildLogConnectionRequest.Container,
		buildLogConnectionRequest.BuildTask,
		buildLogConnectionRequest.BuildId,
	)
}

func operatorLogStreamConnection(operatorLogConnectionRequest xterm.OperatorLogConnectionRequest) {
	if operatorLogConnectionRequest.LogTail == "" {
		operatorLogConnectionRequest.LogTail = "1000"
	}
	xterm.XTermOperatorStreamConnection(
		operatorLogConnectionRequest.WsConnection,
		operatorLogConnectionRequest.Namespace,
		operatorLogConnectionRequest.Controller,
	)
}

func componentLogStreamConnection(componentLogConnectionRequest xterm.ComponentLogConnectionRequest) {
	xterm.XTermComponentStreamConnection(
		componentLogConnectionRequest.WsConnection,
		componentLogConnectionRequest.Component,
		componentLogConnectionRequest.Namespace,
		componentLogConnectionRequest.Release,
	)
}

func podEventStreamConnection(buildLogConnectionRequest xterm.PodEventConnectionRequest) {
	xterm.XTermPodEventStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.Namespace,
		buildLogConnectionRequest.Controller,
	)
}

func scanImageLogStreamConnection(buildLogConnectionRequest xterm.ScanImageLogConnectionRequest) {
	xterm.XTermScanImageLogStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.Namespace,
		buildLogConnectionRequest.Controller,
		buildLogConnectionRequest.Container,
		buildLogConnectionRequest.CmdType,
		buildLogConnectionRequest.ScanImageType,
		buildLogConnectionRequest.ContainerRegistryUrl,
		&buildLogConnectionRequest.ContainerRegistryUser,
		&buildLogConnectionRequest.ContainerRegistryPat,
	)
}
func XTermClusterToolStreamConnection(buildLogConnectionRequest xterm.ClusterToolConnectionRequest) {
	xterm.XTermClusterToolStreamConnection(
		buildLogConnectionRequest.WsConnection,
		buildLogConnectionRequest.CmdType,
		buildLogConnectionRequest.Tool,
	)
}
