package core

import (
	"context"
	"encoding/json"
	"log/slog"
	"mogenius-operator/src/cpumonitor"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/rammonitor"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/xterm"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"
)

type XtermService interface {
	LiveStreamConnection(wsConnectionRequest xterm.WsConnectionRequest, datagram structs.Datagram, httpApi HttpService, store valkeyclient.ValkeyClient, podNames []string)
}

type xtermService struct {
	logger *slog.Logger
}

func NewXtermService(logger *slog.Logger) XtermService {
	self := &xtermService{}
	self.logger = logger

	return self
}

func (self *xtermService) LiveStreamConnection(conReq xterm.WsConnectionRequest, datagram structs.Datagram, httpApi HttpService, store valkeyclient.ValkeyClient, podNames []string) {
	logger := self.logger.With("scope", "LiveStreamConnection")

	var valkeyKey string
	switch datagram.Pattern {
	case "live-stream/nodes-traffic", "live-stream/pod-traffic", "live-stream/workspace-traffic":
		valkeyKey = strings.Join([]string{DB_STATS_LIVE_BUCKET_NAME, "traffic", conReq.NodeName}, ":")
	case "live-stream/nodes-memory":
		valkeyKey = strings.Join([]string{DB_STATS_LIVE_BUCKET_NAME, "memory", conReq.NodeName}, ":")
	case "live-stream/nodes-cpu":
		valkeyKey = strings.Join([]string{DB_STATS_LIVE_BUCKET_NAME, "cpu", conReq.NodeName}, ":")
	case "live-stream/pod-memory", "live-stream/workspace-memory":
		valkeyKey = strings.Join([]string{DB_STATS_LIVE_BUCKET_NAME, "memory", "proc", conReq.NodeName}, ":")
	case "live-stream/pod-cpu", "live-stream/workspace-cpu":
		valkeyKey = strings.Join([]string{DB_STATS_LIVE_BUCKET_NAME, "cpu", "proc", conReq.NodeName}, ":")
	default:
		logger.Error("Unsupported pattern for LiveStreamConnection", "pattern", datagram.Pattern)
		return
	}

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

	listener := NewMessageCallback(datagram, func(message any) {
		if conn != nil {
			connWriteLock.Lock()
			err := conn.WriteJSON(message)
			connWriteLock.Unlock()
			if err != nil {
				logger.Error("WriteMessage Broadcast", "error", err)
				cancel() // Close the context to stop the connection
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

	client := store.GetValkeyClient()
	err = client.Receive(ctx, client.B().Subscribe().Channel(valkeyKey).Build(), func(msg valkey.PubSubMessage) {
		var entry any
		// remove unnecessary fields for pods to save bandwidth
		switch datagram.Pattern {
		case "live-stream/pod-memory", "live-stream/workspace-memory":
			data := []rammonitor.PodRamStats{}
			err := json.Unmarshal([]byte(msg.Message), &data)
			if err != nil {
				logger.Error("Unmarshal", "error", err)
				return
			}
			// remove entries which are not the requested pod
			for i := 0; i < len(data); i++ {
				if !slices.Contains(podNames, data[i].Name) {
					data = append(data[:i], data[i+1:]...)
					i--
				}
			}
			entry = data
		case "live-stream/pod-cpu", "live-stream/workspace-cpu":
			data := []cpumonitor.PodCpuStats{}
			err := json.Unmarshal([]byte(msg.Message), &data)
			if err != nil {
				logger.Error("Unmarshal", "error", err)
				return
			}
			for i := 0; i < len(data); i++ {
				if !slices.Contains(podNames, data[i].Name) {
					data = append(data[:i], data[i+1:]...)
					i--
				}
			}
			entry = data
		case "live-stream/pod-traffic", "live-stream/workspace-traffic":
			data := []networkmonitor.PodNetworkStats{}
			err := json.Unmarshal([]byte(msg.Message), &data)
			if err != nil {
				logger.Error("Unmarshal", "error", err)
				return
			}
			for i := 0; i < len(data); i++ {
				if !slices.Contains(podNames, data[i].Pod) {
					data = append(data[:i], data[i+1:]...)
					i--
				}
			}
			entry = data
		default:
			// For other patterns, we can directly use the entry as it is already in the correct format
			err := json.Unmarshal([]byte(msg.Message), &entry)
			if err != nil {
				logger.Error("Unmarshal", "error", err)
				return
			}
		}

		httpApi.Broadcaster().BroadcastResponse(entry, datagram.Pattern)
	})
	if err != nil {
		self.logger.Error("failed to register receive handler", "error", err)
	}
}
