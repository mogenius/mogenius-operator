package services

import (
	"context"
	"fmt"
	"io"
	"mogenius-k8s-manager/src/controllers"
	"mogenius-k8s-manager/src/db"
	dbstats "mogenius-k8s-manager/src/db-stats"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/xterm"
	"os"
	"os/exec"
	"strings"
	"time"

	"mogenius-k8s-manager/src/structs"
	"net/url"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

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

type ClusterResourceInfoDto struct {
	LoadBalancerExternalIps []string              `json:"loadBalancerExternalIps"`
	NodeStats               []dtos.NodeStat       `json:"nodeStats"`
	Country                 *utils.CountryDetails `json:"country"`
	Provider                string                `json:"provider"`
}

func ExecuteCommandRequest(datagram structs.Datagram) interface{} {
	switch datagram.Pattern {
	case structs.PAT_K8SNOTIFICATION:
		return K8sNotification(datagram)
	case structs.PAT_CLUSTERSTATUS:
		return kubernetes.ClusterStatus()
	case structs.PAT_CLUSTERRESOURCEINFO:
		nodeStats := kubernetes.GetNodeStats()
		loadBalancerExternalIps := kubernetes.GetClusterExternalIps()
		country, _ := utils.GuessClusterCountry()
		result := ClusterResourceInfoDto{
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
		serviceLogger.Info("😵😵😵 Received RESTART COMMAND. Restarting now ...")
		time.Sleep(1 * time.Second)
		os.Exit(0)
		return nil
	case structs.PAT_SYSTEM_PRINT_CURRENT_CONFIG:
		return config.AsEnvs()

	// case structs.PAT_IAC_FORCE_SYNC:
	// 	return NewMessageResponse(nil, iacmanager.SyncChanges())
	// case structs.PAT_IAC_GET_STATUS:
	// 	return NewMessageResponse(iacmanager.GetDataModel(), nil)
	// case structs.PAT_IAC_RESET_LOCAL_REPO:
	// 	return NewMessageResponse(nil, iacmanager.ResetLocalRepo())
	// case structs.PAT_IAC_RESET_FILE:
	// 	data := dtos.ResetFileRequest{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	return NewMessageResponse(nil, iacmanager.ResetFile(data.FilePath, data.CommitHash))

	case structs.PAT_ENERGY_CONSUMPTION:
		return EnergyConsumption()

	case structs.PAT_CLUSTER_SYNC_INFO:
		result, err := kubernetes.GetSyncRepoData()
		if err != nil {
			return err
		}
		return result

	// case structs.PAT_CLUSTER_SYNC_UPDATE:
	// 	data := dtos.SyncRepoData{}
	// 	structs.MarshalUnmarshal(&datagram, &data)
	// 	if err := utils.ValidateJSON(data); err != nil {
	// 		return err
	// 	}
	// 	data.AddSecretsToRedaction()
	// 	err := iacmanager.UpdateSyncRepoData(&data)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	err = iacmanager.CheckRepoAccess()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return err

	case structs.PAT_INSTALL_TRAFFIC_COLLECTOR:
		result, err := InstallTrafficCollector()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_POD_STATS_COLLECTOR:
		result, err := InstallPodStatsCollector()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_METRICS_SERVER:
		result, err := InstallMetricsServer()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_INGRESS_CONTROLLER_TREAFIK:
		result, err := InstallIngressControllerTreafik()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_CERT_MANAGER:
		result, err := InstallCertManager()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_CLUSTER_ISSUER:
		data := ClusterIssuerInstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		data.AddSecretsToRedaction()
		result, err := InstallClusterIssuer(data.Email, 0)
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_CONTAINER_REGISTRY:
		result, err := InstallContainerRegistry()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_EXTERNAL_SECRETS:
		result, err := InstallExternalSecrets()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_METALLB:
		result, err := InstallMetalLb()
		return NewMessageResponse(result, err)
	case structs.PAT_INSTALL_KEPLER:
		result, err := InstallKepler()
		return NewMessageResponse(result, err)
	case structs.PAT_UNINSTALL_TRAFFIC_COLLECTOR:
		msg, err := UninstallTrafficCollector()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_POD_STATS_COLLECTOR:
		msg, err := UninstallPodStatsCollector()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_METRICS_SERVER:
		msg, err := UninstallMetricsServer()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_INGRESS_CONTROLLER_TREAFIK:
		msg, err := UninstallIngressControllerTreafik()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_CERT_MANAGER:
		msg, err := UninstallCertManager()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_CLUSTER_ISSUER:
		msg, err := UninstallClusterIssuer()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_CONTAINER_REGISTRY:
		msg, err := UninstallContainerRegistry()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_EXTERNAL_SECRETS:
		msg, err := UninstallExternalSecrets()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_METALLB:
		msg, err := UninstallMetalLb()
		return NewMessageResponse(msg, err)
	case structs.PAT_UNINSTALL_KEPLER:
		msg, err := UninstallKepler()
		return NewMessageResponse(msg, err)
	case structs.PAT_UPGRADE_TRAFFIC_COLLECTOR:
		result, err := UpgradeTrafficCollector()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_PODSTATS_COLLECTOR:
		result, err := UpgradePodStatsCollector()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_METRICS_SERVER:
		result, err := UpgradeMetricsServer()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_INGRESS_CONTROLLER_TREAFIK:
		result, err := UpgradeIngressControllerTreafik()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_CERT_MANAGER:
		result, err := UpgradeCertManager()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_CONTAINER_REGISTRY:
		result, err := UpgradeContainerRegistry()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_METALLB:
		result, err := UpgradeMetalLb()
		return NewMessageResponse(result, err)
	case structs.PAT_UPGRADE_KEPLER:
		result, err := UpgradeKepler()
		return NewMessageResponse(result, err)

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
	case structs.PAT_STATS_TRAFFIC_FOR_CONTROLLER_SOCKET_CONNECTIONS:
		data := kubernetes.K8sController{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return dbstats.GetSocketConnectionsForController(data)

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
		return NewMessageResponse(kubernetes.GetPersistentVolumeClaim(data.Namespace, data.Name))
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
		result, err := kubernetes.AllResourcesFromToCombinedYaml(data.NamespaceName, data.Resources)
		if err != nil {
			return err.Error()
		}
		return result

	case structs.PAT_CLUSTER_HELM_REPO_ADD:
		data := helm.HelmRepoAddRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmRepoAdd(data))
	case structs.PAT_CLUSTER_HELM_REPO_PATCH:
		data := helm.HelmRepoPatchRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmRepoPatch(data))
	case structs.PAT_CLUSTER_HELM_REPO_UPDATE:
		return NewMessageResponse(helm.HelmRepoUpdate())
	case structs.PAT_CLUSTER_HELM_REPO_LIST:
		return NewMessageResponse(helm.HelmRepoList())
	case structs.PAT_CLUSTER_HELM_REPO_REMOVE:
		data := helm.HelmRepoRemoveRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmRepoRemove(data))
	case structs.PAT_CLUSTER_HELM_CHART_SEARCH:
		data := helm.HelmChartSearchRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmChartSearch(data))
	case structs.PAT_CLUSTER_HELM_CHART_INSTALL:
		data := helm.HelmChartInstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmChartInstall(data))
	case structs.PAT_CLUSTER_HELM_CHART_SHOW:
		data := helm.HelmChartShowRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmChartShow(data))
	case structs.PAT_CLUSTER_HELM_CHART_VERSIONS:
		data := helm.HelmChartVersionRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmChartVersion(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_UPGRADE:
		data := helm.HelmReleaseUpgradeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseUpgrade(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_UNINSTALL:
		data := helm.HelmReleaseUninstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseUninstall(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_LIST:
		data := helm.HelmReleaseListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseList(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_STATUS:
		data := helm.HelmReleaseStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseStatus(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_HISTORY:
		data := helm.HelmReleaseHistoryRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseHistory(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_ROLLBACK:
		data := helm.HelmReleaseRollbackRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseRollback(data))
	case structs.PAT_CLUSTER_HELM_RELEASE_GET:
		data := helm.HelmReleaseGetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(helm.HelmReleaseGet(data))

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

	case structs.PAT_LIST_ALL_WORKLOADS:
		resources, err := kubernetes.GetAvailableResources()
		return NewMessageResponse(resources, err)
	case structs.PAT_GET_WORKLOAD_LIST:
		data := utils.SyncResourceEntry{}
		structs.MarshalUnmarshal(&datagram, &data)
		list, err := kubernetes.GetUnstructuredResourceList(data.Group, data.Version, data.Name, data.Namespace)
		return NewMessageResponse(list, err)
	case structs.PAT_DESCRIBE_WORKLOAD:
		data := utils.SyncResourceItem{}
		structs.MarshalUnmarshal(&datagram, &data)
		describeStr, err := kubernetes.DescribeUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		return NewMessageResponse(describeStr, err)
	case structs.PAT_CREATE_NEW_WORKLOAD:
		data := utils.SyncResourceData{}
		structs.MarshalUnmarshal(&datagram, &data)
		newObj, err := kubernetes.CreateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		return NewMessageResponse(newObj, err)
	case structs.PAT_GET_WORKLOAD:
		data := utils.SyncResourceItem{}
		structs.MarshalUnmarshal(&datagram, &data)
		newObj, err := kubernetes.GetUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		return NewMessageResponse(newObj, err)
	case structs.PAT_GET_WORKLOAD_EXAMPLE:
		data := utils.SyncResourceItem{}
		structs.MarshalUnmarshal(&datagram, &data)
		return NewMessageResponse(kubernetes.GetResourceTemplateYaml(data.Group, data.Version, data.Name, data.Kind, data.Namespace, data.ResourceName), nil)
	case structs.PAT_UPDATE_WORKLOAD:
		data := utils.SyncResourceData{}
		structs.MarshalUnmarshal(&datagram, &data)
		updatedObj, err := kubernetes.UpdateUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.YamlData)
		return NewMessageResponse(updatedObj, err)
	case structs.PAT_DELETE_WORKLOAD:
		data := utils.SyncResourceItem{}
		structs.MarshalUnmarshal(&datagram, &data)
		err := kubernetes.DeleteUnstructuredResource(data.Group, data.Version, data.Name, data.Namespace, data.ResourceName)
		return NewMessageResponse(nil, err)

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
		return NewMessageResponse(controllers.ListLabeledNetworkPolicyPorts())
	case structs.PAT_LIST_CONFLICTING_NETWORK_POLICIES:
		data := controllers.ListConflictingNetworkPoliciesRequest{}
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
	case structs.PAT_LIST_CONTROLLER_NETWORK_POLICIES:
		data := controllers.ListControllerLabeledNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.ListControllerLabeledNetwork(data))
	case structs.PAT_UPDATE_NETWORK_POLICIES_TEMPLATE:
		data := []kubernetes.NetworkPolicy{}
		structs.MarshalUnmarshal(&datagram, &data)
		return NewMessageResponse(nil, controllers.UpdateNetworkPolicyTemplate(data))
	case structs.PAT_LIST_ALL_NETWORK_POLICIES:
		return NewMessageResponse(controllers.ListAllNetworkPolicies())
	case structs.PAT_LIST_NAMESPACE_NETWORK_POLICIES:
		data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.ListNamespaceNetworkPolicies(data))
	case structs.PAT_ENFORCE_NETWORK_POLICY_MANAGER:
		data := controllers.EnforceNetworkPolicyManagerRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(nil, controllers.EnforceNetworkPolicyManager(data.NamespaceName))
	case structs.PAT_REMOVE_UNMANAGED_NETWORK_POLICIES:
		data := controllers.RemoveUnmanagedNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(nil, controllers.RemoveUnmanagedNetworkPolicies(data))
	case structs.PAT_LIST_ONLY_NAMESPACE_NETWORK_POLICIES:
		data := controllers.ListNamespaceLabeledNetworkPoliciesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		if err := utils.ValidateJSON(data); err != nil {
			return err
		}
		return NewMessageResponse(controllers.ListManagedAndUnmanagedNamespaceNetworkPolicies(data))
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
	_ = datagram
	result := ServiceLogStreamResult{}

	url, err := url.Parse(data.PostTo)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		serviceLogger.Error(result.Error)
		return result
	}

	pod := kubernetes.PodStatus(data.Namespace, data.PodId, false)
	terminatedState := kubernetes.LastTerminatedStateIfAny(pod)

	var previousResReq *rest.Request
	if terminatedState != nil {
		tmpPreviousResReq, err := PreviousPodLogStream(data.Namespace, data.PodId)
		if err != nil {
			serviceLogger.Error("failed to get previous pod log stream", "error", err)
		} else {
			previousResReq = tmpPreviousResReq
		}
	}

	restReq, err := PodLogStream(data)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		serviceLogger.Error(result.Error)
		return result
	}

	if terminatedState != nil {
		serviceLogger.Info("Logger try multiStreamData")
		go multiStreamData(previousResReq, restReq, terminatedState, url.String())
	} else {
		serviceLogger.Info("Logger try streamData")
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
		serviceLogger.Error(err.Error())
	} else {
		structs.SendDataWs(toServerUrl, stream)
	}
	endGofunc()
}

func multiStreamData(previousRestReq *rest.Request, restReq *rest.Request, terminatedState *v1.ContainerStateTerminated, toServerUrl string) {
	ctx := context.Background()
	cancelCtx, endGofunc := context.WithCancel(ctx)

	lastState := kubernetes.LastTerminatedStateToString(terminatedState)

	var previousStream io.ReadCloser
	if previousRestReq != nil {
		tmpPreviousStream, err := previousRestReq.Stream(cancelCtx)
		if err != nil {
			serviceLogger.Error(err.Error())
			previousStream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
		} else {
			previousStream = tmpPreviousStream
		}
	}

	stream, err := restReq.Stream(cancelCtx)
	if err != nil {
		serviceLogger.Error(err.Error())
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

func ExecuteBinaryRequestUpload(datagram structs.Datagram) *FilesUploadRequest {
	data := FilesUploadRequest{}
	structs.MarshalUnmarshal(&datagram, &data)
	return &data
}

func K8sNotification(d structs.Datagram) interface{} {
	serviceLogger.Info("Received pattern", "pattern", d.Pattern)
	return nil
}

func execShConnection(podCmdConnectionRequest xterm.PodCmdConnectionRequest) {
	// allows to execute itself without being in $PATH (e.g. while developing locally)
	self, err := os.Executable()
	if err != nil {
		serviceLogger.Error("failed to get current executable path", "error", err)
		return
	}

	cmd := exec.Command(
		self,
		"exec",
		"--namespace",
		podCmdConnectionRequest.Namespace,
		"--pod",
		podCmdConnectionRequest.Pod,
		"--container",
		podCmdConnectionRequest.Container,
		"--",
		"sh",
	)

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

	pod := kubernetes.PodStatus(podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod, false)
	terminatedState := kubernetes.LastTerminatedStateIfAny(pod)

	var previousRestReq *rest.Request
	if terminatedState != nil {
		tmpPreviousResReq, err := PreviousPodLogStream(podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod)
		if err != nil {
			serviceLogger.Error(err.Error())
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
		serviceLogger.Error(err.Error())
		previousStream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
	} else {
		previousStream = tmpPreviousStream
	}

	data, err := io.ReadAll(previousStream)
	if err != nil {
		serviceLogger.Error("failed to read data", "error", err)
	}

	lastState := kubernetes.LastTerminatedStateToString(terminatedState)

	nl := strings.NewReader("\r\n")
	previousState := strings.NewReader(lastState)
	headlineLastLog := strings.NewReader("Last Log:\r\n")
	headlineCurrentLog := strings.NewReader("\r\nCurrent Log:\r\n")

	return io.MultiReader(previousState, nl, headlineLastLog, strings.NewReader(string(data)), nl, headlineCurrentLog)
}

func logStreamConnection(podCmdConnectionRequest xterm.PodCmdConnectionRequest) {
	self, err := os.Executable()
	if err != nil {
		serviceLogger.Error("failed to get current executable path", "error", err)
		return
	}

	cmd := exec.Command(
		self,
		"logs",
		"--namespace",
		podCmdConnectionRequest.Namespace,
		"--pod",
		podCmdConnectionRequest.Pod,
		"--container",
		podCmdConnectionRequest.Container,
		"--tail-lines",
		podCmdConnectionRequest.LogTail,
	)

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

func componentLogStreamConnection(componentLogConnectionRequest xterm.ComponentLogConnectionRequest) {
	xterm.XTermComponentStreamConnection(
		componentLogConnectionRequest.WsConnection,
		componentLogConnectionRequest.Component,
		componentLogConnectionRequest.Namespace,
		componentLogConnectionRequest.Controller,
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
