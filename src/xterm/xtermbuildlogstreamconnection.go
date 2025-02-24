package xterm

import (
	"context"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/structs"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func readChannelBuildLog(ch chan string, conn *websocket.Conn, connWriteLock *sync.Mutex, ctx context.Context) {
	for message := range ch {
		select {
		case <-ctx.Done():
			return
		default:
			if conn != nil {
				connWriteLock.Lock()
				err := conn.WriteMessage(websocket.TextMessage, []byte(message))
				connWriteLock.Unlock()
				if err != nil {
					xtermLogger.Error("WriteMessage", "error", err)
				}
			}
			continue
		}
	}
}

func XTermBuildLogStreamConnection(wsConnectionRequest WsConnectionRequest, namespace string, controller string, container string, buildTask structs.BuildPrefixEnum, buildId int64) {
	if wsConnectionRequest.WebsocketScheme == "" {
		xtermLogger.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		xtermLogger.Error("WebsocketHost is empty")
		return
	}

	buildTimeout, err := strconv.Atoi(config.Get("MO_BUILDER_BUILD_TIMEOUT"))
	assert.Assert(err == nil, err)
	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(buildTimeout))
	// websocket connection
	readMessages, conn, connWriteLock, _, err := GenerateWsConnection("build-logs", namespace, controller, "", container, websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		xtermLogger.Error("Unable to connect to websocket", "error", err)
		return
	}

	// bolt db key
	key := structs.BuildJobInfoEntryKey(buildId, buildTask, namespace, controller, container)

	defer func() {
		// XtermLogger.Info("[XTermBuildLogStreamConnection] Closing connection.")
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

	go readChannelBuildLog(ch, conn, connWriteLock, ctx)

	// init
	go func(ch chan string) {
		// TODO BROKEN
		// data := kubernetes.GetDb().GetItemByKey(key)
		// build := structs.CreateBuildJobEntryFromData(data)
		build := structs.CreateBuildJobEntryFromData([]byte{})
		if ch != nil {
			ch <- build.Result
		}
	}(ch)

	// websocket to input
	websocketToCmdInput(*readMessages, ctx, nil, nil)
}
