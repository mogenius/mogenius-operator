package kubernetes

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: Bene delete this file, as it is not used anymore.
func GetK8sDaemonset(namespaceName string, name string) (*v1.DaemonSet, error) {
	clientset := clientProvider.K8sClientSet()
	daemonset, err := clientset.AppsV1().DaemonSets(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	daemonset.Kind = "DaemonSet"
	daemonset.APIVersion = "apps/v1"

	return daemonset, err
}
