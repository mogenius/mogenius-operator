package services

import (
	"mogenius-k8s-manager/dtos"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"

	"github.com/gorilla/websocket"
)

var ALL_REQUESTS = []string{
	"HeartBeat",
	"ClusterStatus",
	"Test",
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
}

func ExecuteRequest(datagram structs.Datagram, c *websocket.Conn) interface{} {
	switch datagram.Pattern {
	case "HeartBeat":
		return HeartBeat(datagram, c)
	case "ClusterStatus":
		return mokubernetes.ClusterStatus()
	case "cicd/build-info GET":
		return BuildInfo(datagram.Payload.(BuildInfoRequest))
	case "cicd/build-info-array POST":
		return BuildInfoArray(datagram.Payload.(BuildInfoArrayRequest))
	case "cicd/build-log GET":
		return BuildLog(datagram.Payload.(BuildLogRequest))
	case "files/storage-stats GET":
		return AllFiles()
	case "files/list POST":
		return List(datagram.Payload.(FilesListRequest))
	case "files/download POST":
		return Download(datagram.Payload.(FilesDownloadRequest))
	case "files/upload POST":
		return Upload(datagram.Payload.(FilesUploadRequest))
	case "files/update POST":
		return Update(datagram.Payload.(FilesUpdateRequest))
	case "files/create-folder POST":
		return CreateFolder(datagram.Payload.(FilesCreateFolderRequest))
	case "files/rename POST":
		return Rename(datagram.Payload.(FilesRenameRequest))
	case "files/chown POST":
		return Chown(datagram.Payload.(FilesChownRequest))
	case "files/chmod POST":
		return Chmod(datagram.Payload.(FilesChmodRequest))
	case "files/delete POST":
		return Delete(datagram.Payload.(FilesDeleteRequest))
	case "namespace/create POST":
		return CreateNamespace(datagram.Payload.(NamespaceCreateRequest))
	case "namespace/delete POST":
		return DeleteNamespace(datagram.Payload.(NamespaceDeleteRequest))
	case "namespace/shutdown POST":
		return ShutdownNamespace(datagram.Payload.(NamespaceShutdownRequest))
	case "namespace/reboot POST":
		return RebootNamespace(datagram.Payload.(NamespaceRebootRequest))
	case "namespace/ingress-state/:state GET":
		return SetIngressState(datagram.Payload.(NamespaceSetIngressStateRequest))
	case "namespace/pod-ids/:namespace GET":
		return PodIds(datagram.Payload.(NamespacePodIdsRequest))
	case "namespace/get-cluster-pods GET":
		return ClusterPods()
	case "namespace/validate-cluster-pods POST":
		return ValidateClusterPods(datagram.Payload.(NamespaceValidateClusterPodsRequest))
	case "namespace/validate-ports POST":
		return ValidateClusterPorts(datagram.Payload.(NamespaceValidatePortsRequest))
	case "namespace/storage-size POST":
		return StorageSize(datagram.Payload.(NamespaceStorageSizeRequest))
	case "service/create POST":
		return CreateService(datagram.Payload.(ServiceCreateRequest))
	case "service/delete POST":
		return DeleteService(datagram.Payload.(ServiceDeleteRequest))
	case "service/pod-ids/:namespace/:service GET":
		return ServicePodIds(datagram.Payload.(ServiceGetPodIdsRequest))
	case "service/images/:imageName PATCH":
		return SetImage(datagram.Payload.(ServiceSetImageRequest))
	case "service/log/:namespace/:podId GET":
		return PodLog(datagram.Payload.(ServiceGetLogRequest))
	case "service/log-stream/:namespace/:podId/:sinceSeconds SSE":
		return PodLogStream(datagram.Payload.(ServiceLogStreamRequest))
	case "service/resource-status/:resource/:namespace/:name/:statusOnly GET":
		return PodStatus(datagram.Payload.(ServiceResourceStatusRequest))
	case "service/build POST":
		return Build(datagram.Payload.(ServiceBuildRequest))
	case "service/restart POST":
		return Restart(datagram.Payload.(ServiceRestartRequest))
	case "service/stop POST":
		return StopService(datagram.Payload.(ServiceStopRequest))
	case "service/start POST":
		return StartService(datagram.Payload.(ServiceStartRequest))
	case "service/update-service POST":
		return UpdateService(datagram.Payload.(ServiceUpdateRequest))
	case "service/spectrum-bind POST":
		return BindSpectrum(datagram.Payload.(ServiceBindSpectrumRequest))
	case "service/spectrum-unbind DELETE":
		return UnbindSpectrum(datagram.Payload.(ServiceUnbindSpectrumRequest))
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
	}

	datagram.Err = "Pattern not found"
	return datagram
}

func HeartBeat(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Info(utils.FunctionName())
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return nil
}

func TestUpdateIngress(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Info(utils.FunctionName())
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return mokubernetes.UpdateIngress(dtos.K8sNamespaceDtoExampleData(), dtos.K8sStageDtoExampleData(), nil, nil)
}

func TestCreateNamespace(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Info(utils.FunctionName())
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return mokubernetes.CreateNamespace(dtos.K8sStageDtoExampleData())
}

func TestDeleteNamespace(d structs.Datagram, c *websocket.Conn) interface{} {
	logger.Log.Info(utils.FunctionName())
	logger.Log.Infof("Received '%s' from %s", d.Pattern, c.RemoteAddr().String())
	return mokubernetes.DeleteNamespace(dtos.K8sStageDtoExampleData())
}

func ReportState() {
	// TODO: implement me
}
