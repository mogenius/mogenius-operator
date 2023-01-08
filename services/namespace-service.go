package services

import "mogenius-k8s-manager/dtos"

func CreateNamespace(r NamespaceCreateRequest) bool {
	// TODO: Implement
	return false
}

func DeleteNamespace(r NamespaceDeleteRequest) bool {
	// TODO: Implement
	return false
}

func ShutdownNamespace(r NamespaceShutdownRequest) bool {
	// TODO: Implement
	return false
}

func RebootNamespace(r NamespaceRebootRequest) bool {
	// TODO: Implement
	return false
}

func SetIngressState(r NamespaceSetIngressStateRequest) interface{} {
	// ENABLED = 'ENABLED',
	// DISABLED = 'DISABLED',
	// TRAFFIC_EXCEEDED = 'TRAFFIC_EXCEEDED'
	// TODO: Implement
	return nil
}

func PodIds(r NamespacePodIdsRequest) interface{} {
	// TODO: Implement
	return nil
}

func ClusterPods() []string {
	// TODO: Implement
	return []string{}
}

func ValidateClusterPods(r NamespaceValidateClusterPodsRequest) interface{} {
	// TODO: Implement
	return nil
}

func ValidateClusterPorts(r NamespaceValidatePortsRequest) interface{} {
	// TODO: Implement
	return nil
}

func StorageSize(r NamespaceStorageSizeRequest) map[string]int {
	// TODO: Implement
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
