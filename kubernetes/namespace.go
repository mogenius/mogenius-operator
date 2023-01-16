package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
)

func CreateNamespace(job *utils.Job, namespace dtos.K8sNamespaceDto, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *utils.Command {
	cmd := utils.CreateCommand("Create Kubernetes namespace", job, c)
	wg.Add(1)
	go func(cmd *utils.Command, wg *sync.WaitGroup) {
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
			logger.Log.Errorf("CreateNamespace ERROR: %s", err.Error())
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

func DeleteNamespace(stage dtos.K8sStageDto) error {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		logger.Log.Errorf("DeleteNamespace ERROR: %s", err.Error())
	}

	namespaceClient := kubeProvider.ClientSet.CoreV1().Namespaces()

	logger.Log.Infof("Deleting namespace '%s'.", stage.K8sName)

	err = namespaceClient.Delete(context.TODO(), stage.K8sName, metav1.DeleteOptions{})
	if err != nil {
		logger.Log.Errorf("DeleteNamespace ERROR: %s", err.Error())
	} else {
		logger.Log.Infof("Deleted namespace '%s'.", stage.K8sName)
	}

	return err
}
