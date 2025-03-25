package kubernetes

import (
	"context"
	"strings"

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
			_, err = client.Create(context.TODO(), &configMap, MoCreateOptions())
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

func GetConfigMapWR(namespace string, name string) K8sWorkloadResult {
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().ConfigMaps(namespace)

	cfgMap, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(cfgMap.Data["data"], err)
}

func WriteConfigMap(namespace string, name string, data string, labels map[string]string) error {
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().ConfigMaps(namespace)

	cfgMap, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		newConfigMap := v1Core.ConfigMap{}
		newConfigMap.Data = make(map[string]string)
		newConfigMap.Name = name
		newConfigMap.Namespace = namespace
		newConfigMap.Labels = labels
		newConfigMap.Data["data"] = data
		_, err := client.Create(context.TODO(), &newConfigMap, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else if err == nil && cfgMap != nil {
		cfgMap.Data["data"] = data
		// merge new configmap labels with existing ones
		for key, value := range labels {
			cfgMap.Labels[key] = value
		}

		_, err := client.Update(context.TODO(), cfgMap, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		k8sLogger.Error("CreateOrUpdateConfigMap", "error", err)
		return err
	}
	return nil
}

func ListConfigMapWithFieldSelector(namespace string, labelSelector string, prefix string) K8sWorkloadResult {
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().ConfigMaps(namespace)

	cfgMaps, err := client.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return WorkloadResult(nil, err)
	}

	// delete all configmaps that do not start with prefix
	if prefix != "" {
		for i := len(cfgMaps.Items) - 1; i >= 0; i-- {
			if !strings.HasPrefix(cfgMaps.Items[i].Name, prefix) {
				cfgMaps.Items = append(cfgMaps.Items[:i], cfgMaps.Items[i+1:]...)
			}
		}
	}

	return WorkloadResult(cfgMaps.Items, err)
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

func UpdateK8sConfigMap(data v1Core.ConfigMap) error {
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().ConfigMaps(data.Namespace)

	_, err := client.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		k8sLogger.Error("UpdateK8sConfigMap", "error", err.Error())
		return err
	}
	return nil
}
