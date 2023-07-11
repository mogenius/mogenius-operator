package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"os/exec"
	"strings"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
)

func CreateNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes namespace", job)
	cmd.Start(fmt.Sprintf("Creating namespace '%s'.", stage.Name))

	kubeProvider := NewKubeProvider()
	namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()
	newNamespace := applyconfcore.Namespace(stage.Name)

	applyOptions := metav1.ApplyOptions{
		Force:        true,
		FieldManager: DEPLOYMENTNAME,
	}

	newNamespace.WithLabels(map[string]string{
		"name": stage.Name,
	})

	_, err := namespaceClient.Apply(context.TODO(), newNamespace, applyOptions)
	if err != nil {
		cmd.Fail(fmt.Sprintf("CreateNamespace ERROR: %s", err.Error()))
	} else {
		cmd.Success(fmt.Sprintf("Created namespace '%s'.", newNamespace.Name))
	}
	return cmd
}

func DeleteNamespace(job *structs.Job, stage dtos.K8sStageDto, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes namespace", job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting namespace '%s'.", stage.Name))

		kubeProvider := NewKubeProvider()
		namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()

		err := namespaceClient.Delete(context.TODO(), stage.Name, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNamespace ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted namespace '%s'.", stage.Name))
		}
	}(cmd, wg)
	return cmd
}

func ListAllNamespaceNames() []string {
	result := []string{}

	kubeProvider := NewKubeProvider()
	namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()

	namespaceList, err := namespaceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("ListAll ERROR: %s", err.Error())
		return result
	}

	for _, ns := range namespaceList.Items {
		result = append(result, ns.Name)
	}

	return result
}

func ListAllNamespace() []v1.Namespace {
	result := []v1.Namespace{}

	kubeProvider := NewKubeProvider()
	namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()

	namespaceList, err := namespaceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("ListAllNamespace ERROR: %s", err.Error())
		return result
	}

	for _, ns := range namespaceList.Items {
		result = append(result, ns)
	}

	return result
}

func ListK8sNamespaces(namespaceName string) []v1.Namespace {
	result := []v1.Namespace{}

	kubeProvider := NewKubeProvider()
	namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()

	namespaceList, err := namespaceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("ListAllNamespace ERROR: %s", err.Error())
		return result
	}

	for _, ns := range namespaceList.Items {
		if namespaceName == "" {
			result = append(result, ns)
		} else {
			if strings.HasPrefix(ns.Name, namespaceName) {
				result = append(result, ns)
			}
		}
	}

	return result
}

func DeleteK8sNamespace(data v1.Namespace) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()
	err := namespaceClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}

func DescribeK8sNamespace(name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "namespace", name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(err.Error())
	}
	return WorkloadResult(string(output))
}

func NamespaceExists(namespaceName string) (bool, error) {
	kubeProvider := NewKubeProvider()
	namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()
	ns, err := namespaceClient.Get(context.TODO(), namespaceName, metav1.GetOptions{})
	return (ns != nil && err == nil), err
}
