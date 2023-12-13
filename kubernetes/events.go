package kubernetes

import (
	"context"
	"log"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

func WatchEvents() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		return
	}

	var lastResourceVersion = ""
	for {
		// Create a watcher for all Kubernetes events
		watcher, err := provider.ClientSet.CoreV1().Events("").Watch(context.TODO(), v1.ListOptions{Watch: true, ResourceVersion: lastResourceVersion})

		if err != nil || watcher == nil {
			if apierrors.IsGone(err) {
				lastResourceVersion = ""
			}
			log.Printf("Error creating watcher: %v", err)
			time.Sleep(RETRYTIMEOUT * time.Second) // Wait for 5 seconds before retrying
			continue
		} else {
			logger.Log.Notice("Watcher connected successfully. Start watching events...")
		}

		// Start watching events
		for event := range watcher.ResultChan() {
			if event.Object != nil {
				eventDto := dtos.CreateEvent(string(event.Type), event.Object)
				datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto)

				eventObj, isEvent := event.Object.(*v1Core.Event)
				if isEvent {
					// currentVersion, errCurrentVer := strconv.Atoi(eventObj.ObjectMeta.ResourceVersion)
					// lastVersion, _ := strconv.Atoi(lastResourceVersion)
					// if errCurrentVer == nil {
					lastResourceVersion = eventObj.ObjectMeta.ResourceVersion
					message := eventObj.Message
					kind := eventObj.InvolvedObject.Kind
					reason := eventObj.Reason
					count := eventObj.Count
					// if currentVersion > lastVersion {
					structs.EventServerSendData(datagram, kind, reason, message, count)
					// } else {
					// 	if utils.CONFIG.Misc.Debug {
					// 		logger.Log.Errorf("Versions are out of order: %d / %d", lastVersion, currentVersion)
					// 		logger.Log.Errorf("%s/%s -> %s (Count: %d)\n", kind, reason, message, count)
					// 	}
					// }
					// }
				} else if event.Type == "ERROR" {
					var errObj *v1.Status = event.Object.(*v1.Status)
					logger.Log.Errorf("WATCHER (%d): '%s'", errObj.Code, errObj.Message)
					logger.Log.Error("WATCHER: Reset lastResourceVersion to empty.")
					lastResourceVersion = ""
					time.Sleep(RETRYTIMEOUT * time.Second) // Wait for 5 seconds before retrying
					break
				}
			}
		}

		// If the watcher channel is closed, wait for 5 seconds before retrying
		logger.Log.Errorf("Watcher channel closed. Waiting before retrying with '%s' ...", lastResourceVersion)
		watcher.Stop()
		time.Sleep(RETRYTIMEOUT * time.Second)
	}
}
