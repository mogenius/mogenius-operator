package kubernetes

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: Bene delete this file, as it is not used anymore.
func GetReplicaset(namespaceName string, name string) (*v1.ReplicaSet, error) {
	clientset := clientProvider.K8sClientSet()
	replicaSet, err := clientset.AppsV1().ReplicaSets(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	replicaSet.Kind = "ReplicaSet"
	replicaSet.APIVersion = "apps/v1"

	return replicaSet, err
}
