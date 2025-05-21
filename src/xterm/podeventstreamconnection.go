package xterm

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var eventChannels = make(map[string]chan string)

func readChannelPodEvent(ch chan string, conn *websocket.Conn, connWriteLock *sync.Mutex, ctx context.Context) {
	for message := range ch {
		select {
		case <-ctx.Done():
			return
		default:
			if conn != nil {
				var event v1.Event

				if err := json.Unmarshal([]byte(message), &event); err != nil {
					xtermLogger.Error("Unable to unmarshal event", "error", err)
					continue
				}
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

		ch := eventChannels[key]
		_, exists := eventChannels[key]
		if exists {
			if ch != nil {
				close(ch)
			}
			delete(eventChannels, key)
		}
	}()

	ch, exists := eventChannels[key]
	if exists {
		if ch != nil {
			close(ch)
		}
		delete(eventChannels, key)
	}
	eventChannels[key] = make(chan string)
	ch = eventChannels[key]

	go readChannelPodEvent(ch, conn, connWriteLock, ctx)

	// init
	go func(ch chan string) {
		data, err := store.List(50, kubernetes.VALKEY_RESOURCE_PREFIX, "v1", "Event", namespace)
		if err != nil {
			xtermLogger.Error("Error getting events from pod-events", "error", err.Error())
			return
		}

		if len(data) == 0 {
			emptyEvent := &v1.Event{Message: "No recent events found. Restart the Pod to generate visible events or enjoy the silence.", FirstTimestamp: metav1.Time{Time: time.Now()}}
			emptyEventData, err := json.Marshal(emptyEvent)
			if err != nil {
				xtermLogger.Error("Error getting events from pod-events", "error", err.Error())
				return
			}
			data = append(data, string(emptyEventData))
		}

		for _, v := range data {
			ch <- v
		}
	}(ch)

	// websocket to input
	websocketToCmdInput(*readMessages, ctx, nil, nil)
}
