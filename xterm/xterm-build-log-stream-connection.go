package xterm

import (
	"context"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func readChannelBuildLog(ch chan string, conn *websocket.Conn, ctx context.Context) {
	for message := range ch {
		select {
		case <-ctx.Done():
			return
		default:
			if conn != nil {
				err := conn.WriteMessage(websocket.TextMessage, []byte(message))
				if err != nil {
					log.Errorf("WriteMessage: %s", err.Error())
				}
			}
			continue
		}
	}
}

func XTermBuildLogStreamConnection(wsConnectionRequest WsConnectionRequest, namespace string, controller string, container string, buildTask structs.BuildPrefixEnum, buildId uint64) {
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
	readMessages, conn, err := generateWsConnection("build-logs", namespace, controller, "", container, websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}

	// bolt db key
	key := structs.BuildJobInfoEntryKey(buildId, buildTask, namespace, controller, container)

	defer func() {
		// log.Info("[XTermBuildLogStreamConnection] Closing connection.")
		cancel()

		ch := LogChannels[key]
		_, exists := LogChannels[key]
		if exists {
			if ch != nil {
				close(ch)
			}
			delete(LogChannels, key)
		}
	}()

	ch, exists := LogChannels[key]
	if exists {
		if ch != nil {
			close(ch)
		}
		delete(LogChannels, key)
	}
	LogChannels[key] = make(chan string)
	ch = LogChannels[key]

	go readChannelBuildLog(ch, conn, ctx)

	// init
	go func(ch chan string) {
		data := db.GetItemByKey(key)
		build := structs.CreateBuildJobEntryFromData(data)
		if ch != nil {
			ch <- build.Result
		}
	}(ch)

	// websocket to input
	websocketToCmdInput(*readMessages, ctx, nil, nil)
}
