package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"mogenius-k8s-manager/store"
	"slices"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func WatchAllResources() {
	provider, err := NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatalf("Error creating provider for watcher. Cannot continue: %s", err.Error())
		return
	}

	// Discover all resources in the cluster
	resources, err := provider.ClientSet.Discovery().ServerPreferredResources()
	if err != nil {
		K8sLogger.Fatalf("Error discovering resources: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	err = retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		for _, resourceList := range resources {
			for _, resource := range resourceList.APIResources {
				if slices.Contains(resource.Verbs, "list") && slices.Contains(resource.Verbs, "watch") {
					go func() {
						err := watchResource(provider, resource.Name, resource.Kind, resourceList.GroupVersion)
						if err != nil {
							K8sLogger.Errorf("failed to initialize watchhandler for resource %s %s: %s", resource.Kind, resourceList.GroupVersion, err.Error())
						}
					}()
				}
			}
		}
		return nil
	})
	if err != nil {
		K8sLogger.Fatalf("Error watching resources: %s", err.Error())
	}

	// Wait forever
	select {}
}

func watchResource(provider *KubeProvider, resourceName string, resourceKind string, groupVersion string) error {
	gv, err := schema.ParseGroupVersion(groupVersion)
	if err != nil {
		return fmt.Errorf("invalid groupVersion: %s", err)
	}

	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(provider.DynamicClient, time.Minute*10, v1.NamespaceAll, nil)

	gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: resourceName}
	resourceInformer := informerFactory.ForResource(gvr).Informer()

	err = resourceInformer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		if err == io.EOF {
			return // closed normally, its fine
		}
		K8sLogger.Errorf(`WatchError on Name('%s') Kind('%s') GroupVersion('%s'): %s`, resourceName, resourceKind, groupVersion, err)
	})
	if err != nil {
		return fmt.Errorf("failed to set error watch handler: %s", err)
	}
	_, err = resourceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			unstructuredObj, ok := obj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(obj)
				bodyString := string(body)
				K8sLogger.Warnf(`failed to deserialize: %s`, bodyString)
				return
			}
			SetStoreIfNeeded(resourceKind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj)
			IacManagerWriteResourceYaml(resourceKind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			unstructuredObj, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(newObj)
				bodyString := string(body)
				K8sLogger.Warnf(`failed to deserialize: %s`, bodyString)
				return
			}
			SetStoreIfNeeded(resourceKind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj)
			IacManagerWriteResourceYaml(resourceKind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj)
		},
		DeleteFunc: func(obj interface{}) {
			unstructuredObj, ok := obj.(*unstructured.Unstructured)
			if !ok {
				body, _ := json.Marshal(obj)
				bodyString := string(body)
				K8sLogger.Warnf(`failed to deserialize: %s`, bodyString)
				return
			}
			DeleteFromStoreIfNeeded(resourceKind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj)
			IacManagerDeleteResourceYaml(resourceKind, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), obj)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add eventhandler: %s", err.Error())
	}

	stopCh := make(chan struct{})
	go resourceInformer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced) {
		return fmt.Errorf("failed to sync cache for resource: %s", resourceKind)
	}

	return nil
}

func SetStoreIfNeeded(kind string, namespace string, name string, obj *unstructured.Unstructured) {
	if kind == "Deployment" || kind == "ReplicaSet" || kind == "CronJob" || kind == "Pod" || kind == "Job" || kind == "Event" {
		err := store.GlobalStore.Set(obj, kind, namespace, name)
		if err != nil {
			K8sLogger.Error(err)
		}
		if kind == "Event" {
			var event v1.Event
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &event)
			if err != nil {
				return
			}
			processEvent(&event)
		}
	}
}

func DeleteFromStoreIfNeeded(kind string, namespace string, name string, obj *unstructured.Unstructured) {
	if kind == "Deployment" || kind == "ReplicaSet" || kind == "CronJob" || kind == "Pod" || kind == "Job" || kind == "Event" {
		err := store.GlobalStore.Delete(kind, namespace, name)
		if err != nil {
			K8sLogger.Error(err)
		}
		if kind == "Event" {
			var event v1.Event
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &event)
			if err != nil {
				return
			}
			processEvent(&event)
		}
	}
	if kind == "PersistentVolume" {
		var pv v1.PersistentVolume
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pv)
		if err != nil {
			return
		}
		handlePVDeletion(&pv)
	}
}
