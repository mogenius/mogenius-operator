package services

import (
	"mogenius-k8s-manager/src/dtos"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/websocket"
	"sync"
)

func CreateNamespace(eventClient websocket.WebsocketClient, r NamespaceCreateRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob(eventClient, "Create namespace "+r.Project.DisplayName+"/"+r.Namespace.DisplayName, r.Project.Id, r.Namespace.Name, "", serviceLogger)
	job.Start(eventClient)
	wg.Go(func() {
		CreateNamespaceCmds(eventClient, job, r)
	})

	go func() {
		wg.Wait()
		job.Finish(eventClient)
	}()

	return job
}

func CreateNamespaceCmds(eventClient websocket.WebsocketClient, job *structs.Job, r NamespaceCreateRequest) {
	mokubernetes.CreateNamespace(eventClient, job, r.Project, r.Namespace)
	// mokubernetes.CreateNetworkPolicyNamespace(job, r.Namespace, "allow-namespace-communication", wg)

	// if r.Project.ContainerRegistryUser != nil && r.Project.ContainerRegistryPat != nil {
	mokubernetes.CreateOrUpdateClusterImagePullSecret(eventClient, job, r.Project, r.Namespace)
	// }
}

func DeleteNamespace(eventClient websocket.WebsocketClient, r NamespaceDeleteRequest) *structs.Job {
	job := structs.CreateJob(
		eventClient,
		"Delete namespace "+r.Project.DisplayName+"/"+r.Namespace.DisplayName,
		r.Project.Id,
		r.Namespace.Name,
		"",
		serviceLogger,
	)
	job.Start(eventClient)

	go func() {
		mokubernetes.DeleteNamespace(eventClient, job, r.Namespace)
		job.Finish(eventClient)
	}()

	return job
}

type NamespaceCreateRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
}

type NamespaceDeleteRequest struct {
	Project   dtos.K8sProjectDto   `json:"project" validate:"required"`
	Namespace dtos.K8sNamespaceDto `json:"namespace" validate:"required"`
}

type NamespaceBackupRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}
