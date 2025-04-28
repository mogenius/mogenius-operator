package kubernetes

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllDaemonsets(namespaceName string) []v1.DaemonSet {
	result := []v1.DaemonSet{}

	clientset := clientProvider.K8sClientSet()
	daemonsetList, err := clientset.AppsV1().DaemonSets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("AllDaemonsets", "error", err.Error())
		return result
	}

	for _, daemonset := range daemonsetList.Items {
		daemonset.Kind = "DaemonSet"
		daemonset.APIVersion = "apps/v1"
		result = append(result, daemonset)
	}
	return result
}

func GetK8sDaemonset(namespaceName string, name string) (*v1.DaemonSet, error) {
	clientset := clientProvider.K8sClientSet()
	daemonset, err := clientset.AppsV1().DaemonSets(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	daemonset.Kind = "DaemonSet"
	daemonset.APIVersion = "apps/v1"

	return daemonset, err
}
