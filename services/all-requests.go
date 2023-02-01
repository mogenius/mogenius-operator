package services

import (
	"bufio"
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

func ExecuteRequest(datagram structs.Datagram, c *websocket.Conn) (interface{}, *bufio.Reader) {
	switch datagram.Pattern {
	case "HeartBeat":
		return HeartBeat(datagram, c), nil
	case "K8sNotification":
		return K8sNotification(datagram, c), nil
	case "ClusterStatus":
		return mokubernetes.ClusterStatus(), nil
	case "files/storage-stats GET":
		return AllFiles(), nil
	case "files/list POST":
		data := FilesListRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return List(data, c), nil
	case "files/download POST":
		data := FilesDownloadRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		reader, err := Download(data, c)
		return nil, reader
	case "files/upload POST":
		data := FilesUploadRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return Upload(data, c), nil
	case "files/update POST":
		data := FilesUpdateRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return Update(data, c), nil
	case "files/create-folder POST":
		data := FilesCreateFolderRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return CreateFolder(data, c), nil
	case "files/rename POST":
		data := FilesRenameRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return Rename(data, c), nil
	case "files/chown POST":
		data := FilesChownRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return Chown(data, c), nil
	case "files/chmod POST":
		data := FilesChmodRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return Chmod(data, c), nil
	case "files/delete POST":
		data := FilesDeleteRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return Delete(data, c), nil
	case "namespace/create POST":
		data := NamespaceCreateRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return CreateNamespace(data, c), nil
	case "namespace/delete POST":
		data := NamespaceDeleteRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return DeleteNamespace(data, c), nil
	case "namespace/shutdown POST":
		data := NamespaceShutdownRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return ShutdownNamespace(data, c), nil
	case "namespace/reboot POST":
		data := NamespaceRebootRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return RebootNamespace(data, c), nil
	case "namespace/ingress-state/:state GET":
		data := NamespaceSetIngressStateRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return SetIngressState(data, c), nil
	case "namespace/pod-ids/:namespace GET":
		data := NamespacePodIdsRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return PodIds(data, c), nil
	case "namespace/get-cluster-pods GET":
		return ClusterPods(c), nil
	case "namespace/validate-cluster-pods POST":
		data := NamespaceValidateClusterPodsRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return ValidateClusterPods(data, c), nil
	case "namespace/validate-ports POST":
		data := NamespaceValidatePortsRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return ValidateClusterPorts(data, c), nil
	case "namespace/storage-size POST":
		data := NamespaceStorageSizeRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return StorageSize(data, c), nil
	case "service/create POST":
		data := ServiceCreateRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return CreateService(data, c), nil
	case "service/delete POST":
		data := ServiceDeleteRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return DeleteService(data, c), nil
	case "service/pod-ids/:namespace/:service GET":
		data := ServiceGetPodIdsRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return ServicePodIds(data, c), nil
	case "service/images/:imageName PATCH":
		data := ServiceSetImageRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return SetImage(data, c), nil
	case "service/log/:namespace/:podId GET":
		data := ServiceGetLogRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return PodLog(data, c), nil
	case "service/log-stream/:namespace/:podId/:sinceSeconds SSE":
		data := ServiceLogStreamRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		reader, err := PodLogStream(data, c)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return nil, reader
	case "service/resource-status/:resource/:namespace/:name/:statusOnly GET":
		data := ServiceResourceStatusRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return PodStatus(data, c), nil
	case "service/restart POST":
		data := ServiceRestartRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return Restart(data, c), nil
	case "service/stop POST":
		data := ServiceStopRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return StopService(data, c), nil
	case "service/start POST":
		data := ServiceStartRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return StartService(data, c), nil
	case "service/update-service POST":
		data := ServiceUpdateRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return UpdateService(data, c), nil
	case "service/spectrum-bind POST":
		data := ServiceBindSpectrumRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return BindSpectrum(data, c), nil
	case "service/spectrum-unbind DELETE":
		data := ServiceUnbindSpectrumRequest{}
		bytes, err := json.Marshal(datagram.Payload)
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram, nil
		}
		return UnbindSpectrum(data, c), nil
	case "service/spectrum-configmaps GET":
		return SpectrumConfigmaps(c), nil
	}

	datagram.Err = "Pattern not found"
	return datagram, nil
}

func HeartBeat(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return nil
}

func K8sNotification(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return nil
}
