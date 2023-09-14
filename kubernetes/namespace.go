package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
)

func CreateNamespace(job *structs.Job, project dtos.K8sProjectDto, namespace dtos.K8sNamespaceDto) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes namespace", job)
	cmd.Start(fmt.Sprintf("Creating namespace '%s'.", namespace.Name))

	kubeProvider := punq.NewKubeProvider(nil)
	namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()
	newNamespace := applyconfcore.Namespace(namespace.Name)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	newNamespace.WithLabels(map[string]string{
		"name": namespace.Name,
	})

	_, err := namespaceClient.Apply(context.TODO(), newNamespace, applyOptions)
	if err != nil {
		cmd.Fail(fmt.Sprintf("CreateNamespace ERROR: %s", err.Error()))
	} else {
		cmd.Success(fmt.Sprintf("Created namespace '%s'.", newNamespace.Name))
	}
	return cmd
}

func DeleteNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes namespace", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting namespace '%s'.", namespace.Name))

		kubeProvider := punq.NewKubeProvider(nil)
		namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()

		err := namespaceClient.Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted namespace '%s'.", namespace.Name))
		}
	}(cmd, wg)
	return cmd
}
