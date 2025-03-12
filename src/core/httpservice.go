package core

import (
	"encoding/json"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type HttpService interface {
	Run(addr string)
	Link(socketapi SocketApi)
	Broadcaster() *Broadcaster
}

type httpService struct {
	logger      *slog.Logger
	config      cfg.ConfigModule
	dbstats     kubernetes.ValkeyStatsDb
	api         Api
	broadcaster *Broadcaster

	socketapi SocketApi
}

type MessageCallback struct {
	Id      string
	MsgType string
	MsgFunc func(message interface{})
}

func NewMessageCallback(datagram structs.Datagram, callback func(message interface{})) MessageCallback {
	self := MessageCallback{}
	self.Id = datagram.Id
	self.MsgType = datagram.Pattern
	self.MsgFunc = callback

	return self
}

func (self *MessageCallback) Equals(other *MessageCallback) bool {
	return self.Id == other.Id
}

type Broadcaster struct {
	mu        sync.Mutex
	Listeners []MessageCallback
}

// Add a listener (callback) to the broadcaster
func (self *Broadcaster) AddListener(callback MessageCallback) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.Listeners = append(self.Listeners, callback)
}

// Remove a listener (callback) from the broadcaster
func (self *Broadcaster) RemoveListener(callback MessageCallback) {
	self.mu.Lock()
	defer self.mu.Unlock()

	for i, listener := range self.Listeners {
		if listener.Equals(&callback) {
			self.Listeners = append(self.Listeners[:i], self.Listeners[i+1:]...)
			continue
		}
	}
}

func (self *Broadcaster) BroadcastResponse(message interface{}, messageType string) {
	self.mu.Lock()
	defer self.mu.Unlock()

	for _, listener := range self.Listeners {
		if listener.MsgType == messageType {
			listener.MsgFunc(message)
		}
	}
}

func NewHttpApi(
	logManagerModule logging.SlogManager,
	configModule cfg.ConfigModule,
	dbstats kubernetes.ValkeyStatsDb,
	apiModule Api,
) HttpService {
	assert.Assert(logManagerModule != nil)
	assert.Assert(configModule != nil)
	assert.Assert(dbstats != nil)

	return &httpService{
		logger:  logManagerModule.CreateLogger("http"),
		config:  configModule,
		dbstats: dbstats,
		api:     apiModule,
		broadcaster: &Broadcaster{
			Listeners: []MessageCallback{},
			mu:        sync.Mutex{},
		},
	}
}

func (self *httpService) Run(addr string) {
	assert.Assert(self.logger != nil)
	assert.Assert(self.config != nil)
	assert.Assert(self.socketapi != nil)

	self.logger.Debug("initializing http.ServeMux", "addr", addr)
	mux := http.NewServeMux()

	// Deprecated: Typo in `GET /healtz`. Use `GET /healthz` instead.
	mux.Handle("GET /healtz", self.withRequestLogging(http.HandlerFunc(self.getHealthz)))
	mux.Handle("GET /healthz", self.withRequestLogging(http.HandlerFunc(self.getHealthz)))

	mux.Handle("GET /ws", self.withRequestLogging(http.HandlerFunc(self.handleWs)))

	if utils.IsDevBuild() {
		self.addApiRoutes(mux)
	}

	self.logger.Info("starting API server", "addr", addr)
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		self.logger.Error("failed to start api server", "error", err)
	}
}

func (self *httpService) Link(socketapi SocketApi) {
	assert.Assert(socketapi != nil)

	self.socketapi = socketapi
}

func (self *httpService) Broadcaster() *Broadcaster {
	return self.broadcaster
}

func (self *httpService) getHealthz(w http.ResponseWriter, _ *http.Request) {
	healthStatus := map[string]string{
		"version": version.Ver,
		"branch":  version.Branch,
		"hash":    version.GitCommitHash,
		"buildAt": version.BuildTimestamp,
		"stage":   self.config.Get("MO_STAGE"),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(healthStatus)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *httpService) withRequestLogging(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		self.logger.Debug("api request",
			"request.Header", r.Header,
			"request.ContentLength", r.ContentLength,
			"request.RequestURI", r.RequestURI,
			"request.Url", r.URL,
			"request.RemoteAddr", r.RemoteAddr,
			"request.Proto", r.Proto,
		)
		handler.ServeHTTP(w, r)
	})
}

// WEBSOCKET
// only for internal connections from pod-stat-collector or traffic-collector
// this enables us to have a bi-directional communication channel
// Example:
// User whats to get CPU utilization stream for all Nodes
// 1. User sends a pattern "cpu-utilization"
// 2. K8sManager broadcasts the message to all connected clients (DaemonSet in this case, pod on each node)
// 3. All connected Pods which implement the pattern respond with the datastream
// 4. K8sManager receives the datastream and relay it to the requesting client via websocket

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (self *httpService) handleWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		self.logger.Error("Upgrading connection to WebSocket", "error", err)
		return
	}
	defer func() {
		if ws != nil {
			ws.Close()
		}
	}()

	self.logger.Info("WebSocket connection established")

	for {
		datagram := &structs.Datagram{}
		err := ws.ReadJSON(datagram)
		if err != nil {
			self.logger.Error("Reading message from websocket", "error", err)
			break
		}
		self.handleIncomingDatagram(datagram)
	}
}

func (self *httpService) handleIncomingDatagram(datagram *structs.Datagram) {
	switch datagram.Pattern {
	case "traffic-utilization":
		self.broadcaster.BroadcastResponse(datagram.Payload, "live-stream/nodes-traffic")

	case "cpu-utilization":
		self.broadcaster.BroadcastResponse(datagram.Payload, "live-stream/nodes-cpu")

	case "mem-utilization":
		self.broadcaster.BroadcastResponse(datagram.Payload, "live-stream/nodes-memory")

	// SAVE TO DB
	case "traffic-status":
		stat := &structs.InterfaceStats{}
		dataBytes, err := json.Marshal(datagram.Payload)
		if err != nil {
			self.logger.Error("failed to marshal interface stats", "error", err)
			return
		}
		err = json.Unmarshal(dataBytes, stat)
		if err != nil {
			self.logger.Error("failed to unmarshal interface stats", "error", err)
			return
		}
		self.dbstats.AddInterfaceStatsToDb(*stat)

	case "cni-status":
		cniData := &[]structs.CniData{}
		dataBytes, err := json.Marshal(datagram.Payload)
		if err != nil {
			self.logger.Error("failed to marshal cniData", "error", err)
			return
		}
		err = json.Unmarshal(dataBytes, cniData)
		if err != nil {
			self.logger.Error("failed to unmarshal cniData", "error", err)
			return
		}
		self.dbstats.ReplaceCniData(*cniData)

	case "podstats-status":
		stats := &[]structs.PodStats{}
		dataBytes, err := json.Marshal(datagram.Payload)
		if err != nil {
			self.logger.Error("failed to marshal pod stats", "error", err)
			return
		}
		err = json.Unmarshal(dataBytes, stats)
		if err != nil {
			self.logger.Error("failed to unmarshal pod stats", "error", err)
			return
		}
		for _, v := range *stats {
			self.dbstats.AddPodStatsToDb(v)
		}

	case "nodestats-status":
		stats := &[]structs.NodeStats{}
		dataBytes, err := json.Marshal(datagram.Payload)
		if err != nil {
			self.logger.Error("failed to marshal node stats", "error", err)
			return
		}
		err = json.Unmarshal(dataBytes, stats)
		if err != nil {
			self.logger.Error("failed to unmarshal node stats", "error", err)
			return
		}
		for _, v := range *stats {
			self.dbstats.AddNodeStatsToDb(v)
		}

	default:
		self.logger.Warn("Unknown pattern", "pattern", datagram.Pattern)
	}
}
