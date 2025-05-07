package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/websocket"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
)

func CreateNamespace(eventClient websocket.WebsocketClient, job *structs.Job, project dtos.K8sProjectDto, namespace dtos.K8sNamespaceDto) {
	cmd := structs.CreateCommand(eventClient, "create", "Create Kubernetes namespace", job)
	cmd.Start(eventClient, job, "Creating namespace")

	clientset := clientProvider.K8sClientSet()
	namespaceClient := clientset.CoreV1().Namespaces()
	newNamespace := applyconfcore.Namespace(namespace.Name)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: GetOwnDeploymentName(config),
	}

	newNamespace.WithLabels(MoUpdateLabels(&map[string]string{"name": namespace.Name}, &project.Id, &namespace, nil, config))

	_, err := namespaceClient.Apply(context.TODO(), newNamespace, applyOptions)
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("CreateNamespace ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Created namespace")
	}
}

func DeleteNamespace(eventClient websocket.WebsocketClient, job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand(eventClient, "create", "Delete Kubernetes namespace", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(eventClient, job, "Deleting namespace")

		clientset := clientProvider.K8sClientSet()
		namespaceClient := clientset.CoreV1().Namespaces()

		err := namespaceClient.Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(eventClient, job, fmt.Sprintf("DeleteNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(eventClient, job, "Deleted namespace")
		}
	}(wg)
}

func NamespaceExists(namespaceName string) (bool, error) {
	clientset := clientProvider.K8sClientSet()
	namespaceClient := clientset.CoreV1().Namespaces()
	ns, err := namespaceClient.Get(context.TODO(), namespaceName, metav1.GetOptions{})
	return (ns != nil && err == nil), err
}

func ListAllNamespaceNames() []string {
	result := []string{}

	clientset := clientProvider.K8sClientSet()
	namespaceClient := clientset.CoreV1().Namespaces()

	namespaceList, err := namespaceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("ListAll", "error", err.Error())
		return result
	}

	for _, ns := range namespaceList.Items {
		result = append(result, ns.Name)
	}

	return result
}
