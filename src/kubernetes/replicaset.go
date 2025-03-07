package kubernetes

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllReplicasets(namespaceName string) []v1.ReplicaSet {
	result := []v1.ReplicaSet{}

	clientset := clientProvider.K8sClientSet()
	replicaSetList, err := clientset.AppsV1().ReplicaSets(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("AllReplicasets", "error", err.Error())
		return result
	}

	for _, replicaSet := range replicaSetList.Items {
		replicaSet.Kind = "ReplicaSet"
		replicaSet.APIVersion = "apps/v1"
		result = append(result, replicaSet)
	}
	return result
}

func GetReplicaset(namespaceName string, name string) (*v1.ReplicaSet, error) {
	clientset := clientProvider.K8sClientSet()
	replicaSet, err := clientset.AppsV1().ReplicaSets(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	replicaSet.Kind = "ReplicaSet"
	replicaSet.APIVersion = "apps/v1"

	return replicaSet, err
}
