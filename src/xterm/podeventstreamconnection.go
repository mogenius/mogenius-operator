package xterm

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	json "github.com/json-iterator/go"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func writeEvent(conn *websocket.Conn, connWriteLock *sync.Mutex, event v1.Event) {
	if conn != nil {
		formattedTime := event.ObjectMeta.CreationTimestamp.Time.Format("2006-01-02 15:04:05")
		if !strings.HasSuffix(event.Message, "\n") && !strings.HasSuffix(event.Message, "\n\r") {
			event.Message = event.Message + "\n\r"
		}
		connWriteLock.Lock()
		var err error
		if strings.HasPrefix(event.Message, "No recent events found.") {
			err = conn.WriteMessage(websocket.TextMessage, []byte(string(event.Message)))
		} else {
			err = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("[%s] %s%s", formattedTime, utils.FillWith(event.Reason, 28, " "), event.Message)))
		}
		connWriteLock.Unlock()
		if err != nil {
			xtermLogger.Error("WriteMessage", "error", err)
		}
	}
}

func PodEventStreamConnection(wsConnectionRequest WsConnectionRequest, namespace string, controller string) {
	if wsConnectionRequest.WebsocketScheme == "" {
		xtermLogger.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		xtermLogger.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(600))
	readMessages, conn, connWriteLock, _, err := GenerateWsConnection("pod-events", namespace, controller, "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		xtermLogger.Error("Unable to connect to websocket", "error", err)
		return
	}

	data, err := store.List(50, kubernetes.VALKEY_RESOURCE_PREFIX, "v1", "Event", namespace, controller+"*")
	if err != nil {
		xtermLogger.Error("Error getting events from pod-events", "error", err.Error())
		return
	}

	// sort the events by timestamp
	if len(data) > 0 {
		sort.Slice(data, func(i, j int) bool {
			event := &v1.Event{}
			if err := json.Unmarshal([]byte(data[i]), event); err != nil {
				xtermLogger.Error("Unable to unmarshal event", "error", err)
				return false
			}
			event2 := &v1.Event{}
			if err := json.Unmarshal([]byte(data[j]), event2); err != nil {
				xtermLogger.Error("Unable to unmarshal event", "error", err)
				return false
			}
			return event.ObjectMeta.CreationTimestamp.Time.Before(event2.ObjectMeta.CreationTimestamp.Time)
		})
	}

	if len(data) == 0 {
		emptyEvent := v1.Event{Message: "No recent events found. Restart the Pod to generate visible events or enjoy the silence.", FirstTimestamp: metav1.Time{Time: time.Now()}}
		writeEvent(conn, connWriteLock, emptyEvent)
	}
	for _, item := range data {
		event := v1.Event{}
		if err := json.Unmarshal([]byte(item), &event); err != nil {
			xtermLogger.Error("Unable to unmarshal event", "error", err)
			continue
		}
		writeEvent(conn, connWriteLock, event)
	}

	// websocket to input
	websocketToCmdInput(*readMessages, ctx, nil, nil)
}
