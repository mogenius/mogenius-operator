package kubernetes

import (
	"fmt"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	appv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func WatchStatefulSets() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchStatefulSets(provider, "statefulsets")
	})

	// Wait forever
	select {}
}

func watchStatefulSets(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1.StatefulSet)
			castedObj.Kind = "StatefulSet"
			castedObj.APIVersion = "apps/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1.StatefulSet)
			castedObj.Kind = "StatefulSet"
			castedObj.APIVersion = "apps/v1"
			iacmanager.WriteResourceYaml(kindName, castedObj.Namespace, castedObj.Name, castedObj)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1.StatefulSet)
			castedObj.Kind = "StatefulSet"
			castedObj.APIVersion = "apps/v1"
			iacmanager.DeleteResourceYaml(kindName, castedObj.Namespace, castedObj.Name)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.AppsV1().RESTClient(),
		kindName,
		appv1.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1.StatefulSet{}, 0)
	resourceInformer.AddEventHandler(handler)

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
