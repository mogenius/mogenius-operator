package kubernetes

import (
	"context"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfcore "k8s.io/client-go/applyconfigurations/core/v1"
)

func CreateNamespace(stage dtos.K8sStageDto) error {
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

	logger.Log.Infof("Creating namespace '%s'.", *namespace.Name)

	_, err = namespaceClient.Apply(context.TODO(), namespace, applyOptions)
	if err != nil {
		logger.Log.Errorf("CreateNamespace ERROR: %s", err.Error())
	} else {
		logger.Log.Infof("Created namespace '%s'.", namespace.Name)
	}

	return err
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
