package services

import (
	"bufio"
	"encoding/json"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/rest"
)

var COMMAND_REQUESTS = []string{
	"K8sNotification",
	"ClusterStatus",
	"ClusterResourceInfo",
	"KubernetesEvent",
	"UpgradeK8sManager",
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
	"service/create",
	"service/delete",
	"service/pod-ids",
	"service/set-image",
	"service/log",
	"service/resource-status",
	"service/restart",
	"service/stop",
	"service/start",
	"service/update-service",
	"service/spectrum-bind",
	"service/spectrum-unbind",
	"service/spectrum-configmaps",
}

var STREAM_REQUESTS = []string{
	"service/log-stream",
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
		loadBalancerExternalIps := kubernetes.GetClusterExternalIps()
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
	case "service/set-image":
		data := ServiceSetImageRequest{}
		marshalUnmarshal(&datagram, &data)
		return SetImage(data, c)
	case "service/log":
		data := ServiceGetLogRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodLog(data, c)
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
	}

	datagram.Err = "Pattern not found"
	return datagram
}

func ExecuteStreamRequest(datagram structs.Datagram, c *websocket.Conn) (interface{}, *rest.Request) {
	switch datagram.Pattern {
	case "service/log-stream":
		data := ServiceLogStreamRequest{}
		marshalUnmarshal(&datagram, &data)
		restReq, err := PodLogStream(data, c)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return data, restReq
	}

	datagram.Err = "Pattern not found"
	return datagram, nil
}

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
