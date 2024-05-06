package xterm

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/utils"
	"net/url"
	"time"
)

func readChannelPodEvent(ch chan string, conn *websocket.Conn, ctx context.Context) {
	for message := range ch {
		select {
		case <-ctx.Done():
			return
		default:
			if conn != nil {
				var events []v1.Event

				if err := json.Unmarshal([]byte(message), &events); err != nil {
					log.Errorf("Unable to unmarshal event: %s", err.Error())
					continue
				}
				for _, event := range events {
					formattedTime := event.FirstTimestamp.Time.Format("2006-01-02 15:04:05")
					err := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("[%s] %s\n\r", formattedTime, event.Message)))
					if err != nil {
						log.Errorf("WriteMessage: %s", err.Error())
					}
				}

			}
			continue
		}
	}
}

func XTermPodEventStreamConnection(wsConnectionRequest WsConnectionRequest, namespace string, controller string) {
	if wsConnectionRequest.WebsocketScheme == "" {
		log.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		log.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
	// websocket connection
	conn, err := generateWsConnection("scan-image-logs", namespace, controller, "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}

	key := fmt.Sprintf("%s-%s", namespace, controller)

	defer func() {
		log.Info("[XTermPodEventStreamConnection] Closing connection.")
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
	ch, _ = kubernetes.EventChannels[key]

	go readChannelPodEvent(ch, conn, ctx)

	// init
	go func(ch chan string) {
		data := db.GetEventByKey(key)
		if ch != nil {
			ch <- string(data)
		}
	}(ch)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, reader, err := conn.ReadMessage()
			if err != nil {
				log.Errorf("Unable to grab next reader: %s", err.Error())
				return
			}

			if string(reader) == "PEER_IS_READY" {
				continue
			}
		}
	}
}
