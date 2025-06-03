package core

import (
	"context"
	"encoding/json"
	"log/slog"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/valkeyclient"
	"mogenius-k8s-manager/src/xterm"
	"net/url"
	"time"

	"github.com/go-redis/redis/v8"
)

type XtermService interface {
	LiveStreamConnection(wsConnectionRequest xterm.WsConnectionRequest, datagram structs.Datagram, httpApi HttpService, store valkeyclient.ValkeyClient)
}

type xtermService struct {
	logger *slog.Logger
}

func NewXtermService(logger *slog.Logger) XtermService {
	self := &xtermService{}
	self.logger = logger

	return self
}

func (self *xtermService) LiveStreamConnection(wsConnectionRequest xterm.WsConnectionRequest, datagram structs.Datagram, httpApi HttpService, store valkeyclient.ValkeyClient) {
	logger := self.logger.With("scope", "LiveStreamConnection")

	var pubsub *redis.PubSub
	switch datagram.Pattern {
	case "live-stream/nodes-traffic":
		pubsub = store.SubscribeToKey(DB_STATS_LIVE_BUCKET_NAME, "traffic", "homeserver")
	case "live-stream/nodes-memory":
		pubsub = store.SubscribeToKey(DB_STATS_LIVE_BUCKET_NAME, "memory", "homeserver")
	case "live-stream/nodes-cpu":
		pubsub = store.SubscribeToKey(DB_STATS_LIVE_BUCKET_NAME, "cpu", "homeserver")
	default:
		logger.Error("Unsupported pattern for LiveStreamConnection", "pattern", datagram.Pattern)
		return
	}
	defer pubsub.Close()

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
	defer httpApi.Broadcaster().RemoveListener(listener)

	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				logger.Error("failed to read from connection", "error", err)
				break
			}
		}
	}()

	for msg := range pubsub.Channel() {
		var entry interface{}
		err := json.Unmarshal([]byte(msg.Payload), &entry)
		if err != nil {
			logger.Error("Unmarshal", "error", err)
			continue
		}
		// XXX remove
		logger.Info("Received message from pubsub", "pattern", datagram.Pattern)
		httpApi.Broadcaster().BroadcastResponse(entry, datagram.Pattern)
	}
}
