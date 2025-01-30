package xterm

import (
	"context"
	"mogenius-k8s-manager/src/httpservice"
	"net/url"
	"time"
)

func LiveStreamConnection(wsConnectionRequest WsConnectionRequest, dataPattern string, httpApi *httpservice.HttpService) {
	if wsConnectionRequest.WebsocketScheme == "" {
		xtermLogger.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		xtermLogger.Error("WebsocketHost is empty")
		return
	}
	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3600)
	// websocket connection
	_, conn, connWriteLock, err := generateWsConnection("live-stream-"+dataPattern, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		xtermLogger.Error("Unable to connect to websocket", "error", err)
		return
	}

	listener := httpservice.MessageCallback{
		MsgFunc: func(message interface{}) {
			if conn != nil {
				connWriteLock.Lock()
				err := conn.WriteJSON(message)
				connWriteLock.Unlock()
				if err != nil {
					xtermLogger.Error("WriteMessage", "error", err)
				}
			}
		},
		MsgType: dataPattern,
	}
	httpApi.Broadcaster.AddListener(listener)

	defer func() {
		httpApi.Broadcaster.RemoveListener(listener)
	}()

	select {
	case <-ctx.Done():
		return
	default:
	}

	xtermLogger.Error("LiveStreamConnection", "error", "context done")
}
