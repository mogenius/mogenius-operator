package kubernetes

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetStatefulSet(namespaceName string, name string) (*v1.StatefulSet, error) {
	clientset := clientProvider.K8sClientSet()
	statefulSet, err := clientset.AppsV1().StatefulSets(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	statefulSet.Kind = "StatefulSet"
	statefulSet.APIVersion = "apps/v1"

	return statefulSet, err
}
