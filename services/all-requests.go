package services

import (
	// "bufio"

	"context"
	"fmt"
	"io"
	"strings"

	// "fmt"
	"mogenius-k8s-manager/builder"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"net/url"

	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"
	punqStructs "github.com/mogenius/punq/structs"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func ExecuteCommandRequest(datagram structs.Datagram) interface{} {
	switch datagram.Pattern {
	case PAT_K8SNOTIFICATION:
		return K8sNotification(datagram)
	case PAT_CLUSTERSTATUS:
		return punq.ClusterStatus(nil)
	case PAT_CLUSTERRESOURCEINFO:
		nodeStats := punq.GetNodeStats(nil)
		loadBalancerExternalIps := punq.GetClusterExternalIps(nil)
		result := punqDtos.ClusterResourceInfoDto{
			NodeStats:               nodeStats,
			LoadBalancerExternalIps: loadBalancerExternalIps,
		}
		return result
	case PAT_UPGRADEK8SMANAGER:
		data := K8sManagerUpgradeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return UpgradeK8sManager(data)

	case PAT_CLUSTER_FORCE_RECONNECT:
		return kubernetes.ClusterForceReconnect()

	case PAT_FILES_LIST:
		data := FilesListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return List(data)
	case PAT_FILES_CREATE_FOLDER:
		data := FilesCreateFolderRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return CreateFolder(data)
	case PAT_FILES_RENAME:
		data := FilesRenameRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return Rename(data)
	case PAT_FILES_CHOWN:
		data := FilesChownRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return Chown(data)
	case PAT_FILES_CHMOD:
		data := FilesChmodRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return Chmod(data)
	case PAT_FILES_DELETE:
		data := FilesDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return Delete(data)
	case PAT_FILES_DOWNLOAD:
		data := FilesDownloadRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return Download(data)

	case PAT_CLUSTER_EXECUTE_HELM_CHART_TASK:
		data := ClusterHelmRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return InstallHelmChart(data)
	case PAT_CLUSTER_UNINSTALL_HELM_CHART:
		data := ClusterHelmUninstallRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return DeleteHelmChart(data)
	case PAT_CLUSTER_TCP_UDP_CONFIGURATION:
		return TcpUdpClusterConfiguration()

	case PAT_NAMESPACE_CREATE:
		data := NamespaceCreateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return CreateNamespace(data)
	case PAT_NAMESPACE_DELETE:
		data := NamespaceDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return DeleteNamespace(data)
	case PAT_NAMESPACE_SHUTDOWN:
		data := NamespaceShutdownRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return ShutdownNamespace(data)
	case PAT_NAMESPACE_POD_IDS:
		data := NamespacePodIdsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return PodIds(data)
	case PAT_NAMESPACE_VALIDATE_CLUSTER_PODS:
		data := NamespaceValidateClusterPodsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return ValidateClusterPods(data)
	case PAT_NAMESPACE_VALIDATE_PORTS:
		data := NamespaceValidatePortsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return ValidateClusterPorts(data)
	case PAT_NAMESPACE_LIST_ALL:
		return ListAllNamespaces()
	case PAT_NAMESPACE_GATHER_ALL_RESOURCES:
		data := NamespaceGatherAllResourcesRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return ListAllResourcesForNamespace(data)
	case PAT_NAMESPACE_BACKUP:
		data := NamespaceBackupRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := kubernetes.BackupNamespace(data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case PAT_NAMESPACE_RESTORE:
		data := NamespaceRestoreRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		result, err := kubernetes.RestoreNamespace(data.YamlData, data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case PAT_SERVICE_CREATE:
		data := ServiceCreateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		data.Service.ApplyDefaults()
		return CreateService(data)
	case PAT_SERVICE_DELETE:
		data := ServiceDeleteRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		data.Service.ApplyDefaults()
		return DeleteService(data)
	case PAT_SERVICE_POD_IDS:
		data := ServiceGetPodIdsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return ServicePodIds(data)
	case PAT_SERVICE_POD_EXISTS:
		data := ServicePodExistsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return ServicePodExists(data)
	case PAT_SERVICE_PODS:
		data := ServicePodsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return ServicePodStatus(data)
	case PAT_SERVICE_SET_IMAGE:
		data := ServiceSetImageRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		data.ApplyDefaults()
		return SetImage(data)
	case PAT_SERVICE_LOG:
		data := ServiceGetLogRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return PodLog(data)
	case PAT_SERVICE_LOG_ERROR:
		data := ServiceGetLogRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return PodLogError(data)
	case PAT_SERVICE_RESOURCE_STATUS:
		data := ServiceResourceStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return PodStatus(data)
	case PAT_SERVICE_RESTART:
		data := ServiceRestartRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		data.Service.ApplyDefaults()
		return Restart(data)
	case PAT_SERVICE_STOP:
		data := ServiceStopRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		data.Service.ApplyDefaults()
		return StopService(data)
	case PAT_SERVICE_START:
		data := ServiceStartRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		data.Service.ApplyDefaults()
		return StartService(data)
	case PAT_SERVICE_UPDATE_SERVICE:
		data := ServiceUpdateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		data.Service.ApplyDefaults()
		return UpdateService(data)
	case PAT_SERVICE_TRIGGER_JOB:
		data := ServiceTriggerJobRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		data.Service.ApplyDefaults()
		return TriggerJobService(data)

	case PAT_SERVICE_LOG_STREAM:
		data := ServiceLogStreamRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return logStream(data, datagram)

	case PAT_LIST_CREATE_TEMPLATES:
		return punq.ListCreateTemplates()

	case PAT_LIST_NAMESPACES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.ListK8sNamespaces(data.NamespaceName, nil)
	case PAT_LIST_DEPLOYMENTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sDeployments(data.NamespaceName, nil)
	case PAT_LIST_SERVICES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sServices(data.NamespaceName, nil)
	case PAT_LIST_PODS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sPods(data.NamespaceName, nil)
	case PAT_LIST_INGRESSES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sIngresses(data.NamespaceName, nil)
	case PAT_LIST_CONFIGMAPS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sConfigmaps(data.NamespaceName, nil)
	case PAT_LIST_SECRETS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sSecrets(data.NamespaceName, nil)
	case PAT_LIST_NODES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.ListK8sNodes(nil)
	case PAT_LIST_DAEMONSETS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sDaemonsets(data.NamespaceName, nil)
	case PAT_LIST_STATEFULSETS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllStatefulSets(data.NamespaceName, nil)
	case PAT_LIST_JOBS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllJobs(data.NamespaceName, nil)
	case PAT_LIST_CRONJOBS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllCronjobs(data.NamespaceName, nil)
	case PAT_LIST_REPLICASETS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sReplicasets(data.NamespaceName, nil)
	case PAT_LIST_PERSISTENT_VOLUMES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllPersistentVolumes(nil)
	case PAT_LIST_PERSISTENT_VOLUME_CLAIMS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sPersistentVolumeClaims(data.NamespaceName, nil)
	case PAT_LIST_HORIZONTAL_POD_AUTOSCALERS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllHpas(data.NamespaceName, nil)
	case PAT_LIST_EVENTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllEvents(data.NamespaceName, nil)
	case PAT_LIST_CERTIFICATES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllK8sCertificates(data.NamespaceName, nil)
	case PAT_LIST_CERTIFICATEREQUESTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllCertificateSigningRequests(data.NamespaceName, nil)
	case PAT_LIST_ORDERS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllOrders(data.NamespaceName, nil)
	case PAT_LIST_ISSUERS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllIssuer(data.NamespaceName, nil)
	case PAT_LIST_CLUSTERISSUERS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllClusterIssuers(nil)
	case PAT_LIST_SERVICE_ACCOUNT:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllServiceAccounts(data.NamespaceName, nil)
	case PAT_LIST_ROLE:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllRoles(data.NamespaceName, nil)
	case PAT_LIST_ROLE_BINDING:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllRoleBindings(data.NamespaceName, nil)
	case PAT_LIST_CLUSTER_ROLE:
		return punq.AllClusterRoles(nil)
	case PAT_LIST_CLUSTER_ROLE_BINDING:
		return punq.AllClusterRoleBindings(nil)
	case PAT_LIST_VOLUME_ATTACHMENT:
		return punq.AllVolumeAttachments(nil)
	case PAT_LIST_NETWORK_POLICY:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllNetworkPolicies(data.NamespaceName, nil)
	case PAT_LIST_STORAGE_CLASS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllStorageClasses(nil)
	case PAT_LIST_CUSTOM_RESOURCE_DEFINITIONS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		// TODO: sdk not giving crds. on hold
		return nil
	case PAT_LIST_ENDPOINTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllEndpoints(data.NamespaceName, nil)
	case PAT_LIST_LEASES:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllLeases(data.NamespaceName, nil)
	case PAT_LIST_PRIORITYCLASSES:
		return punq.AllPriorityClasses(nil)
	case PAT_LIST_VOLUMESNAPSHOTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		// TODO: sdk not giving crds. on hold
		return nil
	case PAT_LIST_RESOURCEQUOTAS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.AllResourceQuotas(data.NamespaceName, nil)

	case PAT_DESCRIBE_NAMESPACE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sNamespace(data.ResourceName, nil)
	case PAT_DESCRIBE_DEPLOYMENT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sDeployment(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_SERVICE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sService(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_POD:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sPod(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_INGRESS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sIngress(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_CONFIGMAP:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sConfigmap(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_SECRET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sSecret(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_NODE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sNode(data.ResourceName, nil)
	case PAT_DESCRIBE_DAEMONSET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sDaemonSet(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_STATEFULSET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sStatefulset(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_JOB:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sJob(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_CRONJOB:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sCronJob(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_REPLICASET:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sReplicaset(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_PERSISTENT_VOLUME:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sPersistentVolume(data.ResourceName, nil)
	case PAT_DESCRIBE_PERSISTENT_VOLUME_CLAIM:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sPersistentVolumeClaim(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_HORIZONTAL_POD_AUTOSCALER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sHpa(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_EVENT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sEvent(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_CERTIFICATE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sCertificate(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_CERTIFICATEREQUEST:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sCertificateSigningRequest(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_ORDER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sOrder(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_ISSUER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sIssuer(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_CLUSTERISSUER:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sClusterIssuer(data.ResourceName, nil)
	case PAT_DESCRIBE_SERVICE_ACCOUNT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sServiceAccount(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_ROLE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sRole(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_ROLE_BINDING:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sRoleBinding(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_CLUSTER_ROLE:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sClusterRole(data.ResourceName, nil)
	case PAT_DESCRIBE_CLUSTER_ROLE_BINDING:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sClusterRoleBinding(data.ResourceName, nil)
	case PAT_DESCRIBE_VOLUME_ATTACHMENT:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sVolumeAttachment(data.ResourceName, nil)
	case PAT_DESCRIBE_NETWORK_POLICY:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sNetworkPolicy(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_STORAGE_CLASS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sStorageClass(data.ResourceName, nil)
	case PAT_DESCRIBE_CUSTOM_RESOURCE_DEFINITIONS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sCustomResourceDefinition(data.ResourceName, nil)
	case PAT_DESCRIBE_ENDPOINTS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sEndpoint(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_LEASES:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sLease(data.NamespaceName, data.ResourceName, nil)
	case PAT_DESCRIBE_PRIORITYCLASSES:
		return punq.AllPriorityClasses(nil)
	case PAT_DESCRIBE_VOLUMESNAPSHOTS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		// TODO: sdk not giving crds. on hold
		return nil
	case PAT_DESCRIBE_RESOURCEQUOTAS:
		data := K8sDescribeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DescribeK8sResourceQuota(data.NamespaceName, data.ResourceName, nil)

	case PAT_UPDATE_DEPLOYMENT:
		data := K8sUpdateDeploymentRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateDeployment(data)
	case PAT_UPDATE_SERVICE:
		data := K8sUpdateServiceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateService(data)
	case PAT_UPDATE_POD:
		data := K8sUpdatePodRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdatePod(data)
	case PAT_UPDATE_INGRESS:
		data := K8sUpdateIngressRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateIngress(data)
	case PAT_UPDATE_CONFIGMAP:
		data := K8sUpdateConfigmapRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateConfigMap(data)
	case PAT_UPDATE_SECRET:
		data := K8sUpdateSecretRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateSecret(data)
	case PAT_UPDATE_DAEMONSET:
		data := K8sUpdateDaemonSetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateDaemonSet(data)
	case PAT_UPDATE_STATEFULSET:
		data := K8sUpdateStatefulSetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateStatefulset(data)
	case PAT_UPDATE_JOB:
		data := K8sUpdateJobRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateJob(data)
	case PAT_UPDATE_CRONJOB:
		data := K8sUpdateCronJobRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateCronJob(data)
	case PAT_UPDATE_REPLICASET:
		data := K8sUpdateReplicaSetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sUpdateReplicaSet(data)
	case PAT_UPDATE_PERSISTENT_VOLUME:
		data := K8sUpdatePersistentVolumeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sPersistentVolume(*data.Data, nil)
	case PAT_UPDATE_PERSISTENT_VOLUME_CLAIM:
		data := K8sUpdatePersistentVolumeClaimRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sPersistentVolumeClaim(*data.Data, nil)
	case PAT_UPDATE_HORIZONTAL_POD_AUTOSCALERS:
		data := K8sUpdateHPARequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sHpa(*data.Data, nil)
	case PAT_UPDATE_CERTIFICATES:
		data := K8sUpdateCertificateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sCertificate(*data.Data, nil)
	case PAT_UPDATE_CERTIFICATEREQUESTS:
		data := K8sUpdateCertificateRequestRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sCertificateSigningRequest(*data.Data, nil)
	case PAT_UPDATE_ORDERS:
		data := K8sUpdateOrderRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sOrder(*data.Data, nil)
	case PAT_UPDATE_ISSUERS:
		data := K8sUpdateIssuerRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sIssuer(*data.Data, nil)
	case PAT_UPDATE_CLUSTERISSUERS:
		data := K8sUpdateClusterIssuerRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sClusterIssuer(*data.Data, nil)
	case PAT_UPDATE_SERVICE_ACCOUNT:
		data := K8sUpdateServiceAccountRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sServiceAccount(*data.Data, nil)
	case PAT_UPDATE_ROLE:
		data := K8sUpdateRoleRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sRole(*data.Data, nil)
	case PAT_UPDATE_ROLE_BINDING:
		data := K8sUpdateRoleBindingRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sRoleBinding(*data.Data, nil)
	case PAT_UPDATE_CLUSTER_ROLE:
		data := K8sUpdateClusterRoleRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sClusterRole(*data.Data, nil)
	case PAT_UPDATE_CLUSTER_ROLE_BINDING:
		data := K8sUpdateClusterRoleBindingRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sClusterRoleBinding(*data.Data, nil)
	case PAT_UPDATE_VOLUME_ATTACHMENT:
		data := K8sUpdateVolumeAttachmentRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sVolumeAttachment(*data.Data, nil)
	case PAT_UPDATE_NETWORK_POLICY:
		data := K8sUpdateNetworkPolicyRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sNetworkPolicy(*data.Data, nil)
	case PAT_UPDATE_STORAGE_CLASS:
		data := K8sUpdateStorageClassRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sStorageClass(*data.Data, nil)
	case PAT_UPDATE_CUSTOM_RESOURCE_DEFINITIONS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		// TODO: sdk not giving crds. on hold
		return nil
	case PAT_UPDATE_ENDPOINTS:
		data := K8sUpdateEndpointRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sEndpoint(*data.Data, nil)
	case PAT_UPDATE_LEASES:
		data := K8sUpdateLeaseRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sLease(*data.Data, nil)
	case PAT_UPDATE_PRIORITYCLASSES:
		data := K8sUpdatePriorityClassRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sPriorityClass(*data.Data, nil)
	case PAT_UPDATE_VOLUMESNAPSHOTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		// TODO: sdk not giving crds. on hold
		return nil
	case PAT_UPDATE_RESOURCEQUOTAS:
		data := K8sUpdateResourceQuotaRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.UpdateK8sResourceQuota(*data.Data, nil)

	case PAT_DELETE_NAMESPACE:
		data := K8sDeleteNamespaceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteNamespace(data)
	case PAT_DELETE_DEPLOYMENT:
		data := K8sDeleteDeploymentRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteDeployment(data)
	case PAT_DELETE_SERVICE:
		data := K8sDeleteServiceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteService(data)
	case PAT_DELETE_POD:
		data := K8sDeletePodRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeletePod(data)
	case PAT_DELETE_INGRESS:
		data := K8sDeleteIngressRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteIngress(data)
	case PAT_DELETE_CONFIGMAP:
		data := K8sDeleteConfigmapRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteConfigMap(data)
	case PAT_DELETE_SECRET:
		data := K8sDeleteSecretRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteSecret(data)
	case PAT_DELETE_DAEMONSET:
		data := K8sDeleteDaemonsetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteDaemonSet(data)
	case PAT_DELETE_STATEFULSET:
		data := K8sDeleteStatefulsetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteStatefulset(data)
	case PAT_DELETE_JOB:
		data := K8sDeleteJobRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteJob(data)
	case PAT_DELETE_CRONJOB:
		data := K8sDeleteCronjobRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteCronJob(data)
	case PAT_DELETE_REPLICASET:
		data := K8sDeleteReplicasetRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return K8sDeleteReplicaSet(data)
	case PAT_DELETE_PERSISTENT_VOLUME:
		data := K8sDeletePersistentVolumeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sPersistentVolume(*data.Data, nil)
	case PAT_DELETE_PERSISTENT_VOLUME_CLAIM:
		data := K8sDeletePersistentVolumeClaimRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sPersistentVolumeClaim(*data.Data, nil)
	case PAT_DELETE_HORIZONTAL_POD_AUTOSCALERS:
		data := K8sDeleteHPARequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sHpa(*data.Data, nil)
	case PAT_DELETE_CERTIFICATES:
		data := K8sDeleteCertificateRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sCertificate(*data.Data, nil)
	case PAT_DELETE_CERTIFICATEREQUESTS:
		data := K8sDeleteCertificateRequestRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sCertificateSigningRequest(*data.Data, nil)
	case PAT_DELETE_ORDERS:
		data := K8sDeleteOrderRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sOrder(*data.Data, nil)
	case PAT_DELETE_ISSUERS:
		data := K8sDeleteIssuerRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sIssuer(*data.Data, nil)
	case PAT_DELETE_CLUSTERISSUERS:
		data := K8sDeleteClusterIssuerRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sClusterIssuer(*data.Data, nil)
	case PAT_DELETE_SERVICE_ACCOUNT:
		data := K8sDeleteServiceAccountRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sServiceAccount(*data.Data, nil)
	case PAT_DELETE_ROLE:
		data := K8sDeleteRoleRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sRole(*data.Data, nil)
	case PAT_DELETE_ROLE_BINDING:
		data := K8sDeleteRoleBindingRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sRoleBinding(*data.Data, nil)
	case PAT_DELETE_CLUSTER_ROLE:
		data := K8sDeleteClusterRoleRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sClusterRole(*data.Data, nil)
	case PAT_DELETE_CLUSTER_ROLE_BINDING:
		data := K8sDeleteClusterRoleBindingRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sClusterRoleBinding(*data.Data, nil)
	case PAT_DELETE_VOLUME_ATTACHMENT:
		data := K8sDeleteVolumeAttachmentRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sVolumeAttachment(*data.Data, nil)
	case PAT_DELETE_NETWORK_POLICY:
		data := K8sDeleteNetworkPolicyRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sNetworkPolicy(*data.Data, nil)
	case PAT_DELETE_STORAGE_CLASS:
		data := K8sDeleteStorageClassRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sStorageClass(*data.Data, nil)
	case PAT_DELETE_CUSTOM_RESOURCE_DEFINITIONS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		// TODO: sdk not giving crds. on hold
		return nil
	case PAT_DELETE_ENDPOINTS:
		data := K8sDeleteEndpointRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sEndpoint(*data.Data, nil)
	case PAT_DELETE_LEASES:
		data := K8sDeleteLeaseRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sLease(*data.Data, nil)
	case PAT_DELETE_PRIORITYCLASSES:
		data := K8sDeletePriorityClassRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sPriorityClass(*data.Data, nil)
	case PAT_DELETE_VOLUMESNAPSHOTS:
		data := K8sListRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		// TODO: sdk not giving crds. on hold
		return nil
	case PAT_DELETE_RESOURCEQUOTAS:
		data := K8sDeleteResourceQuotaRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return punq.DeleteK8sResourceQuota(*data.Data, nil)

	case PAT_BUILDER_STATUS:
		return builder.BuilderStatus()
	case PAT_BUILD_INFOS:
		data := structs.BuildJobStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return builder.BuildJobInfos(data.BuildId)
	case PAT_BUILD_LIST_ALL:
		return builder.ListAll()
	case PAT_BUILD_LIST_BY_PROJECT:
		data := structs.ListBuildByProjectIdRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return builder.ListByProjectId(data.ProjectId)
	case PAT_BUILD_ADD:
		data := structs.BuildJob{}
		structs.MarshalUnmarshal(&datagram, &data)
		return builder.Add(data)
	case PAT_BUILD_SCAN:
		data := structs.BuildJob{}
		structs.MarshalUnmarshal(&datagram, &data)
		return builder.Scan(data, true)
	case PAT_BUILD_CANCEL:
		data := structs.BuildJob{}
		structs.MarshalUnmarshal(&datagram, &data)
		return builder.Cancel(data.BuildId)
	case PAT_BUILD_DELETE:
		data := structs.BuildJobStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return builder.Delete(data.BuildId)
	case PAT_BUILD_LAST_JOB_OF_SERVICES:
		data := structs.BuildServicesStatusRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return builder.LastNJobsPerServices(data.MaxResults, data.ServiceIds)
	case PAT_BUILD_JOB_LIST_OF_SERVICE:
		data := structs.BuildServiceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return builder.LastNJobsPerService(data.MaxResults, data.ServiceId)
	case PAT_BUILD_LAST_JOB_INFO_OF_SERVICE:
		data := structs.BuildServiceRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return builder.LastBuildForService(data.ServiceId)

	case PAT_EXEC_SHELL:
		// data := structs.BuildJobStatusRequest{}
		// structs.MarshalUnmarshal(&datagram, &data)
		return kubernetes.ExecTest()

	case PAT_STORAGE_CREATE_VOLUME:
		data := NfsVolumeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return CreateMogeniusNfsVolume(data)
	case PAT_STORAGE_DELETE_VOLUME:
		data := NfsVolumeRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return DeleteMogeniusNfsVolume(data)
	case PAT_STORAGE_BACKUP_VOLUME:
		data := NfsVolumeBackupRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return BackupMogeniusNfsVolume(data)
	case PAT_STORAGE_RESTORE_VOLUME:
		data := NfsVolumeRestoreRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return RestoreMogeniusNfsVolume(data)
	case PAT_STORAGE_STATS:
		data := NfsVolumeStatsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return StatsMogeniusNfsVolume(data)
	case PAT_STORAGE_NAMESPACE_STATS:
		data := NfsNamespaceStatsRequest{}
		structs.MarshalUnmarshal(&datagram, &data)
		return StatsMogeniusNfsNamespace(data)
	case PAT_POPEYE_CONSOLE:
		return PopeyeConsole()
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
		logger.Log.Error(result.Error)
		return result
	}

	pod := punq.PodStatus(data.Namespace, data.PodId, false, nil)
	terminatedState := punq.LastTerminatedStateIfAny(pod)

	var previousResReq *rest.Request
	if terminatedState != nil {
		tmpPreviousResReq, err := PreviousPodLogStream(data)
		if err != nil {
			logger.Log.Error(err.Error())
		} else {
			previousResReq = tmpPreviousResReq
		}
	}

	restReq, err := PodLogStream(data)
	if err != nil {
		result.Error = err.Error()
		result.Success = false
		logger.Log.Error(result.Error)
		return result
	}

	if terminatedState != nil {
		logger.Log.Infof("Logger try multiStreamData")
		go multiStreamData(previousResReq, restReq, terminatedState, url.String())
	} else {
		logger.Log.Infof("Logger try streamData")
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
		logger.Log.Error(err.Error())
	}

	structs.SendDataWs(toServerUrl, stream)
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
			logger.Log.Error(err.Error())
			previousStream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
		} else {
			previousStream = tmpPreviousStream
		}
	}

	stream, err := restReq.Stream(cancelCtx)
	if err != nil {
		logger.Log.Error(err.Error())
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
	return punqStructs.ExecuteBashCommandWithResponse("Generate popeye report", "popeye")
}

func ExecuteBinaryRequestUpload(datagram structs.Datagram) *FilesUploadRequest {
	data := FilesUploadRequest{}
	structs.MarshalUnmarshal(&datagram, &data)
	return &data
}

func K8sNotification(d structs.Datagram) interface{} {
	logger.Log.Infof("Received '%s'.", d.Pattern)
	return nil
}
