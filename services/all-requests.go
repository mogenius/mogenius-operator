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

var COMMAND_REQUESTS = []string{
	"K8sNotification",
	"ClusterStatus",
	"ClusterResourceInfo",
	"KubernetesEvent",
	"UpgradeK8sManager",
	"SERVICE_POD_EXISTS",
	"SERVICE_PODS",

	"files/list",
	"files/download",
	"files/create-folder",
	"files/rename",
	"files/chown",
	"files/chmod",
	"files/delete",

	"cluster/execute-helm-chart-task",
	"cluster/uninstall-helm-chart",
	"cluster/tcp-udp-configuration",

	"namespace/create",
	"namespace/delete",
	"namespace/shutdown",
	"namespace/pod-ids",
	"namespace/validate-cluster-pods",
	"namespace/validate-ports",
	"namespace/list-all",
	"namespace/gather-all-resources",
	"namespace/backup",
	"namespace/restore",

	"service/create",
	"service/delete",
	"service/pod-ids",
	"service/set-image",
	"service/log",
	"service/log-error",
	"service/resource-status",
	"service/restart",
	"service/stop",
	"service/start",
	"service/update-service",
	"service/spectrum-bind",
	"service/spectrum-unbind",
	"service/spectrum-configmaps",

	"service/log-stream",

	"list/create-templates",

	"list/namespaces",
	"list/deployments",
	"list/services",
	"list/pods",
	"list/ingresses",
	"list/configmaps",
	"list/secrets",
	"list/nodes",
	"list/daemonsets",
	"list/statefulsets",
	"list/jobs",
	"list/cronjobs",
	"list/replicasets",
	"list/persistent_volumes",
	"list/persistent_volume_claims",
	"list/horizontal_pod_autoscalers",
	"list/events",
	"list/certificates",
	"list/certificaterequests",
	"list/orders",
	"list/issuers",
	"list/clusterissuers",
	"list/service_account",
	"list/role",
	"list/role_binding",
	"list/cluster_role",
	"list/cluster_role_binding",
	"list/volume_attachment",
	"list/network_policy",
	"list/storage_class",

	"create/namespace",
	"create/deployment",
	"create/service",
	"create/pod",
	"create/ingress",
	"create/configmap",
	"create/secret",
	"create/daemonset",
	"create/statefulset",
	"create/job",
	"create/cronjob",
	"create/replicaset",
	"create/persistent_volume",
	"create/persistent_volume_claim",
	"create/horizontal_pod_autoscaler",
	"create/certificate",
	"create/certificaterequest",
	"create/order",
	"create/issuer",
	"create/clusterissuer",
	"create/service_account",
	"create/role",
	"create/role_binding",
	"create/cluster_role",
	"create/cluster_role_binding",
	"create/volume_attachment",
	"create/network_policy",
	"create/storage_class",

	"describe/namespace",
	"describe/deployment",
	"describe/service",
	"describe/pod",
	"describe/ingresse",
	"describe/configmap",
	"describe/secret",
	"describe/node",
	"describe/daemonset",
	"describe/statefulset",
	"describe/job",
	"describe/cronjob",
	"describe/replicaset",
	"describe/persistent_volume",
	"describe/persistent_volume_claim",
	"describe/horizontal_pod_autoscaler",
	"describe/event",
	"describe/certificate",
	"describe/certificaterequest",
	"describe/order",
	"describe/issuer",
	"describe/clusterissuer",
	"describe/service_account",
	"describe/role",
	"describe/role_binding",
	"describe/cluster_role",
	"describe/cluster_role_binding",
	"describe/volume_attachment",
	"describe/network_policy",
	"describe/storage_class",

	"update/deployment",
	"update/service",
	"update/pod",
	"update/ingress",
	"update/configmap",
	"update/secret",
	"update/daemonset",
	"update/statefulset",
	"update/job",
	"update/cronjob",
	"update/replicaset",
	"update/persistent_volume",
	"update/persistent_volume_claim",
	"update/horizontal_pod_autoscalers",
	"update/certificates",
	"update/certificaterequests",
	"update/orders",
	"update/issuers",
	"update/clusterissuers",
	"update/service_account",
	"update/role",
	"update/role_binding",
	"update/cluster_role",
	"update/cluster_role_binding",
	"update/volume_attachment",
	"update/network_policy",
	"update/storage_class",

	"delete/namespace",
	"delete/deployment",
	"delete/service",
	"delete/pod",
	"delete/ingress",
	"delete/configmap",
	"delete/secret",
	"delete/daemonset",
	"delete/statefulset",
	"delete/job",
	"delete/cronjob",
	"delete/replicaset",
	"delete/persistent_volume",
	"delete/persistent_volume_claim",
	"delete/certificates",
	"delete/certificaterequests",
	"delete/orders",
	"delete/issuers",
	"delete/clusterissuers",
	"delete/service_account",
	"delete/role",
	"delete/role_binding",
	"delete/cluster_role",
	"delete/cluster_role_binding",
	"delete/volume_attachment",
	"delete/network_policy",
	"delete/storage_class",

	"storage/create-volume",
	"storage/delete-volume",
	"storage/backup-volume",
	"storage/restore-volume",
	"storage/stats",
	"storage/namespace/stats",

	"popeye-console",
}

var BINARY_REQUEST_UPLOAD = []string{
	"files/upload",
}

func ExecuteCommandRequest(datagram structs.Datagram) interface{} {
	switch datagram.Pattern {
	case "K8sNotification":
		return K8sNotification(datagram)
	case "ClusterStatus":
		return mokubernetes.ClusterStatus()
	case "ClusterResourceInfo":
		nodeStats := mokubernetes.GetNodeStats()
		loadBalancerExternalIps := mokubernetes.GetClusterExternalIps()
		result := dtos.ClusterResourceInfoDto{
			NodeStats:               nodeStats,
			LoadBalancerExternalIps: loadBalancerExternalIps,
		}
		return result
	case "UpgradeK8sManager":
		data := K8sManagerUpgradeRequest{}
		marshalUnmarshal(&datagram, &data)
		return UpgradeK8sManager(data)
	case "files/list":
		data := FilesListRequest{}
		marshalUnmarshal(&datagram, &data)
		return List(data)
	case "files/create-folder":
		data := FilesCreateFolderRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateFolder(data)
	case "files/rename":
		data := FilesRenameRequest{}
		marshalUnmarshal(&datagram, &data)
		return Rename(data)
	case "files/chown":
		data := FilesChownRequest{}
		marshalUnmarshal(&datagram, &data)
		return Chown(data)
	case "files/chmod":
		data := FilesChmodRequest{}
		marshalUnmarshal(&datagram, &data)
		return Chmod(data)
	case "files/delete":
		data := FilesDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return Delete(data)
	case "files/download":
		data := FilesDownloadRequest{}
		marshalUnmarshal(&datagram, &data)
		return Download(data)

	case "cluster/execute-helm-chart-task":
		data := ClusterHelmRequest{}
		marshalUnmarshal(&datagram, &data)
		return InstallHelmChart(data)
	case "cluster/uninstall-helm-chart":
		data := ClusterHelmUninstallRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteHelmChart(data)
	case "cluster/tcp-udp-configuration":
		return TcpUdpClusterConfiguration()

	case "namespace/create":
		data := NamespaceCreateRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateNamespace(data)
	case "namespace/delete":
		data := NamespaceDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteNamespace(data)
	case "namespace/shutdown":
		data := NamespaceShutdownRequest{}
		marshalUnmarshal(&datagram, &data)
		return ShutdownNamespace(data)
	case "namespace/pod-ids":
		data := NamespacePodIdsRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodIds(data)
	case "namespace/validate-cluster-pods":
		data := NamespaceValidateClusterPodsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ValidateClusterPods(data)
	case "namespace/validate-ports":
		data := NamespaceValidatePortsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ValidateClusterPorts(data)
	case "namespace/list-all":
		return ListAllNamespaces()
	case "namespace/gather-all-resources":
		data := NamespaceGatherAllResourcesRequest{}
		marshalUnmarshal(&datagram, &data)
		return ListAllResourcesForNamespace(data)
	case "namespace/backup":
		data := NamespaceBackupRequest{}
		marshalUnmarshal(&datagram, &data)
		result, err := mokubernetes.BackupNamespace(data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case "namespace/restore":
		data := NamespaceRestoreRequest{}
		marshalUnmarshal(&datagram, &data)
		result, err := mokubernetes.RestoreNamespace(data.YamlData, data.NamespaceName)
		if err != nil {
			return err.Error()
		}
		return result
	case "service/create":
		data := ServiceCreateRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateService(data)
	case "service/delete":
		data := ServiceDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteService(data)
	case "service/pod-ids":
		data := ServiceGetPodIdsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ServicePodIds(data)
	case "SERVICE_POD_EXISTS":
		data := ServicePodExistsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ServicePodExists(data)
	case "SERVICE_PODS":
		data := ServicePodsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ServicePodStatus(data)
	case "service/set-image":
		data := ServiceSetImageRequest{}
		marshalUnmarshal(&datagram, &data)
		return SetImage(data)
	case "service/log":
		data := ServiceGetLogRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodLog(data)
	case "service/log-error":
		data := ServiceGetLogRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodLogError(data)
	case "service/resource-status":
		data := ServiceResourceStatusRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodStatus(data)
	case "service/restart":
		data := ServiceRestartRequest{}
		marshalUnmarshal(&datagram, &data)
		return Restart(data)
	case "service/stop":
		data := ServiceStopRequest{}
		marshalUnmarshal(&datagram, &data)
		return StopService(data)
	case "service/start":
		data := ServiceStartRequest{}
		marshalUnmarshal(&datagram, &data)
		return StartService(data)
	case "service/update-service":
		data := ServiceUpdateRequest{}
		marshalUnmarshal(&datagram, &data)
		return UpdateService(data)

	case "service/log-stream":
		data := ServiceLogStreamRequest{}
		marshalUnmarshal(&datagram, &data)
		return logStream(data, datagram)

	case "list/create-templates":
		return mokubernetes.ListCreateTemplates()

	case "list/namespaces":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.ListK8sNamespaces(data.NamespaceName)
	case "list/deployments":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sDeployments(data.NamespaceName)
	case "list/services":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sServices(data.NamespaceName)
	case "list/pods":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sPods(data.NamespaceName)
	case "list/ingresses":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sIngresses(data.NamespaceName)
	case "list/configmaps":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sConfigmaps(data.NamespaceName)
	case "list/secrets":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sSecrets(data.NamespaceName)
	case "list/nodes":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.ListK8sNodes()
	case "list/daemonsets":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sDaemonsets(data.NamespaceName)
	case "list/statefulsets":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllStatefulSets(data.NamespaceName)
	case "list/jobs":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllJobs(data.NamespaceName)
	case "list/cronjobs":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllCronjobs(data.NamespaceName)
	case "list/replicasets":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sReplicasets(data.NamespaceName)
	case "list/persistent_volumes":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllPersistentVolumes()
	case "list/persistent_volume_claims":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sPersistentVolumeClaims(data.NamespaceName)
	case "list/horizontal_pod_autoscalers":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllHpas(data.NamespaceName)
	case "list/events":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllEvents(data.NamespaceName)
	case "list/certificates":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllK8sCertificates(data.NamespaceName)
	case "list/certificaterequests":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllCertificateSigningRequests(data.NamespaceName)
	case "list/orders":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllOrders(data.NamespaceName)
	case "list/issuers":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllIssuer(data.NamespaceName)
	case "list/clusterissuers":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllClusterIssuers()
	case "list/service_account":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllServiceAccounts(data.NamespaceName)
	case "list/role":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllRoles(data.NamespaceName)
	case "list/role_binding":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllRoleBindings(data.NamespaceName)
	case "list/cluster_role":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllClusterRoles(data.NamespaceName)
	case "list/cluster_role_binding":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllClusterRoleBindings(data.NamespaceName)
	case "list/volume_attachment":
		return mokubernetes.AllVolumeAttachments()
	case "list/network_policy":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllNetworkPolicies(data.NamespaceName)
	case "list/storage_class":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllStorageClasses()

	case "describe/namespace":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sNamespace(data.NamespaceName)
	case "describe/deployment":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sDeployment(data.NamespaceName, data.ResourceName)
	case "describe/service":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sService(data.NamespaceName, data.ResourceName)
	case "describe/pod":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sPod(data.NamespaceName, data.ResourceName)
	case "describe/ingress":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sIngress(data.NamespaceName, data.ResourceName)
	case "describe/configmap":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sConfigmap(data.NamespaceName, data.ResourceName)
	case "describe/secret":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sSecret(data.NamespaceName, data.ResourceName)
	case "describe/node":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sNode(data.ResourceName)
	case "describe/daemonset":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sDaemonSet(data.NamespaceName, data.ResourceName)
	case "describe/statefulset":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sStatefulset(data.NamespaceName, data.ResourceName)
	case "describe/job":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sJob(data.NamespaceName, data.ResourceName)
	case "describe/cronjob":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sCronJob(data.NamespaceName, data.ResourceName)
	case "describe/replicaset":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sReplicaset(data.NamespaceName, data.ResourceName)
	case "describe/persistent_volume":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sPersistentVolume(data.ResourceName)
	case "describe/persistent_volume_claim":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sPersistentVolumeClaim(data.NamespaceName, data.ResourceName)
	case "describe/horizontal_pod_autoscaler":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sHpa(data.NamespaceName, data.ResourceName)
	case "describe/event":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sEvent(data.NamespaceName, data.ResourceName)
	case "describe/certificate":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sCertificate(data.NamespaceName, data.ResourceName)
	case "describe/certificaterequest":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sCertificateSigningRequest(data.ResourceName)
	case "describe/order":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sOrder(data.NamespaceName, data.ResourceName)
	case "describe/issuer":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sIssuer(data.NamespaceName, data.ResourceName)
	case "describe/clusterissuer":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sClusterIssuer(data.ResourceName)
	case "describe/service_account":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sServiceAccount(data.NamespaceName, data.ResourceName)
	case "describe/role":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sRole(data.NamespaceName, data.ResourceName)
	case "describe/role_binding":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sRoleBinding(data.NamespaceName, data.ResourceName)
	case "describe/cluster_role":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sClusterRole(data.ResourceName)
	case "describe/cluster_role_binding":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sClusterRoleBinding(data.ResourceName)
	case "describe/volume_attachment":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sVolumeAttachment(data.ResourceName)
	case "describe/network_policy":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sNetworkPolicy(data.NamespaceName, data.ResourceName)
	case "describe/storage_class":
		data := K8sDescribeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DescribeK8sStorageClass(data.ResourceName)

	case "update/deployment":
		data := K8sUpdateDeploymentRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateDeployment(data)
	case "update/service":
		data := K8sUpdateServiceRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateService(data)
	case "update/pod":
		data := K8sUpdatePodRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdatePod(data)
	case "update/ingress":
		data := K8sUpdateIngressRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateIngress(data)
	case "update/configmap":
		data := K8sUpdateConfigmapRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateConfigMap(data)
	case "update/secret":
		data := K8sUpdateSecretRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateSecret(data)
	case "update/daemonset":
		data := K8sUpdateDaemonSetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateDaemonSet(data)
	case "update/statefulset":
		data := K8sUpdateStatefulSetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateStatefulset(data)
	case "update/job":
		data := K8sUpdateJobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateJob(data)
	case "update/cronjob":
		data := K8sUpdateCronJobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateCronJob(data)
	case "update/replicaset":
		data := K8sUpdateReplicaSetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateReplicaSet(data)
	case "update/persistentvolume":
		data := K8sUpdatePersistentVolumeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sPersistentVolume(*data.Data)
	case "update/persistentvolumeclaim":
		data := K8sUpdatePersistentVolumeClaimRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sPersistentVolumeClaim(*data.Data)
	case "update/horizontal_pod_autoscalers":
		data := K8sUpdateHPARequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sHpa(*data.Data)
	case "update/certificates":
		data := K8sUpdateCertificateRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sCertificate(*data.Data)
	case "update/certificaterequests":
		data := K8sUpdateCertificateRequestRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sCertificateSigningRequest(*data.Data)
	case "update/orders":
		data := K8sUpdateOrderRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sOrder(*data.Data)
	case "update/issuers":
		data := K8sUpdateIssuerRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sIssuer(*data.Data)
	case "update/clusterissuers":
		data := K8sUpdateClusterIssuerRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sClusterIssuer(*data.Data)
	case "update/service_account":
		data := K8sUpdateServiceAccountRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sServiceAccount(*data.Data)
	case "update/role":
		data := K8sUpdateRoleRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sRole(*data.Data)
	case "update/role_binding":
		data := K8sUpdateRoleBindingRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sRoleBinding(*data.Data)
	case "update/cluster_role":
		data := K8sUpdateClusterRoleRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sClusterRole(*data.Data)
	case "update/cluster_role_binding":
		data := K8sUpdateClusterRoleBindingRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sClusterRoleBinding(*data.Data)
	case "update/volume_attachment":
		data := K8sUpdateVolumeAttachmentRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sVolumeAttachment(*data.Data)
	case "update/network_policy":
		data := K8sUpdateNetworkPolicyRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sNetworkPolicy(*data.Data)
	case "update/storage_class":
		data := K8sUpdateStorageClassRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.UpdateK8sStorageClass(*data.Data)

	case "delete/namespace":
		data := K8sDeleteNamespaceRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteNamespace(data)
	case "delete/deployment":
		data := K8sDeleteDeploymentRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteDeployment(data)
	case "delete/service":
		data := K8sDeleteServiceRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteService(data)
	case "delete/pod":
		data := K8sDeletePodRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeletePod(data)
	case "delete/ingress":
		data := K8sDeleteIngressRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteIngress(data)
	case "delete/configmap":
		data := K8sDeleteConfigmapRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteConfigMap(data)
	case "delete/secret":
		data := K8sDeleteSecretRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteSecret(data)
	case "delete/daemonset":
		data := K8sDeleteDaemonsetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteDaemonSet(data)
	case "delete/statefulset":
		data := K8sDeleteStatefulsetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteStatefulset(data)
	case "delete/job":
		data := K8sDeleteJobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteJob(data)
	case "delete/cronjob":
		data := K8sDeleteCronjobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteCronJob(data)
	case "delete/replicaset":
		data := K8sDeleteReplicasetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteReplicaSet(data)
	case "delete/persistentvolume":
		data := K8sDeletePersistentVolumeRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sPersistentVolume(*data.Data)
	case "delete/persistentvolumeclaim":
		data := K8sDeletePersistentVolumeClaimRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sPersistentVolumeClaim(*data.Data)
	case "delete/horizontal_pod_autoscalers":
		data := K8sDeleteHPARequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sHpa(*data.Data)
	case "delete/certificates":
		data := K8sDeleteCertificateRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sCertificate(*data.Data)
	case "delete/certificaterequests":
		data := K8sDeleteCertificateRequestRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sCertificateSigningRequest(*data.Data)
	case "delete/orders":
		data := K8sDeleteOrderRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sOrder(*data.Data)
	case "delete/issuers":
		data := K8sDeleteIssuerRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sIssuer(*data.Data)
	case "delete/clusterissuers":
		data := K8sDeleteClusterIssuerRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sClusterIssuer(*data.Data)
	case "delete/service_account":
		data := K8sDeleteServiceAccountRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sServiceAccount(*data.Data)
	case "delete/role":
		data := K8sDeleteRoleRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sRole(*data.Data)
	case "delete/role_binding":
		data := K8sDeleteRoleBindingRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sRoleBinding(*data.Data)
	case "delete/cluster_role":
		data := K8sDeleteClusterRoleRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sClusterRole(*data.Data)
	case "delete/cluster_role_binding":
		data := K8sDeleteClusterRoleBindingRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sClusterRoleBinding(*data.Data)
	case "delete/volume_attachment":
		data := K8sDeleteVolumeAttachmentRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sVolumeAttachment(*data.Data)
	case "delete/network_policy":
		data := K8sDeleteNetworkPolicyRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sNetworkPolicy(*data.Data)
	case "delete/storage_class":
		data := K8sDeleteStorageClassRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.DeleteK8sStorageClass(*data.Data)

	// case "storage/enable":
	// 	data := NfsStorageInstallRequest{}
	// 	marshalUnmarshal(&datagram, &data)
	// 	return InstallMogeniusNfsStorage(data)
	// case "storage/disable":
	// 	data := NfsStorageInstallRequest{}
	// 	marshalUnmarshal(&datagram, &data)
	// 	return UninstallMogeniusNfsStorage(data)
	// case "storage/check-if-installed":
	// 	return mokubernetes.CheckIfMogeniusNfsIsRunning()
	case "storage/create-volume":
		data := NfsVolumeRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateMogeniusNfsVolume(data)
	case "storage/delete-volume":
		data := NfsVolumeRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteMogeniusNfsVolume(data)
	case "storage/backup-volume":
		data := NfsVolumeBackupRequest{}
		marshalUnmarshal(&datagram, &data)
		return BackupMogeniusNfsVolume(data)
	case "storage/restore-volume":
		data := NfsVolumeRestoreRequest{}
		marshalUnmarshal(&datagram, &data)
		return RestoreMogeniusNfsVolume(data)
	case "storage/stats":
		data := NfsVolumeStatsRequest{}
		marshalUnmarshal(&datagram, &data)
		return StatsMogeniusNfsVolume(data)
	case "storage/namespace/stats":
		data := NfsNamespaceStatsRequest{}
		marshalUnmarshal(&datagram, &data)
		return StatsMogeniusNfsNamespace(data)
	case "popeye-console":
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
