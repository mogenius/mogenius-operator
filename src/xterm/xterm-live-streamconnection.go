package xterm

import (
	"context"
	"mogenius-k8s-manager/src/core"
	"net/url"
	"time"
)

func LiveStreamConnection(wsConnectionRequest WsConnectionRequest, dataPattern string, httpApi core.HttpService) {
	logger := xtermLogger.With("scope", "LiveStreamConnection")

	if wsConnectionRequest.WebsocketScheme == "" {
		logger.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		logger.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3600)
	// websocket connection
	_, conn, connWriteLock, _, err := generateWsConnection(dataPattern, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		logger.Error("Unable to connect to websocket", "error", err)
		return
	}

	listener := core.NewMessageCallback(dataPattern, func(message interface{}) {
		if conn != nil {
			connWriteLock.Lock()
			err := conn.WriteJSON(message)
			connWriteLock.Unlock()
			if err != nil {
				logger.Error("WriteMessage Broadcast", "error", err)
			}
		}
	})

	httpApi.Broadcaster().AddListener(listener)
	defer func() {
		httpApi.Broadcaster().RemoveListener(listener)
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			logger.Error("failed to read from connection", "error", err)
			break
		}
	}
}
