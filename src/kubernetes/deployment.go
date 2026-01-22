package kubernetes

import (
	"context"
	"fmt"
	"mogenius-operator/src/store"
	"strings"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDeploymentsWithFieldSelector(namespace string, labelSelector string) ([]v1.Deployment, error) {
	clientset := clientProvider.K8sClientSet()
	client := clientset.AppsV1().Deployments(namespace)

	deployments, err := client.List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return []v1.Deployment{}, err
	}

	return deployments.Items, nil
}

func IsDeploymentInstalled(namespaceName string, name string) (string, error) {
	ownDeployment := store.GetDeployment(namespaceName, name)
	if ownDeployment == nil {
		return "", fmt.Errorf("deployment not found")
	}

	result := ""
	split := strings.Split(ownDeployment.Spec.Template.Spec.Containers[0].Image, ":")
	if len(split) > 1 {
		result = split[1]
	}

	return result, nil
}
