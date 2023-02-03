package services

import (
	"bufio"
	"encoding/json"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/rest"
)

var COMMAND_REQUESTS = []string{
	"HeartBeat",
	"K8sNotification",
	"ClusterStatus",
	"files/storage-stats GET",
	"files/list POST",
	"files/create-folder POST",
	"files/rename POST",
	"files/chown POST",
	"files/chmod POST",
	"files/delete POST",
	"namespace/create POST",
	"namespace/delete POST",
	"namespace/shutdown POST",
	"namespace/reboot POST",
	"namespace/ingress-state/:state POST",
	"namespace/pod-ids/:namespace GET",
	"namespace/get-cluster-pods GET",
	"namespace/validate-cluster-pods POST",
	"namespace/validate-ports POST",
	"namespace/storage-size POST",
	"service/create POST",
	"service/delete POST",
	"service/pod-ids/:namespace/:serviceId GET",
	"service/images/:imageName PATCH",
	"service/log/:namespace/:podId GET",
	"service/resource-status/:resource/:namespace/:name/:statusOnly GET",
	"service/restart POST",
	"service/stop POST",
	"service/start POST",
	"service/update-service POST",
	"service/spectrum-bind POST",
	"service/spectrum-unbind DELETE",
	"service/spectrum-configmaps GET",
}

var STREAM_REQUESTS = []string{
	"service/log-stream/:namespace/:podId/:sinceSeconds SSE",
}

var BINARY_REQUESTS = []string{
	"files/download POST",
	"files/upload POST",
	"files/update POST",
}

func ExecuteCommandRequest(datagram structs.Datagram, c *websocket.Conn) interface{} {
	switch datagram.Pattern {
	case "HeartBeat":
		return HeartBeat(datagram, c)
	case "K8sNotification":
		return K8sNotification(datagram, c)
	case "ClusterStatus":
		return mokubernetes.ClusterStatus()
	case "files/storage-stats GET":
		return AllFiles()
	case "files/list POST":
		data := FilesListRequest{}
		marshalUnmarshal(&datagram, &data)
		return List(data, c)
	case "files/create-folder POST":
		data := FilesCreateFolderRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateFolder(data, c)
	case "files/rename POST":
		data := FilesRenameRequest{}
		marshalUnmarshal(&datagram, &data)
		return Rename(data, c)
	case "files/chown POST":
		data := FilesChownRequest{}
		marshalUnmarshal(&datagram, &data)
		return Chown(data, c)
	case "files/chmod POST":
		data := FilesChmodRequest{}
		marshalUnmarshal(&datagram, &data)
		return Chmod(data, c)
	case "files/delete POST":
		data := FilesDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return Delete(data, c)
	case "namespace/create POST":
		data := NamespaceCreateRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateNamespace(data, c)
	case "namespace/delete POST":
		data := NamespaceDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteNamespace(data, c)
	case "namespace/shutdown POST":
		data := NamespaceShutdownRequest{}
		marshalUnmarshal(&datagram, &data)
		return ShutdownNamespace(data, c)
	case "namespace/reboot POST":
		data := NamespaceRebootRequest{}
		marshalUnmarshal(&datagram, &data)
		return RebootNamespace(data, c)
	case "namespace/ingress-state/:state GET":
		data := NamespaceSetIngressStateRequest{}
		marshalUnmarshal(&datagram, &data)
		return SetIngressState(data, c)
	case "namespace/pod-ids/:namespace GET":
		data := NamespacePodIdsRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodIds(data, c)
	case "namespace/get-cluster-pods GET":
		return ClusterPods(c)
	case "namespace/validate-cluster-pods POST":
		data := NamespaceValidateClusterPodsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ValidateClusterPods(data, c)
	case "namespace/validate-ports POST":
		data := NamespaceValidatePortsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ValidateClusterPorts(data, c)
	case "namespace/storage-size POST":
		data := NamespaceStorageSizeRequest{}
		marshalUnmarshal(&datagram, &data)
		return StorageSize(data, c)
	case "service/create POST":
		data := ServiceCreateRequest{}
		marshalUnmarshal(&datagram, &data)
		return CreateService(data, c)
	case "service/delete POST":
		data := ServiceDeleteRequest{}
		marshalUnmarshal(&datagram, &data)
		return DeleteService(data, c)
	case "service/pod-ids/:namespace/:serviceId GET":
		data := ServiceGetPodIdsRequest{}
		marshalUnmarshal(&datagram, &data)
		return ServicePodIds(data, c)
	case "service/images/:imageName PATCH":
		data := ServiceSetImageRequest{}
		marshalUnmarshal(&datagram, &data)
		return SetImage(data, c)
	case "service/log/:namespace/:podId GET":
		data := ServiceGetLogRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodLog(data, c)
	case "service/resource-status/:resource/:namespace/:name/:statusOnly GET":
		data := ServiceResourceStatusRequest{}
		marshalUnmarshal(&datagram, &data)
		return PodStatus(data, c)
	case "service/restart POST":
		data := ServiceRestartRequest{}
		marshalUnmarshal(&datagram, &data)
		return Restart(data, c)
	case "service/stop POST":
		data := ServiceStopRequest{}
		marshalUnmarshal(&datagram, &data)
		return StopService(data, c)
	case "service/start POST":
		data := ServiceStartRequest{}
		marshalUnmarshal(&datagram, &data)
		return StartService(data, c)
	case "service/update-service POST":
		data := ServiceUpdateRequest{}
		marshalUnmarshal(&datagram, &data)
		return UpdateService(data, c)
	case "service/spectrum-bind POST":
		data := ServiceBindSpectrumRequest{}
		marshalUnmarshal(&datagram, &data)
		return BindSpectrum(data, c)
	case "service/spectrum-unbind DELETE":
		data := ServiceUnbindSpectrumRequest{}
		marshalUnmarshal(&datagram, &data)
		return UnbindSpectrum(data, c)
	case "service/spectrum-configmaps GET":
		return SpectrumConfigmaps(c)
	}

	datagram.Err = "Pattern not found"
	return datagram
}

func ExecuteStreamRequest(datagram structs.Datagram, c *websocket.Conn) (interface{}, *rest.Request) {
	switch datagram.Pattern {
	case "service/log-stream/:namespace/:podId/:sinceSeconds SSE":
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

func ExecuteBinaryRequest(datagram structs.Datagram, c *websocket.Conn) (interface{}, *bufio.Reader, *int64) {
	switch datagram.Pattern {
	case "files/download POST":
		data := FilesDownloadRequest{}
		marshalUnmarshal(&datagram, &data)
		reader, totalSize, err := Download(data, c)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil, utils.Pointer[int64](0)
		}
		return datagram, reader, &totalSize
	case "files/upload POST":
		data := FilesUploadRequest{}
		marshalUnmarshal(&datagram, &data)
		return Upload(data, c), nil, nil
	case "files/update POST":
		data := FilesUpdateRequest{}
		marshalUnmarshal(&datagram, &data)
		return Update(data, c), nil, nil
	}

	datagram.Err = "Pattern not found"
	return datagram, nil, nil
}

func HeartBeat(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return nil
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
