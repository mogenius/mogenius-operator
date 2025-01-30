package xterm

import (
	"context"
	"mogenius-k8s-manager/src/httpservice"
	"mogenius-k8s-manager/src/structs"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
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
		MsgFunc: func(message string) {
			if conn != nil {
				connWriteLock.Lock()
				err := conn.WriteMessage(websocket.TextMessage, []byte(message))
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
		switch dataPattern {
		case structs.PAT_LIVE_STREAM_NODES_CPU_REQUEST:
			httpApi.RequestCpuUtilizationStreamStop()
		case structs.PAT_LIVE_STREAM_NODES_MEMORY_REQUEST:
			httpApi.RequestMemUtilizationStreamStop()
		case structs.PAT_LIVE_STREAM_NODES_TRAFFIC_REQUEST:
			httpApi.RequestTrafficUtilizationStreamStop()
		default:
			xtermLogger.Error("unknown pattern detected", "error", dataPattern)
		}
	}()

	select {
	case <-ctx.Done():
		return
	default:
	}

	xtermLogger.Error("LiveStreamConnection", "error", "context done")
}
