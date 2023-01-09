package services

import (
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
)

func CreateService(r ServiceCreateRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func DeleteService(r ServiceDeleteRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func SetImage(r ServiceSetImageRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func ServicePodIds(r ServiceGetPodIdsRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func PodLog(r ServiceGetLogRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func PodLogStream(r ServiceLogStreamRequest) interface{} {
	// TODO: Implement XXX WAS AN OBSERVABLE - SSE - written directly to response
	logger.Log.Info(utils.FunctionName())
	return nil
}

func PodStatus(r ServiceResourceStatusRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Build(r ServiceBuildRequest) interface{} {
	// TODO: REMOVE ME! das hier soll in API-Service
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Restart(r ServiceRestartRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func StopService(r ServiceStopRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func StartService(r ServiceStartRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func UpdateService(r ServiceUpdateRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func BindSpectrum(r ServiceBindSpectrumRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func UnbindSpectrum(r ServiceUnbindSpectrumRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func SpectrumConfigmaps() interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

// service/create POST
type ServiceCreateRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

// service/delete POST
type ServiceDeleteRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
}

// service/pod-ids/:namespace/:serviceId GET
type ServiceGetPodIdsRequest struct {
	Namespace string `json:"namespace"`
	ServiceId string `json:"serviceId"`
}

// service/images/:imageName PATCH
type ServiceSetImageRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
	ImageName   string `json:"imageName"`
}

// service/log/:namespace/:podId GET
type ServiceGetLogRequest struct {
	Namespace string `json:"namespace"`
	PodId     string `json:"podId"`
}

// service/log-stream/:namespace/:podId/:sinceSeconds SSE
type ServiceLogStreamRequest struct {
	Namespace    string `json:"namespace"`
	PodId        string `json:"podId"`
	SinceSeconds int    `json:"sinceSeconds"`
}

// service/resource-status/:resource/:namespace/:name/:statusOnly GET
type ServiceResourceStatusRequest struct {
	Resource    string `json:"resource"`
	NamespaceId string `json:"namespaceId"`
	Name        string `json:"name"`
	StatusOnly  bool   `json:"statusOnly"`
}

// TODO: das gehört hier nicht mehr rein. soll in API-Service
// service/build POST
type ServiceBuildRequest struct {
	NamespaceId   string `json:"namespaceId"`
	Stage         string `json:"stage"`
	ServiceId     string `json:"serviceId"`
	CommitHash    string `json:"commitHash"`
	CommitAuthor  string `json:"commitAuthor"`
	CommitMessage string `json:"commitMessage"`
}

// service/restart POST
type ServiceRestartRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
}

// service/stop POST
type ServiceStopRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
}

// service/start POST
type ServiceStartRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
}

// service/update-service POST
type ServiceUpdateRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
}

// service/spectrum-bind POST
type ServiceBindSpectrumRequest struct {
	K8sNamespaceName string `json:"k8sNamespaceName"`
	K8sServiceName   string `json:"k8sServiceName"`
	ExternalPort     int    `json:"externalPort"`
	InternalPort     int    `json:"internalPort"`
	Type             string `json:"type"`
	NamespaceId      string `json:"namespaceId"`
}

// service/spectrum-unbind DELETE
type ServiceUnbindSpectrumRequest struct {
	ExternalPort int    `json:"externalPort"`
	Type         string `json:"type"`
	NamespaceId  string `json:"namespaceId"`
}

// service/spectrum-configmaps GET
