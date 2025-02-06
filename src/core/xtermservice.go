package core

import (
	"context"
	"log/slog"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/xterm"
	"net/url"
	"time"
)

type XtermService interface {
	LiveStreamConnection(wsConnectionRequest xterm.WsConnectionRequest, datagram structs.Datagram, httpApi HttpService)
}

type xtermService struct {
	logger *slog.Logger
}

func NewXtermService(logger *slog.Logger) XtermService {
	self := &xtermService{}
	self.logger = logger

	return self
}

func (self *xtermService) LiveStreamConnection(wsConnectionRequest xterm.WsConnectionRequest, datagram structs.Datagram, httpApi HttpService) {
	logger := self.logger.With("scope", "LiveStreamConnection")

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
	_, conn, connWriteLock, _, err := xterm.GenerateWsConnection(datagram.Pattern, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		logger.Error("Unable to connect to websocket", "error", err)
		return
	}

	listener := NewMessageCallback(datagram, func(message interface{}) {
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
