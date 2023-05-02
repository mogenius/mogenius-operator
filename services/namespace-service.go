package services

import (
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/kubernetes"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

func CreateNamespace(r NamespaceCreateRequest, c *websocket.Conn) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Create cloudspace "+r.Namespace.DisplayName+"/"+r.Stage.DisplayName, r.Namespace.Id, &r.Stage.Id, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.CreateNamespace(&job, r.Namespace, r.Stage, c))
	if r.Stage.StorageSizeInMb > 0 {
		// dataRoot, err := os.Getwd()
		// if err != nil {
		// 	logger.Log.Error(err.Error())
		// }
		// if utils.CONFIG.Kubernetes.RunInCluster {
		// 	dataRoot = "/"
		// }
		//job.AddCmd(structs.CreateBashCommand("Create storage", &job, fmt.Sprintf("mkdir -p %s/mo-data/%s", dataRoot, r.Stage.Id), c, &wg))
		job.AddCmd(mokubernetes.CreateNetworkPolicyNamespace(&job, r.Stage, c, &wg))
		//job.AddCmd(mokubernetes.CreatePersistentVolumeClaim(&job, r.Stage, c, &wg))
		if r.Namespace.ContainerRegistryUser != "" && r.Namespace.ContainerRegistryPat != "" {
			job.AddCmd(mokubernetes.CreateContainerSecret(&job, r.Namespace, r.Stage, c, &wg))
		}
	}
	wg.Wait()
	job.Finish(c)
	return job
}

func DeleteNamespace(r NamespaceDeleteRequest, c *websocket.Conn) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Delete cloudspace "+r.Namespace.DisplayName+"/"+r.Stage.DisplayName, r.Namespace.Id, &r.Stage.Id, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.DeleteNamespace(&job, r.Stage, c, &wg))
	//job.AddCmd(mokubernetes.DeletePersistentVolume(&job, r.Stage, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func ShutdownNamespace(r NamespaceShutdownRequest, c *websocket.Conn) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Shutdown Stage "+r.Stage.DisplayName, r.NamespaceId, &r.Stage.Id, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.StopDeployment(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.DeleteService(&job, r.Stage, r.Service, c, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Stage, nil, nil, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func PodIds(r NamespacePodIdsRequest, c *websocket.Conn) interface{} {
	return kubernetes.PodIdsFor(r.Namespace, nil)
}

func ValidateClusterPods(r NamespaceValidateClusterPodsRequest, c *websocket.Conn) dtos.ValidateClusterPodsDto {
	inDbButNotInCluster := []string{}
	clusterPodNames := mokubernetes.AllPodNames()
	for index, dbPodName := range r.DbPodNames {
		if !utils.Contains(clusterPodNames, dbPodName) {
			inDbButNotInCluster = append(inDbButNotInCluster, dbPodName)
		} else {
			clusterPodNames = utils.Remove(clusterPodNames, index)
		}
	}
	return dtos.ValidateClusterPodsDto{
		InDbButNotInCluster: inDbButNotInCluster,
		InClusterButNotInDb: clusterPodNames,
	}
}

func ValidateClusterPorts(r NamespaceValidatePortsRequest, c *websocket.Conn) interface{} {
	logger.Log.Infof("CleanupIngressPorts: %d ports received from DB.", len(r.Ports))
	if len(r.Ports) <= 0 {
		logger.Log.Error("Received empty ports list. Something seems wrong. Skipping process.")
		return nil
	}
	mokubernetes.CleanupIngressControllerServicePorts(r.Ports)

	return nil
}

func ListAllNamespaces() []string {
	return mokubernetes.ListAllNamespaceNames()
}

func StorageSize(r NamespaceStorageSizeRequest, c *websocket.Conn) map[string]int {
	// TODO: Implement for CephFS
	result := make(map[string]int)
	for _, v := range r.Stageids {
		result[v] = 0
	}
	return result
}

func ListAllResourcesForNamespace(r NamespaceGatherAllResourcesRequest) dtos.NamespaceResourcesDto {
	result := dtos.CreateNamespaceResourcesDto()
	result.Pods = mokubernetes.AllPods(r.NamespaceName)
	result.Services = mokubernetes.AllServices(r.NamespaceName)
	result.Deployments = mokubernetes.AllDeployments(r.NamespaceName)
	result.Daemonsets = mokubernetes.AllDaemonsets(r.NamespaceName)
	result.Replicasets = mokubernetes.AllReplicasets(r.NamespaceName)
	result.Ingresses = mokubernetes.AllIngresses(r.NamespaceName)
	result.Secrets = mokubernetes.AllSecrets(r.NamespaceName)
	result.Configmaps = mokubernetes.AllConfigmaps(r.NamespaceName)
	return result
}

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

type NamespaceShutdownRequest struct {
	NamespaceId string             `json:"namespaceId"`
	Stage       dtos.K8sStageDto   `json:"stage"`
	Service     dtos.K8sServiceDto `json:"service"`
}

func NamespaceShutdownRequestExample() NamespaceShutdownRequest {
	return NamespaceShutdownRequest{
		NamespaceId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Stage:       dtos.K8sStageDtoExampleData(),
		Service:     dtos.K8sServiceDtoExampleData(),
	}
}

type NamespacePodIdsRequest struct {
	Namespace string `json:"namespace"`
}

func NamespacePodIdsRequestExample() NamespacePodIdsRequest {
	return NamespacePodIdsRequest{
		Namespace: "mogenius",
	}
}

type NamespaceValidateClusterPodsRequest struct {
	DbPodNames []string `json:"dbPodNames"`
}

func NamespaceValidateClusterPodsRequestExample() NamespaceValidateClusterPodsRequest {
	return NamespaceValidateClusterPodsRequest{
		DbPodNames: []string{"pod1", "pod2", "pod3", "pod4", "pod5", "mo-traffic-collector-mo7-nnnqv"},
	}
}

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

type NamespaceStorageSizeRequest struct {
	Stageids []string `json:"stageIds"`
}

func NamespaceStorageSizeRequestExample() NamespaceStorageSizeRequest {
	return NamespaceStorageSizeRequest{
		Stageids: []string{"stage1", "stage2", "stage3", "stage4", "stage5"},
	}
}

type NamespaceGatherAllResourcesRequest struct {
	NamespaceName string `json:"namespaceName"`
}

func NamespaceGatherAllResourcesRequestExample() NamespaceGatherAllResourcesRequest {
	return NamespaceGatherAllResourcesRequest{
		NamespaceName: "mogenius",
	}
}

type NamespaceBackupRequest struct {
	NamespaceName string `json:"namespaceName"`
}

func NamespaceBackupRequestExample() NamespaceBackupRequest {
	return NamespaceBackupRequest{
		NamespaceName: "mogenius",
	}
}

type NamespaceRestoreRequest struct {
	NamespaceName string `json:"namespaceName"`
	YamlData      string `json:"yamlData"`
}

func NamespaceRestoreRequestExample() NamespaceRestoreRequest {
	// IF backup.yaml exists. use it. otherwise. used default workload
	data := ""
	fileData, err := os.ReadFile("backup.yaml")
	if err != nil {
		fileData, _ = os.ReadFile("example-workload.yaml")
		data = string(fileData)
	} else {
		data = string(fileData)
	}
	return NamespaceRestoreRequest{
		NamespaceName: "mogenius-test",
		YamlData:      data,
	}
}
