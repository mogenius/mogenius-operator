package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/db"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/utils"
	"strings"

	"mogenius-k8s-manager/src/structs"
	"time"

	v1Core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var EventChannels = make(map[string]chan string)

func ProcessEvent(event *v1Core.Event) {
	processEvent(event)
}

func processEvent(event *v1Core.Event) {
	if event != nil {
		eventDto := dtos.CreateEvent(string(event.Type), event)
		datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto)
		message := event.Message
		kind := event.InvolvedObject.Kind
		reason := event.Reason
		count := event.Count
		structs.EventServerSendData(datagram, kind, reason, message, count)

		// deployment events
		ignoreKind := []string{"CertificateRequest", "Certificate"}
		ignoreNamespaces := []string{"kube-system", "kube-public", "default", "mogenius"}
		if event.InvolvedObject.Kind == "Pod" &&
			!utils.ContainsString(ignoreNamespaces, event.InvolvedObject.Namespace) &&
			!utils.ContainsString(ignoreKind, event.InvolvedObject.Kind) {

			//personJSON, err := json.Marshal(event)
			//if err == nil {
			//	fmt.Println("event as JSON:", string(personJSON))
			//}
			parts := strings.Split(event.InvolvedObject.Name, "-")

			if len(parts) >= 2 {
				parts = parts[:len(parts)-2]
			}
			controllerName := strings.Join(parts, "-")
			err := db.AddPodEvent(event.InvolvedObject.Namespace, controllerName, event, 150)
			if err != nil {
				k8sLogger.Error("Error adding event to db", "error", err.Error())
			}

			key := fmt.Sprintf("%s-%s", event.InvolvedObject.Namespace, controllerName)
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

var allEventsForNamespaceDebounce = utils.NewDebounce("allEventsForNamespaceDebounce", 1000*time.Millisecond, 300*time.Millisecond)

func AllEventsForNamespace(namespaceName string) []v1Core.Event {
	result, _ := allEventsForNamespaceDebounce.CallFn(namespaceName, func() (interface{}, error) {
		return AllEventsForNamespace2(namespaceName), nil
	})
	return result.([]v1Core.Event)
}

func AllEventsForNamespace2(namespaceName string) []v1Core.Event {
	result := []v1Core.Event{}

	provider, err := NewKubeProvider()
	if err != nil {
		return result
	}
	eventList, err := provider.ClientSet.CoreV1().Events(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		k8sLogger.Error("AllEvents", "error", err)
		return result
	}

	moIgnoreNamespaces := config.Get("MO_IGNORE_NAMESPACES")
	var ignoreNamespaces []string
	err = json.Unmarshal([]byte(moIgnoreNamespaces), &ignoreNamespaces)
	assert.Assert(err == nil)
	for _, event := range eventList.Items {
		if !utils.Contains(ignoreNamespaces, event.ObjectMeta.Namespace) {
			event.Kind = "Event"
			event.APIVersion = "v1"
			result = append(result, event)
		}
	}
	return result
}
