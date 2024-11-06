package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/store"
	"mogenius-k8s-manager/structs"
	"reflect"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
)

func CreateNamespace(job *structs.Job, project dtos.K8sProjectDto, namespace dtos.K8sNamespaceDto) {
	cmd := structs.CreateCommand("create", "Create Kubernetes namespace", job)
	cmd.Start(job, "Creating namespace")

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
	}
	namespaceClient := provider.ClientSet.CoreV1().Namespaces()
	newNamespace := applyconfcore.Namespace(namespace.Name)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	newNamespace.WithLabels(MoUpdateLabels(&map[string]string{"name": namespace.Name}, &project.Id, &namespace, nil))

	_, err = namespaceClient.Apply(context.TODO(), newNamespace, applyOptions)
	if err != nil {
		cmd.Fail(job, fmt.Sprintf("CreateNamespace ERROR: %s", err.Error()))
	} else {
		cmd.Success(job, "Created namespace")
	}
}

func DeleteNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Delete Kubernetes namespace", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting namespace")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		namespaceClient := provider.ClientSet.CoreV1().Namespaces()

		err = namespaceClient.Delete(context.TODO(), namespace.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted namespace")
		}
	}(wg)
}

func ListAllNamespaces() ([]coreV1.Namespace, error) {
	result := []coreV1.Namespace{}
	namespaces, err := store.GlobalStore.SearchByPrefix(reflect.TypeOf(coreV1.Namespace{}), "Namespace")

	if err != nil {
		return result, err
	}

	for _, ref := range namespaces {
		if ref == nil {
			continue
		}

		namespace := ref.(*coreV1.Namespace)
		if namespace == nil {
			continue
		}

		result = append(result, *namespace)
	}

	return result, nil
}

func GetNamespace(name string) *coreV1.Namespace {
	ref, err := store.GlobalStore.Get(fmt.Sprintf("%s-%s", "Namespace", name), reflect.TypeOf(coreV1.Namespace{}))
	if err != nil {
		return nil
	}

	if ref == nil {
		return nil
	}

	namespace := ref.(*coreV1.Namespace)
	if namespace == nil {
		return nil
	}

	return namespace
}
