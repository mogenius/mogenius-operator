package kubernetes

import (
	"context"
	"log"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"reflect"
	"time"

	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var lastResourceVersion = ""

func WatchEvents() {
	ctx := context.Background()
	var kubeProvider *KubeProvider
	var err error

	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		logger.Log.Errorf("watchEvents ERROR: %s", err.Error())
		return
	}

	for {
		// Create a watcher for all Kubernetes events
		watcher, err := kubeProvider.ClientSet.CoreV1().Events("").Watch(ctx, v1.ListOptions{Watch: true, ResourceVersion: lastResourceVersion})
		if err != nil {
			log.Printf("Error creating watcher: %v", err)
			time.Sleep(5 * time.Second) // Wait for 5 seconds before retrying
			continue
		}

		// Start watching events
		for event := range watcher.ResultChan() {
			if event.Object != nil {
				eventDto := dtos.CreateEvent(string(event.Type), event.Object)
				datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto, nil)

				if reflect.TypeOf(event.Object).String() == "*v1.Event" {
					var eventObj *v1Core.Event = event.Object.(*v1Core.Event)
					lastResourceVersion = eventObj.ObjectMeta.ResourceVersion
					eventName := &eventObj.Message
					if eventObj.Count > 1 {
						// this disables the log-display (prevent log spamming)
						eventName = nil
					}
					structs.EventServerSendData(datagram, eventName)
				}
			}
		}

		// If the watcher channel is closed, wait for 5 seconds before retrying
		log.Printf("Watcher channel closed. Waiting before retrying...")
		time.Sleep(RETRYTIMEOUT * time.Second)
	}
}
