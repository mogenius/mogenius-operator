package kubernetes

import (
	"context"
	"strings"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllDaemonsets(namespaceName string) []v1.DaemonSet {
	result := []v1.DaemonSet{}

	provider, err := NewKubeProvider()
	if err != nil {
		return result
	}
	daemonsetList, err := provider.ClientSet.AppsV1().DaemonSets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
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

func IsDaemonSetInstalled(namespaceName string, name string) (string, error) {
	ownDaemonset, err := GetK8sDaemonset(namespaceName, name)
	if err != nil {
		return "", err
	}

	result := ""
	split := strings.Split(ownDaemonset.Spec.Template.Spec.Containers[0].Image, ":")
	if len(split) > 1 {
		result = split[1]
	}

	return result, nil
}

func GetK8sDaemonset(namespaceName string, name string) (*v1.DaemonSet, error) {
	provider, err := NewKubeProvider()
	if err != nil {
		return nil, err
	}
	daemonset, err := provider.ClientSet.AppsV1().DaemonSets(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	daemonset.Kind = "DaemonSet"
	daemonset.APIVersion = "apps/v1"

	return daemonset, err
}
