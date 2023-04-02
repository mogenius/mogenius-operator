package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
)

func CreateNamespace(job *structs.Job, namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create Kubernetes namespace", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating namespace '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNamespace ERROR: %s", err.Error()), c)
			return
		}

		namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()
		namespace := applyconfcore.Namespace(stage.K8sName)

		applyOptions := metav1.ApplyOptions{
			Force:        true,
			FieldManager: DEPLOYMENTNAME,
		}

		namespace.WithLabels(map[string]string{
			"name": stage.K8sName,
		})

		_, err = namespaceClient.Apply(context.TODO(), namespace, applyOptions)
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateNamespace ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created namespace '%s'.", *namespace.Name), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteNamespace(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete Kubernetes namespace", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting namespace '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}

		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNamespace ERROR: %s", err.Error()), c)
			return
		}

		namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()

		err = namespaceClient.Delete(context.TODO(), stage.K8sName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteNamespace ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Deleted namespace '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func ListAllNamespaceNames() []string {
	result := []string{}
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		logger.Log.Errorf("ListAll ERROR: %s", err.Error())
		return result
	}

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
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		logger.Log.Errorf("ListAllNamespace ERROR: %s", err.Error())
		return result
	}

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

func DeleteK8sNamespace(data v1.Namespace) K8sWorkloadResult {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		return WorkloadResult(err.Error())
	}

	namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()
	err = namespaceClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(err.Error())
	}
	return WorkloadResult("")
}
