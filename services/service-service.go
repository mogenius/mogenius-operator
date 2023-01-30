package services

import (
	"mogenius-k8s-manager/dtos"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
)

func CreateService(r ServiceCreateRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup

	job := utils.CreateJob("Create Service "+r.Namespace.DisplayName+"/"+r.Stage.DisplayName, r.Namespace.Id, r.Stage.Id, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.CreateService(&job, r.Stage, r.Service, nil, nil, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func DeleteService(r ServiceDeleteRequest, c *websocket.Conn) interface{} {
	var wg sync.WaitGroup

	job := utils.CreateJob("Delete Service "+r.Namespace.DisplayName+"/"+r.Stage.DisplayName, r.Namespace.Id, r.Stage.Id, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.DeleteService(&job, r.Stage, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func SetImage(r ServiceSetImageRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func ServicePodIds(r ServiceGetPodIdsRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func PodLog(r ServiceGetLogRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func PodLogStream(r ServiceLogStreamRequest, c *websocket.Conn) interface{} {
	// TODO: Implement XXX WAS AN OBSERVABLE - SSE - written directly to response
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func PodStatus(r ServiceResourceStatusRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func Restart(r ServiceRestartRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func StopService(r ServiceStopRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func StartService(r ServiceStartRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func UpdateService(r ServiceUpdateRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func BindSpectrum(r ServiceBindSpectrumRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func UnbindSpectrum(r ServiceUnbindSpectrumRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func SpectrumConfigmaps(c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

// service/create POST
type ServiceCreateRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceCreateRequestExample() ServiceCreateRequest {
	return ServiceCreateRequest{
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Stage:     dtos.K8sStageDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

// service/delete POST
type ServiceDeleteRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func ServiceDeleteRequestExample() ServiceDeleteRequest {
	return ServiceDeleteRequest{
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Stage:     dtos.K8sStageDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

// service/pod-ids/:namespace/:serviceId GET
type ServiceGetPodIdsRequest struct {
	Namespace string `json:"namespace"`
	ServiceId string `json:"serviceId"`
}

func ServiceGetPodIdsRequestExample() ServiceGetPodIdsRequest {
	return ServiceGetPodIdsRequest{
		Namespace: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ServiceId: "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
}

// service/images/:imageName PATCH
type ServiceSetImageRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
	ImageName   string `json:"imageName"`
}

func ServiceSetImageRequestExample() ServiceSetImageRequest {
	return ServiceSetImageRequest{
		NamespaceId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Stage:       "73AD838E-BDEC-4D5E-BBEB-C5E4EF0D94BF",
		ServiceId:   "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
		ImageName:   "test",
	}
}

// service/log/:namespace/:podId GET
type ServiceGetLogRequest struct {
	Namespace string `json:"namespace"`
	PodId     string `json:"podId"`
}

func ServiceGetLogRequestExample() ServiceGetLogRequest {
	return ServiceGetLogRequest{
		Namespace: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		PodId:     "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
}

// service/log-stream/:namespace/:podId/:sinceSeconds SSE
type ServiceLogStreamRequest struct {
	Namespace    string `json:"namespace"`
	PodId        string `json:"podId"`
	SinceSeconds int    `json:"sinceSeconds"`
}

func ServiceLogStreamRequestExample() ServiceLogStreamRequest {
	return ServiceLogStreamRequest{
		Namespace:    "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		PodId:        "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
		SinceSeconds: 0,
	}
}

// service/resource-status/:resource/:namespace/:name/:statusOnly GET
type ServiceResourceStatusRequest struct {
	Resource    string `json:"resource"`
	NamespaceId string `json:"namespaceId"`
	Name        string `json:"name"`
	StatusOnly  bool   `json:"statusOnly"`
}

func ServiceResourceStatusRequestExample() ServiceResourceStatusRequest {
	return ServiceResourceStatusRequest{
		Resource:    "deployment",
		NamespaceId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Name:        "test",
		StatusOnly:  true,
	}
}

// service/restart POST
type ServiceRestartRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
}

func ServiceRestartRequestExample() ServiceRestartRequest {
	return ServiceRestartRequest{
		NamespaceId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Stage:       "73AD838E-BDEC-4D5E-BBEB-C5E4EF0D94BF",
		ServiceId:   "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
}

// service/stop POST
type ServiceStopRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
}

func ServiceStopRequestExample() ServiceStopRequest {
	return ServiceStopRequest{
		NamespaceId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Stage:       "73AD838E-BDEC-4D5E-BBEB-C5E4EF0D94BF",
		ServiceId:   "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
}

// service/start POST
type ServiceStartRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
}

func ServiceStartRequestExample() ServiceStartRequest {
	return ServiceStartRequest{
		NamespaceId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Stage:       "73AD838E-BDEC-4D5E-BBEB-C5E4EF0D94BF",
		ServiceId:   "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
}

// service/update-service POST
type ServiceUpdateRequest struct {
	NamespaceId string `json:"namespaceId"`
	Stage       string `json:"stage"`
	ServiceId   string `json:"serviceId"`
}

func ServiceUpdateRequestExample() ServiceUpdateRequest {
	return ServiceUpdateRequest{
		NamespaceId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Stage:       "73AD838E-BDEC-4D5E-BBEB-C5E4EF0D94BF",
		ServiceId:   "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
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

func ServiceBindSpectrumRequestExample() ServiceBindSpectrumRequest {
	return ServiceBindSpectrumRequest{
		K8sNamespaceName: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		K8sServiceName:   "73AD838E-BDEC-4D5E-BBEB-C5E4EF0D94BF",
		ExternalPort:     8080,
		InternalPort:     80,
		Type:             "http",
		NamespaceId:      "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
}

// service/spectrum-unbind DELETE
type ServiceUnbindSpectrumRequest struct {
	ExternalPort int    `json:"externalPort"`
	Type         string `json:"type"`
	NamespaceId  string `json:"namespaceId"`
}

func ServiceUnbindSpectrumRequestExample() ServiceUnbindSpectrumRequest {
	return ServiceUnbindSpectrumRequest{
		ExternalPort: 8080,
		Type:         "http",
		NamespaceId:  "DAF08780-9C55-4A56-BF3C-471FEEE93C41",
	}
}

// service/spectrum-configmaps GET
