package kubernetes

import (
	"context"

	v1Core "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConfigMapFor(namespace string, configMapName string, showError bool) (*v1Core.ConfigMap, error) {
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().ConfigMaps(namespace)

	configMap, err := client.Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		if showError {
			k8sLogger.Error("ConfigMapFor", "error", err.Error())
		}
		return nil, err
	}

	configMap.Kind = "ConfigMap"
	configMap.APIVersion = "v1"
	return configMap, nil
}
