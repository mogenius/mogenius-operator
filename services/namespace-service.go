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
)

func CreateNamespace(r NamespaceCreateRequest) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Create cloudspace "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, &r.Namespace.Id, nil)
	job.Start()
	job.AddCmds(CreateNamespaceCmds(&job, r, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func CreateNamespaceCmds(job *structs.Job, r NamespaceCreateRequest, wg *sync.WaitGroup) []*structs.Command {
	cmds := []*structs.Command{}
	cmds = append(cmds, mokubernetes.CreateNamespace(job, r.Project, r.Namespace))
	cmds = append(cmds, mokubernetes.CreateNetworkPolicyNamespace(job, r.Namespace, wg))

	if r.Project.ContainerRegistryUser != "" && r.Project.ContainerRegistryPat != "" {
		cmds = append(cmds, mokubernetes.CreateOrUpdateContainerSecret(job, r.Project, r.Namespace, wg))
	}
	return cmds
}

func DeleteNamespace(r NamespaceDeleteRequest) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Delete cloudspace "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, &r.Namespace.Id, nil)
	job.Start()
	job.AddCmd(mokubernetes.DeleteNamespace(&job, r.Namespace, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func ShutdownNamespace(r NamespaceShutdownRequest) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Shutdown Stage "+r.Namespace.DisplayName, r.ProjectId, &r.Namespace.Id, nil)
	job.Start()
	job.AddCmd(mokubernetes.StopDeployment(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.DeleteService(&job, r.Namespace, r.Service, &wg))
	job.AddCmd(mokubernetes.UpdateIngress(&job, r.Namespace, nil, nil, &wg))
	wg.Wait()
	job.Finish()
	return job
}

func PodIds(r NamespacePodIdsRequest) interface{} {
	return kubernetes.PodIdsFor(r.Namespace, nil)
}

func ValidateClusterPods(r NamespaceValidateClusterPodsRequest) dtos.ValidateClusterPodsDto {
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

func ValidateClusterPorts(r NamespaceValidatePortsRequest) interface{} {
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
	Project   dtos.K8sProjectDto   `json:"project"`
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
}

func NamespaceCreateRequestExample() NamespaceCreateRequest {
	return NamespaceCreateRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
	}
}

type NamespaceDeleteRequest struct {
	Project   dtos.K8sProjectDto   `json:"project"`
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
}

func NamespaceDeleteRequestExample() NamespaceDeleteRequest {
	return NamespaceDeleteRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
	}
}

type NamespaceShutdownRequest struct {
	ProjectId string               `json:"projectId"`
	Namespace dtos.K8sNamespaceDto `json:"namespace"`
	Service   dtos.K8sServiceDto   `json:"service"`
}

func NamespaceShutdownRequestExample() NamespaceShutdownRequest {
	return NamespaceShutdownRequest{
		ProjectId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
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
