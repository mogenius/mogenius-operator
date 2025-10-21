package kubernetes

import (
	"context"
	"strings"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getDeployment(namespaceName string, controllerName string) (*v1.Deployment, error) {
	clientset := clientProvider.K8sClientSet()
	client := clientset.AppsV1().Deployments(namespaceName)
	return client.Get(context.Background(), controllerName, metav1.GetOptions{})
}

func GetDeploymentsWithFieldSelector(namespace string, labelSelector string) ([]v1.Deployment, error) {
	result := []v1.Deployment{}
	clientset := clientProvider.K8sClientSet()
	client := clientset.AppsV1().Deployments(namespace)

	deployments, err := client.List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return result, err
	}

	return deployments.Items, err
}

func IsDeploymentInstalled(namespaceName string, name string) (string, error) {
	ownDeployment, err := getDeployment(namespaceName, name)
	if err != nil {
		return "", err
	}

	result := ""
	split := strings.Split(ownDeployment.Spec.Template.Spec.Containers[0].Image, ":")
	if len(split) > 1 {
		result = split[1]
	}

	return result, nil
}

func AllDeploymentsIncludeIgnored(namespaceName string) []v1.Deployment {
	result := []v1.Deployment{}
	clientset := clientProvider.K8sClientSet()
	deploymentList, err := clientset.AppsV1().Deployments(namespaceName).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("AllDeployment", "error", err.Error())
		return result
	}

	for _, deployment := range deploymentList.Items {
		deployment.Kind = "Deployment"
		deployment.APIVersion = "apps/v1"
		result = append(result, deployment)
	}

	return result
}
