package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/src/websocket"
	"time"

	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func HandleNetworkPolicyChange(eventClient websocket.WebsocketClient, netPol *v1.NetworkPolicy, reason string) {
	annotations := createAnnotations("mogenius.io/created", time.Now().String())
	// create a new event
	event := &v1Core.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s.%s.%s", netPol.Name, netPol.Namespace, time.Now().String()),
			Namespace:   netPol.Namespace,
			Annotations: annotations,
		},
		InvolvedObject: v1Core.ObjectReference{
			Kind:            "NetworkPolicy",
			Namespace:       netPol.Namespace,
			Name:            netPol.Name,
			UID:             netPol.UID,
			ResourceVersion: netPol.ResourceVersion,
		},
		Reason:    reason,
		Message:   fmt.Sprintf("NetPol %s is being %s", netPol.Name, reason),
		Type:      v1Core.EventTypeNormal,
		EventTime: metav1.MicroTime{Time: time.Now()},
	}

	k8sLogger.Debug("Sending custom network policy event to dispatcher", "event", event)

	processEvent(eventClient, event)
}

func createAnnotations(items ...string) map[string]string {
	if len(items)%2 != 0 {
		return nil
	}

	annotations := make(map[string]string)

	for i := 0; i < len(items); i += 2 {
		key := items[i]
		value := items[i+1]
		annotations[key] = value
	}

	return annotations
}
