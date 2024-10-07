package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	punq "github.com/mogenius/punq/kubernetes"

	v1Core "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
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
// 			GracePeriodSeconds: punqUtils.Pointer[int64](5),
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
				K8sLogger.Errorf("InitNetworkPolicyConfigMap creation ERROR: %s", err)
				return err
			}
		} else {
			K8sLogger.Errorf("InitNetworkPolicyConfigMap get ERROR: %s", err)
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
		K8sLogger.Errorf("CreateOrUpdateConfigMap ERROR: %s", err.Error())
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

func WatchConfigmaps() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	err = retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchConfigmaps(provider, "configmaps")
	})
	if err != nil {
		K8sLogger.Fatalf("Error watching configmaps: %s", err.Error())
	}

	// Wait forever
	select {}
}

func watchConfigmaps(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1Core.ConfigMap)
			castedObj.Kind = "ConfigMap"
			castedObj.APIVersion = "v1"
			IacManagerWriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1Core.ConfigMap)
			castedObj.Kind = "ConfigMap"
			castedObj.APIVersion = "v1"
			IacManagerWriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1Core.ConfigMap)
			castedObj.Kind = "ConfigMap"
			castedObj.APIVersion = "v1"
			IacManagerDeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, obj)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.CoreV1().RESTClient(),
		kindName,
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1Core.ConfigMap{}, 0)
	_, err := resourceInformer.AddEventHandler(handler)
	if err != nil {
		return err
	}

	stopCh := make(chan struct{})
	go resourceInformer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	// This loop will keep the function alive as long as the stopCh is not closed
	for {
		select {
		case <-stopCh:
			// stopCh closed, return from the function
			return nil
		case <-time.After(30 * time.Second):
			// This is to avoid a tight loop in case stopCh is never closed.
			// You can adjust the time as per your needs.
		}
	}
}
