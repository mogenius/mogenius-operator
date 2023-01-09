package services

import (
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
)

func CreateNamespace(r NamespaceCreateRequest) bool {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return false
}

func DeleteNamespace(r NamespaceDeleteRequest) bool {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return false
}

func ShutdownNamespace(r NamespaceShutdownRequest) bool {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return false
}

func RebootNamespace(r NamespaceRebootRequest) bool {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return false
}

func SetIngressState(r NamespaceSetIngressStateRequest) interface{} {
	// ENABLED = 'ENABLED',
	// DISABLED = 'DISABLED',
	// TRAFFIC_EXCEEDED = 'TRAFFIC_EXCEEDED'
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func PodIds(r NamespacePodIdsRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func ClusterPods() []string {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return []string{}
}

func ValidateClusterPods(r NamespaceValidateClusterPodsRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func ValidateClusterPorts(r NamespaceValidatePortsRequest) interface{} {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return nil
}

func StorageSize(r NamespaceStorageSizeRequest) map[string]int {
	// TODO: Implement
	logger.Log.Info(utils.FunctionName())
	return map[string]int{}
}

// namespace/create POST
type NamespaceCreateRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
}

// namespace/delete POST
type NamespaceDeleteRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
}

// namespace/shutdown POST
type NamespaceShutdownRequest struct {
	NamespaceId string           `json:"namespaceId"`
	Stage       dtos.K8sStageDto `json:"stage"`
}

// namespace/reboot POST
type NamespaceRebootRequest struct {
	NamespaceId string           `json:"namespaceId"`
	Stage       dtos.K8sStageDto `json:"stage"`
}

// namespace/ingress-state/:state GET
type NamespaceSetIngressStateRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
	State     string               `json:"state"`
}

// namespace/pod-ids/:namespace GET
type NamespacePodIdsRequest struct {
	Namespace string `json:"namespace"`
}

// namespace/get-cluster-pods GET

// namespace/validate-cluster-pods POST
type NamespaceValidateClusterPodsRequest struct {
	DbPodNames []string `json:"dbPodNames"`
}

// namespace/validate-ports POST
type NamespaceValidatePortsRequest struct {
	Ports []dtos.NamespaceServicePortDto `json:"ports"`
}

// namespace/storage-size POST
type NamespaceStorageSizeRequest struct {
	Stageids []string `json:"stageIds"`
}
