package services

import (
	// "bufio"

	"context"
	"io"
	"strings"

	// "fmt"
	"mogenius-k8s-manager/dtos"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"net/url"

	jsoniter "github.com/json-iterator/go"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func ExecuteCommandRequest(datagram structs.Datagram) interface{} {
	switch datagram.Pattern {
	case PAT_K8SNOTIFICATION:
		return K8sNotification(datagram)
	case PAT_CLUSTERSTATUS:
		return mokubernetes.ClusterStatus()
	case PAT_CLUSTERRESOURCEINFO:
		nodeStats := mokubernetes.GetNodeStats()
		loadBalancerExternalIps := mokubernetes.GetClusterExternalIps()
		result := dtos.ClusterResourceInfoDto{
			NodeStats:               nodeStats,
			LoadBalancerExternalIps: loadBalancerExternalIps,
		}
		return result
	case PAT_UPGRADEK8SMANAGER:
		data := K8sManagerUpgradeRequest{}
		marshalUnmarshal(&datagram, &data)
		return UpgradeK8sManager(data)
	case PAT_FILES_LIST:
		data := FilesListRequest{}
		marshalUnmarshal(&datagram, &data)
		return List(data)
	case PAT_FILES_CREATE_FOLDER:
		data := FilesCreateFolderRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateFolder(data)
	case PAT_FILES_RENAME:
		data := FilesRenameRequest{}
		marshalUnmarshal(&datagram, &data)
		return Rename(data)
	case PAT_FILES_CHOWN:
		data := FilesChownRequest{}
		marshalUnmarshal(&datagram, &data)
		return Chown(data)
	case PAT_FILES_CHMOD:
		data := FilesChmodRequest{}
		marshalUnmarshal(&datagram, &data)
		return Chmod(data)
	case PAT_FILES_DELETE:
		data := FilesDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return Delete(data)
	case PAT_FILES_DOWNLOAD:
		data := FilesDownloadRequest{}
		marshalUnmarshal(&datagram, &data)
		return Download(data)

	case PAT_CLUSTER_EXECUTE_HELM_CHART_TASK:
		data := ClusterHelmRequest{}
		marshalUnmarshal(&datagram, &data)
		return InstallHelmChart(data)
	case PAT_CLUSTER_UNINSTALL_HELM_CHART:
		data := ClusterHelmUninstallRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteHelmChart(data)
	case PAT_CLUSTER_TCP_UDP_CONFIGURATION:
		return TcpUdpClusterConfiguration()

	case PAT_NAMESPACE_CREATE:
		data := NamespaceCreateRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateNamespace(data)
	case PAT_NAMESPACE_DELETE:
		data := NamespaceDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteNamespace(data)
	case PAT_NAMESPACE_SHUTDOWN:
		data := NamespaceShutdownRequest{}
		marshalUnmarshal(&datagram, &data)
		return ShutdownNamespace(data)
	case PAT_NAMESPACE_POD_IDS:
		data := NamespacePodIdsRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodIds(data)
	case PAT_NAMESPACE_VALIDATE_CLUSTER_PODS:
		data := NamespaceValidateClusterPodsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ValidateClusterPods(data)
	case PAT_NAMESPACE_VALIDATE_PORTS:
		data := NamespaceValidatePortsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ValidateClusterPorts(data)
	case PAT_NAMESPACE_LIST_ALL:
		return ListAllNamespaces()
	case PAT_NAMESPACE_GATHER_ALL_RESOURCES:
		data := NamespaceGatherAllResourcesRequest{}
		marshalUnmarshal(&datagram, &data)
		return ListAllResourcesForNamespace(data)
	case PAT_NAMESPACE_BACKUP:
		data := NamespaceBackupRequest{}
		marshalUnmarshal(&datagram, &data)
		result, err := mokubernetes.BackupNamespace(data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case PAT_NAMESPACE_RESTORE:
		data := NamespaceRestoreRequest{}
		marshalUnmarshal(&datagram, &data)
		result, err := mokubernetes.RestoreNamespace(data.YamlData, data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case PAT_SERVICE_CREATE:
		data := ServiceCreateRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateService(data)
	case PAT_SERVICE_DELETE:
		data := ServiceDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteService(data)
	case PAT_SERVICE_POD_IDS:
		data := ServiceGetPodIdsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ServicePodIds(data)
	case PAT_SERVICE_POD_EXISTS:
		data := ServicePodExistsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ServicePodExists(data)
	case PAT_SERVICE_PODS:
		data := ServicePodsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ServicePodStatus(data)
	case PAT_SERVICE_SET_IMAGE:
		data := ServiceSetImageRequest{}
		marshalUnmarshal(&datagram, &data)
		return SetImage(data)
	case PAT_SERVICE_LOG:
		data := ServiceGetLogRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodLog(data)
	case PAT_SERVICE_LOG_ERROR:
		data := ServiceGetLogRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodLogError(data)
	case PAT_SERVICE_RESOURCE_STATUS:
		data := ServiceResourceStatusRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodStatus(data)
	case PAT_SERVICE_RESTART:
		data := ServiceRestartRequest{}
		marshalUnmarshal(&datagram, &data)
		return Restart(data)
	case PAT_SERVICE_STOP:
		data := ServiceStopRequest{}
		marshalUnmarshal(&datagram, &data)
		return StopService(data)
	case PAT_SERVICE_START:
		data := ServiceStartRequest{}
		marshalUnmarshal(&datagram, &data)
		return StartService(data)
	case PAT_SERVICE_UPDATE_SERVICE:
		data := ServiceUpdateRequest{}
		marshalUnmarshal(&datagram, &data)
		return UpdateService(data)

	case PAT_SERVICE_LOG_STREAM:
		data := ServiceLogStreamRequest{}
		marshalUnmarshal(&datagram, &data)
		return logStream(data, datagram)

	case PAT_LIST_CREATE_TEMPLATES:
		return mokubernetes.ListCreateTemplates()

	case PAT_LIST_NAMESPACES:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.ListK8sNamespaces(data.NamespaceName)
	case PAT_LIST_DEPLOYMENTS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sDeployments(data.NamespaceName)
	case PAT_LIST_SERVICES:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sServices(data.NamespaceName)
	case PAT_LIST_PODS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sPods(data.NamespaceName)
	case PAT_LIST_INGRESSES:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sIngresses(data.NamespaceName)
	case PAT_LIST_CONFIGMAPS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sConfigmaps(data.NamespaceName)
	case PAT_LIST_SECRETS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sSecrets(data.NamespaceName)
	case PAT_LIST_NODES:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.ListK8sNodes()
	case PAT_LIST_DAEMONSETS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sDaemonsets(data.NamespaceName)
	case PAT_LIST_STATEFULSETS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllStatefulSets(data.NamespaceName)
	case PAT_LIST_JOBS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllJobs(data.NamespaceName)
	case PAT_LIST_CRONJOBS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllCronjobs(data.NamespaceName)
	case PAT_LIST_REPLICASETS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sReplicasets(data.NamespaceName)
	case PAT_LIST_PERSISTENT_VOLUMES:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllPersistentVolumes()
	case PAT_LIST_PERSISTENT_VOLUME_CLAIMS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sPersistentVolumeClaims(data.NamespaceName)
	case PAT_LIST_HORIZONTAL_POD_AUTOSCALERS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllHpas(data.NamespaceName)
	case PAT_LIST_EVENTS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllEvents(data.NamespaceName)
	case PAT_LIST_CERTIFICATES:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sCertificates(data.NamespaceName)
	case PAT_LIST_CERTIFICATEREQUESTS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllCertificateSigningRequests(data.NamespaceName)
	case PAT_LIST_ORDERS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllOrders(data.NamespaceName)
	case PAT_LIST_ISSUERS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllIssuer(data.NamespaceName)
	case PAT_LIST_CLUSTERISSUERS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllClusterIssuers()
	case PAT_LIST_SERVICE_ACCOUNT:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllServiceAccounts(data.NamespaceName)
	case PAT_LIST_ROLE:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllRoles(data.NamespaceName)
	case PAT_LIST_ROLE_BINDING:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllRoleBindings(data.NamespaceName)
	case PAT_LIST_CLUSTER_ROLE:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllClusterRoles(data.NamespaceName)
	case PAT_LIST_CLUSTER_ROLE_BINDING:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllClusterRoleBindings(data.NamespaceName)
	case PAT_LIST_VOLUME_ATTACHMENT:
		return mokubernetes.AllVolumeAttachments()
	case PAT_LIST_NETWORK_POLICY:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllNetworkPolicies(data.NamespaceName)
	case PAT_LIST_STORAGE_CLASS:
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllStorageClasses()

	case PAT_DESCRIBE_NAMESPACE:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sNamespace(data.ResourceName)
	case PAT_DESCRIBE_DEPLOYMENT:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sDeployment(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_SERVICE:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sService(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_POD:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sPod(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_INGRESS:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sIngress(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_CONFIGMAP:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sConfigmap(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_SECRET:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sSecret(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_NODE:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sNode(data.ResourceName)
	case PAT_DESCRIBE_DAEMONSET:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sDaemonSet(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_STATEFULSET:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sStatefulset(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_JOB:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sJob(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_CRONJOB:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sCronJob(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_REPLICASET:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sReplicaset(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_PERSISTENT_VOLUME:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sPersistentVolume(data.ResourceName)
	case PAT_DESCRIBE_PERSISTENT_VOLUME_CLAIM:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sPersistentVolumeClaim(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_HORIZONTAL_POD_AUTOSCALER:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sHpa(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_EVENT:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sEvent(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_CERTIFICATE:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sCertificate(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_CERTIFICATEREQUEST:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sCertificateSigningRequest(data.ResourceName)
	case PAT_DESCRIBE_ORDER:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sOrder(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_ISSUER:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sIssuer(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_CLUSTERISSUER:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sClusterIssuer(data.ResourceName)
	case PAT_DESCRIBE_SERVICE_ACCOUNT:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sServiceAccount(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_ROLE:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sRole(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_ROLE_BINDING:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sRoleBinding(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_CLUSTER_ROLE:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sClusterRole(data.ResourceName)
	case PAT_DESCRIBE_CLUSTER_ROLE_BINDING:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sClusterRoleBinding(data.ResourceName)
	case PAT_DESCRIBE_VOLUME_ATTACHMENT:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sVolumeAttachment(data.ResourceName)
	case PAT_DESCRIBE_NETWORK_POLICY:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sNetworkPolicy(data.NamespaceName, data.ResourceName)
	case PAT_DESCRIBE_STORAGE_CLASS:
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sStorageClass(data.ResourceName)

	case PAT_UPDATE_DEPLOYMENT:
		data := K8sUpdateDeploymentRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateDeployment(data)
	case PAT_UPDATE_SERVICE:
		data := K8sUpdateServiceRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateService(data)
	case PAT_UPDATE_POD:
		data := K8sUpdatePodRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdatePod(data)
	case PAT_UPDATE_INGRESS:
		data := K8sUpdateIngressRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateIngress(data)
	case PAT_UPDATE_CONFIGMAP:
		data := K8sUpdateConfigmapRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateConfigMap(data)
	case PAT_UPDATE_SECRET:
		data := K8sUpdateSecretRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateSecret(data)
	case PAT_UPDATE_DAEMONSET:
		data := K8sUpdateDaemonSetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateDaemonSet(data)
	case PAT_UPDATE_STATEFULSET:
		data := K8sUpdateStatefulSetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateStatefulset(data)
	case PAT_UPDATE_JOB:
		data := K8sUpdateJobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateJob(data)
	case PAT_UPDATE_CRONJOB:
		data := K8sUpdateCronJobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateCronJob(data)
	case PAT_UPDATE_REPLICASET:
		data := K8sUpdateReplicaSetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateReplicaSet(data)
	case PAT_UPDATE_PERSISTENT_VOLUME:
		data := K8sUpdatePersistentVolumeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sPersistentVolume(*data.Data)
	case PAT_UPDATE_PERSISTENT_VOLUME_CLAIM:
		data := K8sUpdatePersistentVolumeClaimRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sPersistentVolumeClaim(*data.Data)
	case PAT_UPDATE_HORIZONTAL_POD_AUTOSCALERS:
		data := K8sUpdateHPARequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sHpa(*data.Data)
	case PAT_UPDATE_CERTIFICATES:
		data := K8sUpdateCertificateRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sCertificate(*data.Data)
	case PAT_UPDATE_CERTIFICATEREQUESTS:
		data := K8sUpdateCertificateRequestRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sCertificateSigningRequest(*data.Data)
	case PAT_UPDATE_ORDERS:
		data := K8sUpdateOrderRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sOrder(*data.Data)
	case PAT_UPDATE_ISSUERS:
		data := K8sUpdateIssuerRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sIssuer(*data.Data)
	case PAT_UPDATE_CLUSTERISSUERS:
		data := K8sUpdateClusterIssuerRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sClusterIssuer(*data.Data)
	case PAT_UPDATE_SERVICE_ACCOUNT:
		data := K8sUpdateServiceAccountRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sServiceAccount(*data.Data)
	case PAT_UPDATE_ROLE:
		data := K8sUpdateRoleRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sRole(*data.Data)
	case PAT_UPDATE_ROLE_BINDING:
		data := K8sUpdateRoleBindingRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sRoleBinding(*data.Data)
	case PAT_UPDATE_CLUSTER_ROLE:
		data := K8sUpdateClusterRoleRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sClusterRole(*data.Data)
	case PAT_UPDATE_CLUSTER_ROLE_BINDING:
		data := K8sUpdateClusterRoleBindingRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sClusterRoleBinding(*data.Data)
	case PAT_UPDATE_VOLUME_ATTACHMENT:
		data := K8sUpdateVolumeAttachmentRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sVolumeAttachment(*data.Data)
	case PAT_UPDATE_NETWORK_POLICY:
		data := K8sUpdateNetworkPolicyRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sNetworkPolicy(*data.Data)
	case PAT_UPDATE_STORAGE_CLASS:
		data := K8sUpdateStorageClassRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sStorageClass(*data.Data)

	case PAT_DELETE_NAMESPACE:
		data := K8sDeleteNamespaceRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteNamespace(data)
	case PAT_DELETE_DEPLOYMENT:
		data := K8sDeleteDeploymentRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteDeployment(data)
	case PAT_DELETE_SERVICE:
		data := K8sDeleteServiceRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteService(data)
	case PAT_DELETE_POD:
		data := K8sDeletePodRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeletePod(data)
	case PAT_DELETE_INGRESS:
		data := K8sDeleteIngressRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteIngress(data)
	case PAT_DELETE_CONFIGMAP:
		data := K8sDeleteConfigmapRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteConfigMap(data)
	case PAT_DELETE_SECRET:
		data := K8sDeleteSecretRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteSecret(data)
	case PAT_DELETE_DAEMONSET:
		data := K8sDeleteDaemonsetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteDaemonSet(data)
	case PAT_DELETE_STATEFULSET:
		data := K8sDeleteStatefulsetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteStatefulset(data)
	case PAT_DELETE_JOB:
		data := K8sDeleteJobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteJob(data)
	case PAT_DELETE_CRONJOB:
		data := K8sDeleteCronjobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteCronJob(data)
	case PAT_DELETE_REPLICASET:
		data := K8sDeleteReplicasetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteReplicaSet(data)
	case PAT_DELETE_PERSISTENT_VOLUME:
		data := K8sDeletePersistentVolumeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sPersistentVolume(*data.Data)
	case PAT_DELETE_PERSISTENT_VOLUME_CLAIM:
		data := K8sDeletePersistentVolumeClaimRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sPersistentVolumeClaim(*data.Data)
	case PAT_DELETE_HORIZONTAL_POD_AUTOSCALERS:
		data := K8sDeleteHPARequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sHpa(*data.Data)
	case PAT_DELETE_CERTIFICATES:
		data := K8sDeleteCertificateRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sCertificate(*data.Data)
	case PAT_DELETE_CERTIFICATEREQUESTS:
		data := K8sDeleteCertificateRequestRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sCertificateSigningRequest(*data.Data)
	case PAT_DELETE_ORDERS:
		data := K8sDeleteOrderRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sOrder(*data.Data)
	case PAT_DELETE_ISSUERS:
		data := K8sDeleteIssuerRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sIssuer(*data.Data)
	case PAT_DELETE_CLUSTERISSUERS:
		data := K8sDeleteClusterIssuerRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sClusterIssuer(*data.Data)
	case PAT_DELETE_SERVICE_ACCOUNT:
		data := K8sDeleteServiceAccountRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sServiceAccount(*data.Data)
	case PAT_DELETE_ROLE:
		data := K8sDeleteRoleRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sRole(*data.Data)
	case PAT_DELETE_ROLE_BINDING:
		data := K8sDeleteRoleBindingRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sRoleBinding(*data.Data)
	case PAT_DELETE_CLUSTER_ROLE:
		data := K8sDeleteClusterRoleRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sClusterRole(*data.Data)
	case PAT_DELETE_CLUSTER_ROLE_BINDING:
		data := K8sDeleteClusterRoleBindingRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sClusterRoleBinding(*data.Data)
	case PAT_DELETE_VOLUME_ATTACHMENT:
		data := K8sDeleteVolumeAttachmentRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sVolumeAttachment(*data.Data)
	case PAT_DELETE_NETWORK_POLICY:
		data := K8sDeleteNetworkPolicyRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sNetworkPolicy(*data.Data)
	case PAT_DELETE_STORAGE_CLASS:
		data := K8sDeleteStorageClassRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sStorageClass(*data.Data)

	case PAT_STORAGE_CREATE_VOLUME:
		data := NfsVolumeRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateMogeniusNfsVolume(data)
	case PAT_STORAGE_DELETE_VOLUME:
		data := NfsVolumeRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteMogeniusNfsVolume(data)
	case PAT_STORAGE_BACKUP_VOLUME:
		data := NfsVolumeBackupRequest{}
		marshalUnmarshal(&datagram, &data)
		return BackupMogeniusNfsVolume(data)
	case PAT_STORAGE_RESTORE_VOLUME:
		data := NfsVolumeRestoreRequest{}
		marshalUnmarshal(&datagram, &data)
		return RestoreMogeniusNfsVolume(data)
	case PAT_STORAGE_STATS:
		data := NfsVolumeStatsRequest{}
		marshalUnmarshal(&datagram, &data)
		return StatsMogeniusNfsVolume(data)
	case PAT_STORAGE_NAMESPACE_STATS:
		data := NfsNamespaceStatsRequest{}
		marshalUnmarshal(&datagram, &data)
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

	pod := mokubernetes.PodStatus(data.Namespace, data.PodId, false)
	terminatedState := mokubernetes.LastTerminatedStateIfAny(pod)

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

	lastState := mokubernetes.LastTerminatedStateToString(terminatedState)

	var previousStream io.ReadCloser
	if previousRestReq != nil {
		tmpPreviousStream, err := previousRestReq.Stream(cancelCtx)
		if err != nil {
			logger.Log.Error(err.Error())
			reader := strings.NewReader("")
			nopCloser := io.NopCloser(reader)
			previousStream = nopCloser
		} else {
			previousStream = tmpPreviousStream
		}
	}

	stream, err := restReq.Stream(cancelCtx)
	if err != nil {
		logger.Log.Error(err.Error())
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
	return structs.ExecuteBashCommandWithResponse("Generate popeye report", "popeye")
}

func ExecuteBinaryRequestUpload(datagram structs.Datagram) *FilesUploadRequest {
	data := FilesUploadRequest{}
	marshalUnmarshal(&datagram, &data)
	return &data
}

func K8sNotification(d structs.Datagram) interface{} {
	logger.Log.Infof("Received '%s'.", d.Pattern)
	return nil
}

func marshalUnmarshal(datagram *structs.Datagram, data interface{}) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(datagram.Payload)
	if err != nil {
		datagram.Err = err.Error()
		return
	}
	err = json.Unmarshal(bytes, data)
	if err != nil {
		datagram.Err = err.Error()
	}
}
