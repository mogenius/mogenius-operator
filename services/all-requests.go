package services

import (
	"encoding/json"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"

	"github.com/gorilla/websocket"
)

var ALL_REQUESTS = []string{
	"HeartBeat",
	"K8sNotification",
	"ClusterStatus",
	"files/storage-stats GET",
	"files/list POST",
	"files/download POST",
	"files/upload POST",
	"files/update POST",
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
	"service/log-stream/:namespace/:podId/:sinceSeconds SSE",
	"service/resource-status/:resource/:namespace/:name/:statusOnly GET",
	"service/restart POST",
	"service/stop POST",
	"service/start POST",
	"service/update-service POST",
	"service/spectrum-bind POST",
	"service/spectrum-unbind DELETE",
	"service/spectrum-configmaps GET",
}

func ExecuteRequest(datagram structs.Datagram, c *websocket.Conn) interface{} {
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
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return List(data, c)
	case "files/download POST":
		data := FilesDownloadRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Download(data, c)
	case "files/upload POST":
		data := FilesUploadRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Upload(data, c)
	case "files/update POST":
		data := FilesUpdateRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Update(data, c)
	case "files/create-folder POST":
		data := FilesCreateFolderRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return CreateFolder(data, c)
	case "files/rename POST":
		data := FilesRenameRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Rename(data, c)
	case "files/chown POST":
		data := FilesChownRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Chown(data, c)
	case "files/chmod POST":
		data := FilesChmodRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Chmod(data, c)
	case "files/delete POST":
		data := FilesDeleteRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Delete(data, c)
	case "namespace/create POST":
		data := NamespaceCreateRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return CreateNamespace(data, c)
	case "namespace/delete POST":
		data := NamespaceDeleteRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return DeleteNamespace(data, c)
	case "namespace/shutdown POST":
		data := NamespaceShutdownRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return ShutdownNamespace(data, c)
	case "namespace/reboot POST":
		data := NamespaceRebootRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return RebootNamespace(data, c)
	case "namespace/ingress-state/:state GET":
		data := NamespaceSetIngressStateRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return SetIngressState(data, c)
	case "namespace/pod-ids/:namespace GET":
		data := NamespacePodIdsRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return PodIds(data, c)
	case "namespace/get-cluster-pods GET":
		return ClusterPods(c)
	case "namespace/validate-cluster-pods POST":
		data := NamespaceValidateClusterPodsRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return ValidateClusterPods(data, c)
	case "namespace/validate-ports POST":
		data := NamespaceValidatePortsRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return ValidateClusterPorts(data, c)
	case "namespace/storage-size POST":
		data := NamespaceStorageSizeRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return StorageSize(data, c)
	case "service/create POST":
		data := ServiceCreateRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return CreateService(data, c)
	case "service/delete POST":
		data := ServiceDeleteRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return DeleteService(data, c)
	case "service/pod-ids/:namespace/:service GET":
		data := ServiceGetPodIdsRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return ServicePodIds(data, c)
	case "service/images/:imageName PATCH":
		data := ServiceSetImageRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return SetImage(data, c)
	case "service/log/:namespace/:podId GET":
		data := ServiceGetLogRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return PodLog(data, c)
	case "service/log-stream/:namespace/:podId/:sinceSeconds SSE":
		data := ServiceLogStreamRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return PodLogStream(data, c)
	case "service/resource-status/:resource/:namespace/:name/:statusOnly GET":
		data := ServiceResourceStatusRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return PodStatus(data, c)
	case "service/restart POST":
		data := ServiceRestartRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Restart(data, c)
	case "service/stop POST":
		data := ServiceStopRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return StopService(data, c)
	case "service/start POST":
		data := ServiceStartRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return StartService(data, c)
	case "service/update-service POST":
		data := ServiceUpdateRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return UpdateService(data, c)
	case "service/spectrum-bind POST":
		data := ServiceBindSpectrumRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return BindSpectrum(data, c)
	case "service/spectrum-unbind DELETE":
		data := ServiceUnbindSpectrumRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return UnbindSpectrum(data, c)
	case "service/spectrum-configmaps GET":
		return SpectrumConfigmaps(c)
	}

	datagram.Err = "Pattern not found"
	return datagram
}

func HeartBeat(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return nil
}

func K8sNotification(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return nil
}
