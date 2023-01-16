package services

import (
	"encoding/json"
	"mogenius-k8s-manager/dtos"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"

	"github.com/gorilla/websocket"
)

var ALL_REQUESTS = []string{
	"HeartBeat",
	"K8sNotification",
	"ClusterStatus",
	"cicd/build-info GET",
	"cicd/build-info-array POST",
	"cicd/build-log GET",
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
	"servcie/create POST",
	"service/delete POST",
	"service/pod-ids/:namespace/:serviceId GET",
	"service/images/:imageName PATCH",
	"service/log/:namespace/:podId GET",
	"service/log-stream/:namespace/:podId/:sinceSeconds SSE",
	"service/resource-status/:resource/:namespace/:name/:statusOnly GET",
	"service/build POST",
	"service/restart POST",
	"service/stop POST",
	"service/start POST",
	"service/update-service POST",
	"service/spectrum-bind POST",
	"service/spectrum-unbind DELETE",
	"service/spectrum-configmaps GET",
}

var ALL_TESTS = []string{
	"TestCreateNamespace",
	"TestDeleteNamespace",
	"TestUpdateIngress",
	"TestCreatePV",
}

func ExecuteRequest(datagram structs.Datagram, c *websocket.Conn) interface{} {
	switch datagram.Pattern {
	case "HeartBeat":
		return HeartBeat(datagram, c)
	case "K8sNotification":
		return K8sNotification(datagram, c)
	case "ClusterStatus":
		return mokubernetes.ClusterStatus()
	case "cicd/build-info GET":
		data := BuildInfoRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return BuildInfo(data)
	case "cicd/build-info-array POST":
		data := BuildInfoArrayRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return BuildInfoArray(data)
	case "cicd/build-log GET":
		data := BuildLogRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return BuildLog(data)
	case "files/storage-stats GET":
		return AllFiles()
	case "files/list POST":
		data := FilesListRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return List(data)
	case "files/download POST":
		data := FilesDownloadRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Download(data)
	case "files/upload POST":
		data := FilesUploadRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Upload(data)
	case "files/update POST":
		data := FilesUpdateRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Update(data)
	case "files/create-folder POST":
		data := FilesCreateFolderRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return CreateFolder(data)
	case "files/rename POST":
		data := FilesRenameRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Rename(data)
	case "files/chown POST":
		data := FilesChownRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Chown(data)
	case "files/chmod POST":
		data := FilesChmodRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Chmod(data)
	case "files/delete POST":
		data := FilesDeleteRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Delete(data)
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
		return DeleteNamespace(data)
	case "namespace/shutdown POST":
		data := NamespaceShutdownRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return ShutdownNamespace(data)
	case "namespace/reboot POST":
		data := NamespaceRebootRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return RebootNamespace(data)
	case "namespace/ingress-state/:state GET":
		data := NamespaceSetIngressStateRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return SetIngressState(data)
	case "namespace/pod-ids/:namespace GET":
		data := NamespacePodIdsRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return PodIds(data)
	case "namespace/get-cluster-pods GET":
		return ClusterPods()
	case "namespace/validate-cluster-pods POST":
		data := NamespaceValidateClusterPodsRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return ValidateClusterPods(data)
	case "namespace/validate-ports POST":
		data := NamespaceValidatePortsRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return ValidateClusterPorts(data)
	case "namespace/storage-size POST":
		data := NamespaceStorageSizeRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return StorageSize(data)
	case "service/create POST":
		data := ServiceCreateRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return CreateService(data)
	case "service/delete POST":
		data := ServiceDeleteRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return DeleteService(data)
	case "service/pod-ids/:namespace/:service GET":
		data := ServiceGetPodIdsRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return ServicePodIds(data)
	case "service/images/:imageName PATCH":
		data := ServiceSetImageRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return SetImage(data)
	case "service/log/:namespace/:podId GET":
		data := ServiceGetLogRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return PodLog(data)
	case "service/log-stream/:namespace/:podId/:sinceSeconds SSE":
		data := ServiceLogStreamRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return PodLogStream(data)
	case "service/resource-status/:resource/:namespace/:name/:statusOnly GET":
		data := ServiceResourceStatusRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return PodStatus(data)
	case "service/build POST":
		data := ServiceBuildRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Build(data)
	case "service/restart POST":
		data := ServiceRestartRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return Restart(data)
	case "service/stop POST":
		data := ServiceStopRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return StopService(data)
	case "service/start POST":
		data := ServiceStartRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return StartService(data)
	case "service/update-service POST":
		data := ServiceUpdateRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return UpdateService(data)
	case "service/spectrum-bind POST":
		data := ServiceBindSpectrumRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return BindSpectrum(data)
	case "service/spectrum-unbind DELETE":
		data := ServiceUnbindSpectrumRequest{}
		err := json.Unmarshal([]byte(datagram.Payload), &data)
		if err != nil {
			datagram.Err = err.Error()
			return datagram
		}
		return UnbindSpectrum(data)
	case "service/spectrum-configmaps GET":
		return SpectrumConfigmaps()
	}

	switch datagram.Pattern {
	case "TestCreateNamespace":
		return TestCreateNamespace(datagram, c)
	case "TestDeleteNamespace":
		return TestDeleteNamespace(datagram, c)
	case "TestUpdateIngress":
		return TestUpdateIngress(datagram, c)
	case "TestCreatePV":
		return TestCreatePv(datagram, c)
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

func TestUpdateIngress(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return mokubernetes.UpdateIngress(dtos.K8sNamespaceDtoExampleData(), dtos.K8sStageDtoExampleData(), nil, nil)
}

func TestCreateNamespace(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	data := NamespaceCreateRequest{}
	err := json.Unmarshal([]byte(d.Payload), &data)
	if err != nil {
		d.Err = err.Error()
		return d
	}
	return CreateNamespace(data, c)
}

func TestDeleteNamespace(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return mokubernetes.DeleteNamespace(dtos.K8sStageDtoExampleData())
}

func TestCreatePv(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return mokubernetes.CreatePersistentVolume(dtos.K8sStageDtoExampleData())
}
