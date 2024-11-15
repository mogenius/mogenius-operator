package kubernetes

import (
	"context"
	"strings"

	punq "github.com/mogenius/punq/kubernetes"

	v1Core "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// func DeleteConfigMap(job *structs.Job, namespace dtos.K8sNamespaceDto, service dtos.K8sServiceDto, wg *sync.WaitGroup) {
// 	cmd := structs.CreateCommand("delete", "Delete Kubernetes configMap", job)
// 	wg.Add(1)
// 	go func(wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(job, "Deleting configMap")

// 		provider, err := punq.NewKubeProvider(nil)
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
// 			return
// 		}
// 		configMapClient := provider.ClientSet.CoreV1().ConfigMaps(namespace.Name)

// 		deleteOptions := metav1.DeleteOptions{
// 			GracePeriodSeconds: utils[int64](5),
// 		}

// 		err = configMapClient.Delete(context.TODO(), service.ControllerName, deleteOptions)
// 		if err != nil {
// 			cmd.Fail(job, fmt.Sprintf("DeleteConfigMap ERROR: %s", err.Error()))
// 		} else {
// 			cmd.Success(job, "Deleted configMap")
// 		}
// 	}(wg)
// }

// func AddKeyToConfigMap(job *structs.Job, namespace string, configMapName string, key string, value string, wg *sync.WaitGroup) {
// 	cmd := structs.CreateCommand("update", "Update Kubernetes configMap", job)
// 	wg.Add(1)
// 	go func(wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(job, "Updating configMap")

// 		configMap := punq.ConfigMapFor(namespace, configMapName, false, nil)
// 		if configMap != nil {
// 			provider, err := punq.NewKubeProvider(nil)
// 			if err != nil {
// 				cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
// 				return
// 			}
// 			configMapClient := provider.ClientSet.CoreV1().ConfigMaps(namespace)
// 			configMap.Data[key] = value

// 			_, err = configMapClient.Update(context.TODO(), configMap, metav1.UpdateOptions{})
// 			if err != nil {
// 				cmd.Fail(job, fmt.Sprintf("UpdateConfigMap ERROR: %s", err.Error()))
// 				return
// 			} else {
// 				cmd.Success(job, "Update configMap")
// 				return
// 			}
// 		}
// 		cmd.Fail(job, fmt.Sprintf("ConfigMap '%s/%s' not found.", namespace, configMapName))
// 	}(wg)
// }

// func RemoveKeyFromConfigMap(job *structs.Job, namespace string, configMapName string, key string, wg *sync.WaitGroup) {
// 	cmd := structs.CreateCommand("update", "Update Kubernetes configMap", job)
// 	wg.Add(1)
// 	go func(wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(job, "Update Kubernetes configMap.")

// 		configMap := punq.ConfigMapFor(namespace, configMapName, false, nil)
// 		if configMap != nil {
// 			if configMap.Data == nil {
// 				cmd.Success(job, "ConfigMap contains no data. No key was removed.")
// 				return
// 			} else {
// 				delete(configMap.Data, key)

// 				provider, err := punq.NewKubeProvider(nil)
// 				if err != nil {
// 					cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
// 					return
// 				}
// 				updateOptions := metav1.UpdateOptions{
// 					FieldManager: DEPLOYMENTNAME,
// 				}
// 				configMapClient := provider.ClientSet.CoreV1().ConfigMaps(namespace)
// 				_, err = configMapClient.Update(context.TODO(), configMap, updateOptions)
// 				if err != nil {
// 					cmd.Fail(job, fmt.Sprintf("RemoveKey ERROR: %s", err.Error()))
// 					return
// 				}
// 				cmd.Success(job, fmt.Sprintf("Key %s successfully removed.", key))
// 				return
// 			}
// 		}
// 		cmd.Fail(job, fmt.Sprintf("ConfigMap '%s/%s' not found.", namespace, configMapName))
// 	}(wg)
// }

// only create the configmap if it does not exist
func EnsureConfigMapExists(namespace string, configMap v1Core.ConfigMap) error {
	client := GetCoreClient().ConfigMaps(namespace)

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
	client := GetCoreClient().ConfigMaps(namespace)

	cfgMap, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return v1Core.ConfigMap{}
	}
	return *cfgMap
}

func GetConfigMapWR(namespace string, name string) K8sWorkloadResult {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	client := provider.ClientSet.CoreV1().ConfigMaps(namespace)

	cfgMap, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(cfgMap.Data["data"], err)
}

func WriteConfigMap(namespace string, name string, data string, labels map[string]string) error {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return err
	}
	client := provider.ClientSet.CoreV1().ConfigMaps(namespace)

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
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	client := provider.ClientSet.CoreV1().ConfigMaps(namespace)

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
