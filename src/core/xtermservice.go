package core

import (
	"context"
	"log/slog"
	"mogenius-k8s-manager/src/cpumonitor"
	"mogenius-k8s-manager/src/networkmonitor"
	"mogenius-k8s-manager/src/rammonitor"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/valkeyclient"
	"mogenius-k8s-manager/src/xterm"
	"net/url"
	"time"

	json "github.com/json-iterator/go"

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

func (self *xtermService) LiveStreamConnection(conReq xterm.WsConnectionRequest, datagram structs.Datagram, httpApi HttpService, store valkeyclient.ValkeyClient) {
	logger := self.logger.With("scope", "LiveStreamConnection")

	var pubsub *redis.PubSub
	switch datagram.Pattern {
	case "live-stream/nodes-traffic":
		pubsub = store.SubscribeToKey(DB_STATS_LIVE_BUCKET_NAME, "traffic", conReq.NodeName)
	case "live-stream/nodes-memory":
		pubsub = store.SubscribeToKey(DB_STATS_LIVE_BUCKET_NAME, "memory", conReq.NodeName)
	case "live-stream/nodes-cpu":
		pubsub = store.SubscribeToKey(DB_STATS_LIVE_BUCKET_NAME, "cpu", conReq.NodeName)
	case "live-stream/pod-memory":
		pubsub = store.SubscribeToKey(DB_STATS_LIVE_BUCKET_NAME, "memory", "proc", conReq.NodeName)
	case "live-stream/pod-cpu":
		pubsub = store.SubscribeToKey(DB_STATS_LIVE_BUCKET_NAME, "cpu", "proc", conReq.NodeName)
	case "live-stream/pod-traffic":
		pubsub = store.SubscribeToKey(DB_STATS_LIVE_BUCKET_NAME, "traffic", conReq.NodeName)
	default:
		logger.Error("Unsupported pattern for LiveStreamConnection", "pattern", datagram.Pattern)
		return
	}
	defer pubsub.Close()

	if conReq.WebsocketScheme == "" {
		logger.Error("WebsocketScheme is empty")
		return
	}

	if conReq.WebsocketHost == "" {
		logger.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: conReq.WebsocketScheme, Host: conReq.WebsocketHost, Path: "/xterm-stream"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3600)
	defer cancel()
	// websocket connection
	_, conn, connWriteLock, _, err := xterm.GenerateWsConnection(datagram.Pattern, "", "", "", "", websocketUrl, conReq, ctx, cancel)
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
				cancel()       // Close the context to stop the connection
				pubsub.Close() // Close the pubsub channel
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
		// remove unnecessary fields for pods to save bandwidth
		switch datagram.Pattern {
		case "live-stream/pod-memory":
			data := []rammonitor.PodRamStats{}
			err := json.Unmarshal([]byte(msg.Payload), &data)
			if err != nil {
				logger.Error("Unmarshal", "error", err)
				continue
			}
			// remove entries which are not the requested pod
			for i := 0; i < len(data); i++ {
				if data[i].Name != conReq.PodName {
					data = append(data[:i], data[i+1:]...)
					i--
				}
			}
			entry = data
		case "live-stream/pod-cpu":
			data := []cpumonitor.PodCpuStats{}
			err := json.Unmarshal([]byte(msg.Payload), &data)
			if err != nil {
				logger.Error("Unmarshal", "error", err)
				continue
			}
			for i := 0; i < len(data); i++ {
				if data[i].Name != conReq.PodName {
					data = append(data[:i], data[i+1:]...)
					i--
				}
			}
			entry = data
		case "live-stream/pod-traffic":
			data := []networkmonitor.PodNetworkStats{}
			err := json.Unmarshal([]byte(msg.Payload), &data)
			if err != nil {
				logger.Error("Unmarshal", "error", err)
				continue
			}
			for i := 0; i < len(data); i++ {
				if data[i].Pod != conReq.PodName {
					data = append(data[:i], data[i+1:]...)
					i--
				}
			}
			entry = data
		default:
			// For other patterns, we can directly use the entry as it is already in the correct format
			err := json.Unmarshal([]byte(msg.Payload), &entry)
			if err != nil {
				logger.Error("Unmarshal", "error", err)
				continue
			}
		}

		httpApi.Broadcaster().BroadcastResponse(entry, datagram.Pattern)
	}
}
