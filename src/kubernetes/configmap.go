package kubernetes

import (
	"context"

	v1Core "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AllConfigmaps(namespaceName string) []v1Core.ConfigMap {
	result := []v1Core.ConfigMap{}

	clientset := clientProvider.K8sClientSet()
	configmapList, err := clientset.CoreV1().ConfigMaps(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("AllConfigmaps", "error", err.Error())
		return result
	}

	for _, configmap := range configmapList.Items {
		configmap.Kind = "ConfigMap"
		configmap.APIVersion = "v1"
		result = append(result, configmap)
	}
	return result
}

// only create the configmap if it does not exist
func EnsureConfigMapExists(namespace string, configMap v1Core.ConfigMap) error {
	client := clientProvider.K8sClientSet().CoreV1().ConfigMaps(namespace)

	// check if the configmap already exists
	_, err := client.Get(context.TODO(), configMap.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = client.Create(context.TODO(), &configMap, MoCreateOptions(config))
			if err != nil {
				k8sLogger.Error("InitNetworkPolicyConfigMap", "error", err)
				return err
			}
		} else {
			k8sLogger.Error("InitNetworkPolicyConfigMap", "error", err)
			return err
		}
	}
	return nil
}

func GetConfigMap(namespace string, name string) v1Core.ConfigMap {
	client := clientProvider.K8sClientSet().CoreV1().ConfigMaps(namespace)

	cfgMap, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return v1Core.ConfigMap{}
	}
	return *cfgMap
}

func ConfigMapFor(namespace string, configMapName string, showError bool) *v1Core.ConfigMap {
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().ConfigMaps(namespace)

	configMap, err := client.Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		if showError {
			k8sLogger.Error("ConfigMapFor", "error", err.Error())
		}
		return nil
	}
	configMap.Kind = "ConfigMap"
	configMap.APIVersion = "v1"
	return configMap
}
