package kubernetes

import (
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/websocket"
	"strings"

	"mogenius-k8s-manager/src/structs"
	"time"

	v1Core "k8s.io/api/core/v1"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var EventChannels = make(map[string]chan string)

func processEvent(eventClient websocket.WebsocketClient, event *v1Core.Event, eventType string) {
	if event != nil {
		eventDto := dtos.CreateEvent(string(event.Type), event)
		datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto)
		message := event.Message
		kind := event.InvolvedObject.Kind
		reason := event.Reason
		count := event.Count
		structs.EventServerSendData(eventClient, datagram, kind, reason, message, count, eventType)

		// deployment events
		if event.InvolvedObject.Kind == "Pod" {
			parts := strings.Split(event.InvolvedObject.Name, "-")

			if len(parts) >= 2 {
				parts = parts[:len(parts)-2]
			}
			controllerName := strings.Join(parts, "-")
			err := valkeyClient.AddToBucket(100, event, "pod-events", event.InvolvedObject.Namespace, controllerName)
			if err != nil {
				k8sLogger.Error("Error adding event to pod-events", "error", err.Error())
			}

			key := fmt.Sprintf("%s:%s", event.InvolvedObject.Namespace, controllerName)
			ch, exists := EventChannels[key]
			if exists {
				var events []*v1Core.Event
				events = append(events, event)
				updatedData, err := json.Marshal(events)
				if err == nil {
					ch <- string(updatedData)
				}
			}

		}
	} else {
		k8sLogger.Error("malformed event received")
	}
}
