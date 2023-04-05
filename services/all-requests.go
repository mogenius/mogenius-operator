package services

import (
	"bufio"
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/http"
	"net/url"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/rest"
)

var COMMAND_REQUESTS = []string{
	"K8sNotification",
	"ClusterStatus",
	"ClusterResourceInfo",
	"KubernetesEvent",
	"UpgradeK8sManager",
	"SERVICE_POD_EXISTS",
	"files/list",
	"files/create-folder",
	"files/rename",
	"files/chown",
	"files/chmod",
	"files/delete",
	"cluster/execute-helm-chart-task",
	"cluster/uninstall-helm-chart",

	"namespace/create",
	"namespace/delete",
	"namespace/shutdown",
	"namespace/pod-ids",
	"namespace/validate-cluster-pods",
	"namespace/validate-ports",
	"namespace/storage-size",
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
}

var BINARY_REQUESTS_DOWNLOAD = []string{
	"files/download",
}

var BINARY_REQUEST_UPLOAD = []string{
	"files/upload",
}

func ExecuteCommandRequest(datagram structs.Datagram, c *websocket.Conn) interface{} {
	switch datagram.Pattern {
	case "K8sNotification":
		return K8sNotification(datagram, c)
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
		return UpgradeK8sManager(data, c)
	case "files/list":
		data := FilesListRequest{}
		marshalUnmarshal(&datagram, &data)
		return List(data, c)
	case "files/create-folder":
		data := FilesCreateFolderRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateFolder(data, c)
	case "files/rename":
		data := FilesRenameRequest{}
		marshalUnmarshal(&datagram, &data)
		return Rename(data, c)
	case "files/chown":
		data := FilesChownRequest{}
		marshalUnmarshal(&datagram, &data)
		return Chown(data, c)
	case "files/chmod":
		data := FilesChmodRequest{}
		marshalUnmarshal(&datagram, &data)
		return Chmod(data, c)
	case "files/delete":
		data := FilesDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return Delete(data, c)
	case "cluster/execute-helm-chart-task":
		data := ClusterHelmRequest{}
		marshalUnmarshal(&datagram, &data)
		return InstallHelmChart(data, c)
	case "cluster/uninstall-helm-chart":
		data := ClusterHelmUninstallRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteHelmChart(data, c)
	case "namespace/create":
		data := NamespaceCreateRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateNamespace(data, c)
	case "namespace/delete":
		data := NamespaceDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteNamespace(data, c)
	case "namespace/shutdown":
		data := NamespaceShutdownRequest{}
		marshalUnmarshal(&datagram, &data)
		return ShutdownNamespace(data, c)
	case "namespace/pod-ids":
		data := NamespacePodIdsRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodIds(data, c)
	case "namespace/validate-cluster-pods":
		data := NamespaceValidateClusterPodsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ValidateClusterPods(data, c)
	case "namespace/validate-ports":
		data := NamespaceValidatePortsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ValidateClusterPorts(data, c)
	case "namespace/storage-size":
		data := NamespaceStorageSizeRequest{}
		marshalUnmarshal(&datagram, &data)
		return StorageSize(data, c)
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
		return CreateService(data, c)
	case "service/delete":
		data := ServiceDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteService(data, c)
	case "service/pod-ids":
		data := ServiceGetPodIdsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ServicePodIds(data, c)
	case "SERVICE_POD_EXISTS":
		data := ServicePodExistsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ServicePodExists(data, c)
	case "service/set-image":
		data := ServiceSetImageRequest{}
		marshalUnmarshal(&datagram, &data)
		return SetImage(data, c)
	case "service/log":
		data := ServiceGetLogRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodLog(data, c)
	case "service/log-error":
		data := ServiceGetLogRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodLogError(data, c)
	case "service/resource-status":
		data := ServiceResourceStatusRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodStatus(data, c)
	case "service/restart":
		data := ServiceRestartRequest{}
		marshalUnmarshal(&datagram, &data)
		return Restart(data, c)
	case "service/stop":
		data := ServiceStopRequest{}
		marshalUnmarshal(&datagram, &data)
		return StopService(data, c)
	case "service/start":
		data := ServiceStartRequest{}
		marshalUnmarshal(&datagram, &data)
		return StartService(data, c)
	case "service/update-service":
		data := ServiceUpdateRequest{}
		marshalUnmarshal(&datagram, &data)
		return UpdateService(data, c)
	case "service/spectrum-bind":
		data := ServiceBindSpectrumRequest{}
		marshalUnmarshal(&datagram, &data)
		result, err := BindSpectrum(data, c)
		if err != nil {
			logger.Log.Error(err)
		}
		return result
	case "service/spectrum-unbind":
		data := ServiceUnbindSpectrumRequest{}
		marshalUnmarshal(&datagram, &data)
		result, err := UnbindSpectrum(data, c)
		if err != nil {
			logger.Log.Error(err)
		}
		return result
	case "service/spectrum-configmaps":
		return SpectrumConfigmaps(c)
	case "service/log-stream":
		data := ServiceLogStreamRequest{}
		marshalUnmarshal(&datagram, &data)
		return logStream(data, datagram, c)

	case "list/namespaces":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.ListK8sNamespaces(data.NamespaceName)
	case "list/deployments":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllDeployments(data.NamespaceName)
	case "list/services":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllServices(data.NamespaceName)
	case "list/pods":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllPods(data.NamespaceName)
	case "list/ingresses":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllIngresses(data.NamespaceName)
	case "list/configmaps":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllConfigmaps(data.NamespaceName)
	case "list/secrets":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllSecrets(data.NamespaceName)
	case "list/nodes":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.ListNodes()
	case "list/daemonsets":
		data := K8sListRequest{}
		marshalUnmarshal(&datagram, &data)
		return mokubernetes.AllDaemonsets(data.NamespaceName)
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
		return mokubernetes.AllReplicasets(data.NamespaceName)

	case "update/deployment":
		data := K8sUpdateDeploymentRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateDeployment(data, c)
	case "update/service":
		data := K8sUpdateServiceRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateService(data, c)
	case "update/pod":
		data := K8sUpdatePodRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdatePod(data, c)
	case "update/ingress":
		data := K8sUpdateIngressRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateIngress(data, c)
	case "update/configmap":
		data := K8sUpdateConfigmapRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateConfigMap(data, c)
	case "update/secret":
		data := K8sUpdateSecretRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateSecret(data, c)
	case "update/daemonset":
		data := K8sUpdateDaemonSetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateDaemonSet(data, c)
	case "update/statefulset":
		data := K8sUpdateStatefulSetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateStatefulset(data, c)
	case "update/job":
		data := K8sUpdateJobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateJob(data, c)
	case "update/cronjob":
		data := K8sUpdateCronJobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateCronJob(data, c)
	case "update/replicaset":
		data := K8sUpdateReplicaSetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sUpdateReplicaSet(data, c)

	case "delete/namespace":
		data := K8sDeleteNamespaceRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteNamespace(data, c)
	case "delete/deployment":
		data := K8sDeleteDeploymentRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteDeployment(data, c)
	case "delete/service":
		data := K8sDeleteServiceRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteService(data, c)
	case "delete/pod":
		data := K8sDeletePodRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeletePod(data, c)
	case "delete/ingress":
		data := K8sDeleteIngressRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteIngress(data, c)
	case "delete/configmap":
		data := K8sDeleteConfigmapRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteConfigMap(data, c)
	case "delete/secret":
		data := K8sDeleteSecretRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteSecret(data, c)
	case "delete/daemonset":
		data := K8sDeleteDaemonsetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteDaemonSet(data, c)
	case "delete/statefulset":
		data := K8sDeleteStatefulsetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteStatefulset(data, c)
	case "delete/job":
		data := K8sDeleteJobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteJob(data, c)
	case "delete/cronjob":
		data := K8sDeleteCronjobRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteCronJob(data, c)
	case "delete/replicaset":
		data := K8sDeleteReplicasetRequest{}
		marshalUnmarshal(&datagram, &data)
		return K8sDeleteReplicaSet(data, c)
	}

	datagram.Err = "Pattern not found"
	return datagram
}

func logStream(data ServiceLogStreamRequest, datagram structs.Datagram, c *websocket.Conn) ServiceLogStreamResult {
	requestURL := url.URL{Scheme: utils.CONFIG.ApiServer.Proto, Host: fmt.Sprintf("%s:%d", utils.CONFIG.ApiServer.Server, utils.CONFIG.ApiServer.HttpPort), Path: utils.CONFIG.ApiServer.StreamPath}
	result := ServiceLogStreamResult{
		Message: fmt.Sprintf("Initiated log-stream '%s' for '%s/%s'.", requestURL.String(), data.Namespace, data.PodId),
	}

	restReq, err := PodLogStream(data, c)
	if err != nil {
		result.Message = err.Error()
		logger.Log.Error(result.Message)
		return result
	}

	go streamData(restReq, requestURL.String())

	return result
}

func streamData(restReq *rest.Request, toServerUrl string) {

	ctx := context.Background()
	cancelCtx, endGofunc := context.WithCancel(ctx)
	stream, err := restReq.Stream(cancelCtx)
	if err != nil {
		logger.Log.Error(err.Error())
	}
	defer func() {
		if stream != nil {
			stream.Close()
		}
		endGofunc()
	}()
	if err != nil {
		logger.Log.Error(err.Error())
	}

	req, err := http.NewRequest(http.MethodPost, toServerUrl, stream)
	if err != nil {
		logger.Log.Errorf("streamData client: could not create request: %s\n", err)
	}
	req.Header = utils.HttpHeader()

	client := http.Client{
		Timeout: 0 * time.Second, // no timeout
	}

	_, err = client.Do(req)
	if err != nil {
		logger.Log.Errorf("streamData client: error making http request: %s\n", err)
	}
}

// func ExecuteStreamRequest(datagram structs.Datagram, c *websocket.Conn) (interface{}, *rest.Request) {
// 	switch datagram.Pattern {
// 	case "service/log-stream":
// 		data := ServiceLogStreamRequest{}
// 		marshalUnmarshal(&datagram, &data)
// 		restReq, err := PodLogStream(data, c)
// 		if err != nil {
// 			datagram.Err = err.Error()
// 			return datagram, nil
// 		}
// 		return data, restReq
// 	}

// 	datagram.Err = "Pattern not found"
// 	return datagram, nil
// }

func ExecuteBinaryRequestDownload(datagram structs.Datagram, c *websocket.Conn) (interface{}, *bufio.Reader, *int64) {
	switch datagram.Pattern {
	case "files/download":
		data := FilesDownloadRequest{}
		marshalUnmarshal(&datagram, &data)
		reader, totalSize, err := Download(data, c)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil, utils.Pointer[int64](0)
		}
		return datagram, reader, &totalSize
	}

	datagram.Err = "Pattern not found"
	return datagram, nil, nil
}

func ExecuteBinaryRequestUpload(datagram structs.Datagram, c *websocket.Conn) *FilesUploadRequest {
	data := FilesUploadRequest{}
	marshalUnmarshal(&datagram, &data)
	return &data
}

func K8sNotification(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
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
