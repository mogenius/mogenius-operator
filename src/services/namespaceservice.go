package services

import (
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"sync"

	"helm.sh/helm/v3/pkg/action"
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
	// mokubernetes.CreateNetworkPolicyNamespace(job, r.Namespace, "allow-namespace-communication", wg)

	// if r.Project.ContainerRegistryUser != nil && r.Project.ContainerRegistryPat != nil {
	mokubernetes.CreateOrUpdateClusterImagePullSecret(job, r.Project, r.Namespace, wg)
	// }
}

func DeleteNamespace(r NamespaceDeleteRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Delete namespace "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, r.Namespace.Name, "")
	job.Start()
	mokubernetes.DeleteNamespace(job, r.Namespace, &wg)

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

func ValidateClusterPods(r NamespaceValidateClusterPodsRequest) dtos.ValidateClusterPodsDto {
	inDbButNotInCluster := []string{}
	clusterPodNames := kubernetes.AllPodNames()
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

func ValidateClusterPorts(r NamespaceValidatePortsRequest) {
	serviceLogger.Info("CleanupIngressPorts: received ports from DB.", "amountPorts", len(r.Ports), "ports", r.Ports)
	if len(r.Ports) <= 0 {
		serviceLogger.Error("Received empty ports list. Something seems wrong. Skipping process.")
		return
	}
	mokubernetes.CleanupIngressControllerServicePorts(r.Ports)
}

func ListAllNamespaces() []string {
	return kubernetes.ListAllNamespaceNames()
}

func ListAllResourcesForNamespace(r NamespaceGatherAllResourcesRequest) dtos.NamespaceResourcesDto {
	result := dtos.CreateNamespaceResourcesDto()
	result.Pods = kubernetes.AllPods(r.NamespaceName)
	result.Services = kubernetes.AllServices(r.NamespaceName)
	result.Deployments = kubernetes.AllDeployments(r.NamespaceName)
	result.Daemonsets = kubernetes.AllDaemonsets(r.NamespaceName)
	result.Replicasets = kubernetes.AllReplicasets(r.NamespaceName)
	result.Ingresses = kubernetes.AllIngresses(r.NamespaceName)
	result.Secrets = kubernetes.AllSecrets(r.NamespaceName)
	result.Configmaps = kubernetes.AllConfigmaps(r.NamespaceName)
	return result
}

type NamespaceCreateRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
}

type NamespaceDeleteRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
}

type NamespaceShutdownRequest struct {
	ProjectId string               `json:"projectId" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
	Service   dtos.K8sServiceDto   `json:"service" validate:"required"`
}

type NamespacePodIdsRequest struct {
	Namespace string `json:"namespace" validate:"required"`
}

type NamespaceValidateClusterPodsRequest struct {
	DbPodNames []string `json:"dbPodNames" validate:"required"`
}

type NamespaceValidatePortsRequest struct {
	Ports []dtos.NamespaceServicePortDto `json:"ports" validate:"required,dive"`
}

type NamespaceGatherAllResourcesRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

type NamespaceBackupRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

type NamespaceRestoreRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
	YamlData      string `json:"yamlData" validate:"required"`
}

type NamespaceResourceYamlRequest struct {
	NamespaceName string   `json:"namespaceName" validate:"required"`
	Resources     []string `json:"resources" validate:"required"`
}

type HelmDataRequest struct {
	Namespace  string                  `json:"namespace,omitempty"`
	Repo       string                  `json:"repo,omitempty"`
	ChartUrl   string                  `json:"chartUrl,omitempty"`
	Chart      string                  `json:"chart,omitempty"`
	Version    string                  `json:"version,omitempty"`
	Release    string                  `json:"release,omitempty"`
	Values     string                  `json:"values,omitempty"`
	DryRun     bool                    `json:"dryRun,omitempty"`
	ShowFormat action.ShowOutputFormat `json:"format,omitempty"`    // "all" "chart" "values" "readme" "crds"
	GetFormat  structs.HelmGetEnum     `json:"getFormat,omitempty"` // "all" "hooks" "manifest" "notes" "values"
	Revision   int                     `json:"revision,omitempty"`
}
