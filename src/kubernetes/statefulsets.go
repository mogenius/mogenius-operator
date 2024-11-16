package kubernetes

import (
	"context"

	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetStatefulSet(namespaceName string, name string) (*v1.StatefulSet, error) {
	provider, err := NewKubeProvider()
	if err != nil {
		return nil, err
	}
	statefulSet, err := provider.ClientSet.AppsV1().StatefulSets(namespaceName).Get(context.TODO(), name, metav1.GetOptions{})
	statefulSet.Kind = "StatefulSet"
	statefulSet.APIVersion = "apps/v1"

	return statefulSet, err
}
