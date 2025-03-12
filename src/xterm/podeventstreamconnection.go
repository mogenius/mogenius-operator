package xterm

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/kubernetes"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func readChannelPodEvent(ch chan string, conn *websocket.Conn, connWriteLock *sync.Mutex, ctx context.Context) {
	for message := range ch {
		select {
		case <-ctx.Done():
			return
		default:
			if conn != nil {
				var events []v1.Event

				if err := json.Unmarshal([]byte(message), &events); err != nil {
					xtermLogger.Error("Unable to unmarshal event", "error", err)
					continue
				}
				for _, event := range events {
					formattedTime := event.FirstTimestamp.Time.Format("2006-01-02 15:04:05")
					if !strings.HasSuffix(event.Message, "\n") && !strings.HasSuffix(event.Message, "\n\r") {
						event.Message = event.Message + "\n\r"
					}
					connWriteLock.Lock()
					err := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("[%s] %s", formattedTime, event.Message)))
					connWriteLock.Unlock()
					if err != nil {
						xtermLogger.Error("WriteMessage", "error", err)
					}
				}

			}
			continue
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

	key := fmt.Sprintf("%s:%s", namespace, controller)

	defer func() {
		cancel()

		ch := kubernetes.EventChannels[key]
		_, exists := kubernetes.EventChannels[key]
		if exists {
			if ch != nil {
				close(ch)
			}
			delete(kubernetes.EventChannels, key)
		}
	}()

	ch, exists := kubernetes.EventChannels[key]
	if exists {
		if ch != nil {
			close(ch)
		}
		delete(kubernetes.EventChannels, key)
	}
	kubernetes.EventChannels[key] = make(chan string)
	ch = kubernetes.EventChannels[key]

	go readChannelPodEvent(ch, conn, connWriteLock, ctx)

	// init
	go func(ch chan string) {
		data, err := store.LastNEntryFromBucketWithType(50, kubernetes.DB_STATS_POD_EVENTS_NAME, key)
		if err != nil {
			xtermLogger.Error("Error getting events from pod-events", "error", err.Error())
			return
		}
		var events []*v1.Event
		for _, v := range data {
			var event v1.Event
			err := json.Unmarshal([]byte(v), &event)
			if err != nil {
				xtermLogger.Error("Error getting events from pod-events", "error", err.Error())
				continue
			}
			events = append(events, &event)
		}
		if len(events) == 0 {
			events = append(events, &v1.Event{Message: "No recent events found. Restart the Pod to generate visible events or enjoy the silence.", FirstTimestamp: metav1.Time{Time: time.Now()}})
		}
		updatedData, err := json.Marshal(events)
		if err != nil {
			xtermLogger.Error("Error getting events from pod-events", "error", err.Error())
			return
		}
		if ch != nil {
			ch <- string(updatedData)
		}
	}(ch)

	// websocket to input
	websocketToCmdInput(*readMessages, ctx, nil, nil)
}
