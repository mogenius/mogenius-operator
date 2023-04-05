package kubernetes

import (
	"context"
	"log"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"reflect"
	"strconv"
	"time"

	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var lastResourceVersion = ""

func WatchEvents() {
	kubeProvider := NewKubeProvider()

	for {
		// Create a watcher for all Kubernetes events
		watcher, err := kubeProvider.ClientSet.CoreV1().Events("").Watch(context.TODO(), v1.ListOptions{Watch: true, ResourceVersion: lastResourceVersion})
		if err != nil {
			log.Printf("Error creating watcher: %v", err)
			time.Sleep(RETRYTIMEOUT * time.Second) // Wait for 5 seconds before retrying
			continue
		}

		// Start watching events
		for event := range watcher.ResultChan() {
			if event.Object != nil {
				eventDto := dtos.CreateEvent(string(event.Type), event.Object)
				datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto, nil)

				if reflect.TypeOf(event.Object).String() == "*v1.Event" {
					var eventObj *v1Core.Event = event.Object.(*v1Core.Event)
					currentVersion, _ := strconv.Atoi(eventObj.ObjectMeta.ResourceVersion)
					lastVersion, _ := strconv.Atoi(lastResourceVersion)
					if currentVersion > lastVersion {
						lastResourceVersion = eventObj.ObjectMeta.ResourceVersion
						message := eventObj.Message
						kind := eventObj.InvolvedObject.Kind
						reason := eventObj.Reason
						count := eventObj.Count
						structs.EventServerSendData(datagram, kind, reason, message, count)
					}
				} else if event.Type == "ERROR" {
					var errObj *v1.Status = event.Object.(*v1.Status)
					logger.Log.Errorf("WATCHER (%d): '%s'", errObj.Code, errObj.Message)
					logger.Log.Error("WATCHER: Reset lastResourceVersion to empty.")
					lastResourceVersion = ""
				}
			}
		}

		// If the watcher channel is closed, wait for 5 seconds before retrying
		logger.Log.Errorf("Watcher channel closed. Waiting before retrying with '%s' ...", lastResourceVersion)
		time.Sleep(RETRYTIMEOUT * time.Second)
	}
}
