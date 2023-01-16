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

	logger.Log.Info(utils.FunctionName())
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
	}
	wg.Wait()
	job.Finish(c)
	return job
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
