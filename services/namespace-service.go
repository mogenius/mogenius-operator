package services

import (
	"mogenius-k8s-manager/crds"
	"mogenius-k8s-manager/dtos"
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"os"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
)

func CreateNamespace(r NamespaceCreateRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Create namespace "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, r.Namespace.Name, "")
	job.Start()
	CreateNamespaceCmds(job, r, &wg)

	go func() {
		wg.Wait()
		job.Finish()
	}()

	return job
}

func CreateNamespaceCmds(job *structs.Job, r NamespaceCreateRequest, wg *sync.WaitGroup) {
	mokubernetes.CreateNamespace(job, r.Project, r.Namespace)
	mokubernetes.CreateNetworkPolicyNamespace(job, r.Namespace, wg)

	if r.Project.ContainerRegistryUser != nil && r.Project.ContainerRegistryPat != nil {
		mokubernetes.CreateOrUpdateContainerSecret(job, r.Project, r.Namespace, wg)
	}
	crds.CreateEnvironmentCmd(job, r.Project.Name, r.Namespace.Name, crds.CrdEnvironment{
		Id:          r.Namespace.Id,
		DisplayName: r.Namespace.DisplayName,
		CreatedBy:   "MISSING_FIELD",
		Name:        r.Namespace.Name}, wg)
}

func DeleteNamespace(r NamespaceDeleteRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Delete namespace "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, r.Namespace.Name, "")
	job.Start()
	mokubernetes.DeleteNamespace(job, r.Namespace, &wg)

	crds.DeleteEnvironmentCmd(job, r.Project.Name, r.Namespace.Name, &wg)

	go func() {
		wg.Wait()
		job.Finish()
	}()

	return job
}

func ShutdownNamespace(r NamespaceShutdownRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Shutdown Stage "+r.Namespace.DisplayName, r.ProjectId, r.Namespace.Name, r.Service.ControllerName)
	job.Start()
	mokubernetes.StopDeployment(job, r.Namespace, r.Service, &wg)
	mokubernetes.DeleteService(job, r.Namespace, r.Service, &wg)
	mokubernetes.UpdateIngress(job, r.Namespace, r.Service, &wg)

	go func() {
		wg.Wait()
		job.Finish()
	}()

	return job
}

func PodIds(r NamespacePodIdsRequest) interface{} {
	return punq.PodIdsFor(r.Namespace, nil, nil)
}

func ValidateClusterPods(r NamespaceValidateClusterPodsRequest) dtos.ValidateClusterPodsDto {
	inDbButNotInCluster := []string{}
	clusterPodNames := punq.AllPodNames(nil)
	for index, dbPodName := range r.DbPodNames {
		if !punqUtils.Contains(clusterPodNames, dbPodName) {
			inDbButNotInCluster = append(inDbButNotInCluster, dbPodName)
		} else {
			clusterPodNames = punqUtils.Remove(clusterPodNames, index)
		}
	}
	return dtos.ValidateClusterPodsDto{
		InDbButNotInCluster: inDbButNotInCluster,
		InClusterButNotInDb: clusterPodNames,
	}
}

func ValidateClusterPorts(r NamespaceValidatePortsRequest) interface{} {
	ServiceLogger.Infof("CleanupIngressPorts: %d ports received from DB.", len(r.Ports))
	if len(r.Ports) <= 0 {
		ServiceLogger.Error("Received empty ports list. Something seems wrong. Skipping process.")
		return nil
	}
	mokubernetes.CleanupIngressControllerServicePorts(r.Ports)

	return nil
}

func ListAllNamespaces() []string {
	return punq.ListAllNamespaceNames(nil)
}

func ListAllResourcesForNamespace(r NamespaceGatherAllResourcesRequest) dtos.NamespaceResourcesDto {
	result := dtos.CreateNamespaceResourcesDto()
	result.Pods = punq.AllPods(r.NamespaceName, nil)
	result.Services = punq.AllServices(r.NamespaceName, nil)
	result.Deployments = punq.AllDeployments(r.NamespaceName, nil)
	result.Daemonsets = punq.AllDaemonsets(r.NamespaceName, nil)
	result.Replicasets = punq.AllReplicasets(r.NamespaceName, nil)
	result.Ingresses = punq.AllIngresses(r.NamespaceName, nil)
	result.Secrets = punq.AllSecrets(r.NamespaceName, nil)
	result.Configmaps = punq.AllConfigmaps(r.NamespaceName, nil)
	return result
}

type ProjectCreateRequest struct {
	Project crds.CrdProject `json:"project" validate:"required"`
}

func ProjectCreateRequestExample() ProjectCreateRequest {
	return ProjectCreateRequest{
		Project: crds.CrdProjectExampleData(),
	}
}

type ProjectUpdateRequest struct {
	Id          string             `json:"id" validate:"required"`
	ProjectName string             `json:"projectName"`
	DisplayName string             `json:"displayName"`
	ProductId   string             `json:"productId"`
	Limits      crds.ProjectLimits `json:"limits"`
}

func ProjectUpdateRequestExample() ProjectUpdateRequest {
	return ProjectUpdateRequest{
		Id:          "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ProjectName: "mogenius",
		DisplayName: "displayName",
		Limits: crds.ProjectLimits{
			LimitMemoryMB:      1024,
			LimitCpuCores:      1.0,
			EphemeralStorageMB: 1024,
			MaxVolumeSizeGb:    10,
		},
	}
}

type ProjectDeleteRequest struct {
	ProjectName string `json:"projectName" validate:"required"`
	ProjectId   string `json:"projectId" validate:"required"`
}

func ProjectDeleteRequestExample() ProjectDeleteRequest {
	return ProjectDeleteRequest{
		ProjectName: "mogenius",
		ProjectId:   "B0919ACB-92DD-416C-AF67-E59AD4B25265",
	}
}

type NamespaceCreateRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
}

func NamespaceCreateRequestExample() NamespaceCreateRequest {
	return NamespaceCreateRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
	}
}

type NamespaceDeleteRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
}

func NamespaceDeleteRequestExample() NamespaceDeleteRequest {
	return NamespaceDeleteRequest{
		Project:   dtos.K8sProjectDtoExampleData(),
		Namespace: dtos.K8sNamespaceDtoExampleData(),
	}
}

type NamespaceShutdownRequest struct {
	ProjectId string               `json:"projectId" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

func NamespaceShutdownRequestExample() NamespaceShutdownRequest {
	return NamespaceShutdownRequest{
		ProjectId: "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		Namespace: dtos.K8sNamespaceDtoExampleData(),
		Service:   dtos.K8sServiceDtoExampleData(),
	}
}

type NamespacePodIdsRequest struct {
	Namespace string `json:"namespace" validate:"required"`
}

func NamespacePodIdsRequestExample() NamespacePodIdsRequest {
	return NamespacePodIdsRequest{
		Namespace: "mogenius",
	}
}

type NamespaceValidateClusterPodsRequest struct {
	DbPodNames []string `json:"dbPodNames" validate:"required"`
}

func NamespaceValidateClusterPodsRequestExample() NamespaceValidateClusterPodsRequest {
	return NamespaceValidateClusterPodsRequest{
		DbPodNames: []string{"pod1", "pod2", "pod3", "pod4", "pod5", "mo-traffic-collector-mo7-nnnqv"},
	}
}

type NamespaceValidatePortsRequest struct {
	Ports []dtos.NamespaceServicePortDto `json:"ports" validate:"required,dive"`
}

func NamespaceValidatePortsRequestExample() NamespaceValidatePortsRequest {
	return NamespaceValidatePortsRequest{
		Ports: []dtos.NamespaceServicePortDto{
			dtos.NamespaceServicePortDtoExampleData(),
		},
	}
}

type NamespaceGatherAllResourcesRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

func NamespaceGatherAllResourcesRequestExample() NamespaceGatherAllResourcesRequest {
	return NamespaceGatherAllResourcesRequest{
		NamespaceName: "mogenius",
	}
}

type NamespaceBackupRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

func NamespaceBackupRequestExample() NamespaceBackupRequest {
	return NamespaceBackupRequest{
		NamespaceName: "mogenius",
	}
}

type NamespaceRestoreRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
	YamlData      string `json:"yamlData" validate:"required"`
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

type NamespaceResourceYamlRequest struct {
	NamespaceName string   `json:"namespaceName" validate:"required"`
	Resources     []string `json:"resources" validate:"required"`
}

func NamespaceResourceYamlRequestExample() NamespaceResourceYamlRequest {
	return NamespaceResourceYamlRequest{
		NamespaceName: "default",
		Resources:     []string{"pod", "service", "deployment", "daemonset", "ingress", "secret", "configmap"},
	}
}
