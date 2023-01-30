package services

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

func CreateNamespace(r NamespaceCreateRequest, c *websocket.Conn) utils.Job {
	var wg sync.WaitGroup

	job := utils.CreateJob("Create cloudspace "+r.Namespace.DisplayName+"/"+r.Stage.DisplayName, r.Namespace.Id, r.Stage.Id, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.CreateNamespace(&job, r.Namespace, r.Stage, c, &wg))
	if r.Stage.StorageSizeInMb > 0 {
		dataRoot, err := os.Getwd()
		if err != nil {
			logger.Log.Error(err.Error())
		}
		if utils.CONFIG.Kubernetes.RunInCluster {
			dataRoot = "/"
		}
		job.AddCmd(utils.CreateBashCommand("Create storage", &job, fmt.Sprintf("mkdir -p %s/mo-data/%s", dataRoot, r.Stage.Id), c))
		// TODO: IMPLEMENT THESE!!!!
		// job.add(this.createPV(stage))
		// job.add(this.createPVC(stage))
		logger.Log.Info("TODO: IMPLEMENT PV AND PC")
	}
	wg.Wait()
	job.Finish(c)
	return job
}

func DeleteNamespace(r NamespaceDeleteRequest, c *websocket.Conn) utils.Job {
	var wg sync.WaitGroup

	job := utils.CreateJob("Delete cloudspace "+r.Namespace.DisplayName+"/"+r.Stage.DisplayName, r.Namespace.Id, r.Stage.Id, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.DeleteNamespace(&job, r.Stage, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func ShutdownNamespace(r NamespaceShutdownRequest, c *websocket.Conn) bool {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return false
}

func RebootNamespace(r NamespaceRebootRequest, c *websocket.Conn) bool {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return false
}

func SetIngressState(r NamespaceSetIngressStateRequest, c *websocket.Conn) interface{} {
	// ENABLED = 'ENABLED',
	// DISABLED = 'DISABLED',
	// TRAFFIC_EXCEEDED = 'TRAFFIC_EXCEEDED'
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func PodIds(r NamespacePodIdsRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func ClusterPods(c *websocket.Conn) []string {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return []string{}
}

func ValidateClusterPods(r NamespaceValidateClusterPodsRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func ValidateClusterPorts(r NamespaceValidatePortsRequest, c *websocket.Conn) interface{} {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return nil
}

func StorageSize(r NamespaceStorageSizeRequest, c *websocket.Conn) map[string]int {
	// TODO: Implement
	logger.Log.Info("TODO: IMPLEMENT")
	logger.Log.Info(utils.FunctionName())
	return map[string]int{}
}

// namespace/create POST
type NamespaceCreateRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
}

func NamespaceCreateRequestExample() NamespaceCreateRequest {
	return NamespaceCreateRequest{
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Stage:     dtos.K8sStageDtoExampleData(),
	}
}

// namespace/delete POST
type NamespaceDeleteRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
}

func NamespaceDeleteRequestExample() NamespaceDeleteRequest {
	return NamespaceDeleteRequest{
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Stage:     dtos.K8sStageDtoExampleData(),
	}
}

// namespace/shutdown POST
type NamespaceShutdownRequest struct {
	NamespaceId string           `json:"namespaceId"`
	Stage       dtos.K8sStageDto `json:"stage"`
}

func NamespaceShutdownRequestExample() NamespaceShutdownRequest {
	return NamespaceShutdownRequest{
		NamespaceId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Stage:       dtos.K8sStageDtoExampleData(),
	}
}

// namespace/reboot POST
type NamespaceRebootRequest struct {
	NamespaceId string           `json:"namespaceId"`
	Stage       dtos.K8sStageDto `json:"stage"`
}

func NamespaceRebootRequestExample() NamespaceRebootRequest {
	return NamespaceRebootRequest{
		NamespaceId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Stage:       dtos.K8sStageDtoExampleData(),
	}
}

// namespace/ingress-state/:state GET
type NamespaceSetIngressStateRequest struct {
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Stage     dtos.K8sStageDto     `json:"stage"`
	State     string               `json:"state"`
}

func NamespaceSetIngressStateRequestExample() NamespaceSetIngressStateRequest {
	return NamespaceSetIngressStateRequest{
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Stage:     dtos.K8sStageDtoExampleData(),
		State:     "ENABLED",
	}
}

// namespace/pod-ids/:namespace GET
type NamespacePodIdsRequest struct {
	Namespace string `json:"namespace"`
}

func NamespacePodIdsRequestExample() NamespacePodIdsRequest {
	return NamespacePodIdsRequest{
		Namespace: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
	}
}

// namespace/get-cluster-pods GET

// namespace/validate-cluster-pods POST
type NamespaceValidateClusterPodsRequest struct {
	DbPodNames []string `json:"dbPodNames"`
}

func NamespaceValidateClusterPodsRequestExample() NamespaceValidateClusterPodsRequest {
	return NamespaceValidateClusterPodsRequest{
		DbPodNames: []string{"pod1", "pod2"},
	}
}

// namespace/validate-ports POST
type NamespaceValidatePortsRequest struct {
	Ports []dtos.NamespaceServicePortDto `json:"ports"`
}

func NamespaceValidatePortsRequestExample() NamespaceValidatePortsRequest {
	return NamespaceValidatePortsRequest{
		Ports: []dtos.NamespaceServicePortDto{
			dtos.NamespaceServicePortDtoExampleData(),
		},
	}
}

// namespace/storage-size POST
type NamespaceStorageSizeRequest struct {
	Stageids []string `json:"stageIds"`
}

func NamespaceStorageSizeRequestExample() NamespaceStorageSizeRequest {
	return NamespaceStorageSizeRequest{
		Stageids: []string{"stage1", "stage2"},
	}
}
