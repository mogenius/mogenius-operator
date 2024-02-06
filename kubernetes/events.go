package kubernetes

import (
	"context"
	"fmt"
	"log"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	v1Core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"

	"k8s.io/client-go/tools/cache"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

// TODO: REMOVE if NewEventWatcher works
// TODO: REMOVE if NewEventWatcher works
// TODO: REMOVE if NewEventWatcher works
func WatchEvents() {
	var lastResourceVersion = ""
	for {
		provider, err := punq.NewKubeProvider(nil)
		if provider == nil || err != nil {
			logger.Log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
			return
		}

		// Create a watcher for all Kubernetes events
		watcher, err := provider.ClientSet.CoreV1().Events("").Watch(context.TODO(), v1.ListOptions{Watch: true, ResourceVersion: lastResourceVersion})

		if err != nil || watcher == nil {
			if apierrors.IsGone(err) {
				lastResourceVersion = ""
			}
			log.Printf("Error creating watcher: %v", err)
			continue
		} else {
			logger.Log.Notice("Watcher connected successfully. Start watching events...")
		}

		// Start watching events
		for event := range watcher.ResultChan() {
			//fmt.Println(event)
			if event.Object != nil {
				eventDto := dtos.CreateEvent(string(event.Type), event.Object)
				datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto)

				eventObj, isEvent := event.Object.(*v1Core.Event)
				if isEvent {
					lastResourceVersion = eventObj.ObjectMeta.ResourceVersion
					message := eventObj.Message
					kind := eventObj.InvolvedObject.Kind
					reason := eventObj.Reason
					count := eventObj.Count
					// if currentVersion > lastVersion {
					structs.EventServerSendData(datagram, kind, reason, message, count)
				} else if event.Type == "ERROR" {
					var errObj *v1.Status = event.Object.(*v1.Status)
					logger.Log.Errorf("WATCHER (%d): '%s'", errObj.Code, errObj.Message)
					logger.Log.Error("WATCHER: Reset lastResourceVersion to empty.")
					lastResourceVersion = ""
					time.Sleep(RETRYTIMEOUT * time.Second) // Wait for 5 seconds before retrying
					break
				}
			} else {
				logger.Log.Errorf("WATCHER: Malformed event received Restarting watcher.")
				break
			}
		}

		// If the watcher channel is closed, wait for 5 seconds before retrying
		logger.Log.Errorf("Watcher channel closed. Waiting before retrying with '%s' ...", lastResourceVersion)
		watcher.Stop()
		time.Sleep(RETRYTIMEOUT * time.Second)
	}
}

func NewEventWatcher() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		logger.Log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching events with exponential backoff in case of failures
	retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, errors.IsServiceUnavailable, func() error {
		return watchEvents(provider)
	})

	// Wait forever
	select {}

	// // Function to start watching events
	// watchEvents := func() error {
	// 	// Define the listwatch for all events
	// 	listWatch := cache.NewListWatchFromClient(
	// 		provider.ClientSet.CoreV1().RESTClient(),
	// 		"events",
	// 		v1Core.NamespaceAll,
	// 		fields.Nothing(),
	// 	)

	// 	informer := cache.NewSharedInformer(listWatch, &corev1.Event{}, 0)
	// 	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 		AddFunc: func(obj interface{}) {
	// 			event := obj.(*corev1.Event)
	// 			processEvent(event)
	// 		},
	// 		UpdateFunc: func(oldObj, newObj interface{}) {
	// 			event := newObj.(*corev1.Event)
	// 			processEvent(event)
	// 		},
	// 		DeleteFunc: func(obj interface{}) {
	// 			event := obj.(*corev1.Event)
	// 			processEvent(event)
	// 		},
	// 	})
	// 	if err != nil {
	// 		logger.Log.Errorf("Error adding event handler: %s", err.Error())
	// 		return err
	// 	}

	// 	stopCh := make(chan struct{})
	// 	go informer.Run(stopCh)

	// 	// Handle reconnection in case of lost connection
	// 	go func() error {
	// 		for {
	// 			if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
	// 				err := fmt.Errorf("Timed out waiting for caches to sync")
	// 				runtime.HandleError(err)
	// 				return err
	// 			}
	// 			<-stopCh
	// 			time.Sleep(5 * time.Second) // Wait before reconnecting
	// 			stopCh = make(chan struct{})
	// 		}
	// 	}()
	// }

	// // Retry watching events with exponential backoff in case of failures
	// retry.OnError(wait.Backoff{
	// 	Steps:    5,
	// 	Duration: 1 * time.Second,
	// 	Factor:   2.0,
	// 	Jitter:   0.1,
	// }, errors.IsServiceUnavailable, watchEvents)

	// // Wait forever
	// select {}
}

func processEvent(event *corev1.Event) (string, error) {
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
	lw := cache.NewListWatchFromClient(
		provider.ClientSet.CoreV1().RESTClient(),
		"events",
		corev1.NamespaceAll,
		fields.Nothing(),
	)

	informer := cache.NewSharedInformer(lw, &corev1.Event{}, 0)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			event := obj.(*corev1.Event)
			processEvent(event)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			event := newObj.(*corev1.Event)
			processEvent(event)
		},
		DeleteFunc: func(obj interface{}) {
			event := obj.(*corev1.Event)
			processEvent(event)
		},
	})

	stopCh := make(chan struct{})
	go informer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
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
