package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/dtos"

	"mogenius-k8s-manager/structs"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/util/retry"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

func EventWatcher() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching events with exponential backoff in case of failures
	retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchEvents(provider)
	})

	// Wait forever
	select {}
}

func ResourceWatcher() {
	go WatchConfigmaps()
	go WatchDeployments()
	go WatchPods()
	go WatchIngresses()
	go WatchSecrets()
	go WatchServices()
	go WatchNamespaces()
	go WatchNetworkPolicies()
	go WatchJobs()
	go WatchCronJobs()
}

func processEvent(event *v1Core.Event) (string, error) {
	if event != nil {
		eventDto := dtos.CreateEvent(string(event.Type), event)
		datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto)
		message := event.Message
		kind := event.InvolvedObject.Kind
		reason := event.Reason
		count := event.Count
		structs.EventServerSendData(datagram, kind, reason, message, count)
		return event.ObjectMeta.ResourceVersion, nil
	} else {
		return "", fmt.Errorf("malformed event received")
	}
}

func watchEvents(provider *punq.KubeProvider) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			event := obj.(*v1Core.Event)
			processEvent(event)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			event := newObj.(*v1Core.Event)
			processEvent(event)
		},
		DeleteFunc: func(obj interface{}) {
			event := obj.(*v1Core.Event)
			processEvent(event)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.CoreV1().RESTClient(),
		"events",
		v1Core.NamespaceAll,
		fields.Nothing(),
	)
	eventInformer := cache.NewSharedInformer(listWatch, &v1Core.Event{}, 0)
	eventInformer.AddEventHandler(handler)

	stopCh := make(chan struct{})
	go eventInformer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, eventInformer.HasSynced) {
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
