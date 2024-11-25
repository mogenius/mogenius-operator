package httpService

import (
	"encoding/json"
	"io"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	dbstats "mogenius-k8s-manager/src/db-stats"
	"mogenius-k8s-manager/src/interfaces"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/version"
	"net/http"
	"strconv"
)

type HttpService struct {
	logger *slog.Logger
	config interfaces.ConfigModule
}

func NewHttpApi(logManagerModule interfaces.LogManagerModule, configModule interfaces.ConfigModule) *HttpService {
	assert.Assert(logManagerModule != nil)
	assert.Assert(configModule != nil)
	return &HttpService{
		logger: logManagerModule.CreateLogger("http"),
		config: configModule,
	}
}

func (self *HttpService) Run(addr string) {
	assert.Assert(self.logger != nil)
	assert.Assert(self.config != nil)

	self.logger.Debug("initializing http.ServeMux", "addr", addr)
	mux := http.NewServeMux()

	// Deprecated: Typo in `GET /healtz`. Use `GET /healthz` instead.
	mux.Handle("GET /healtz", self.withRequestLogging(http.HandlerFunc(self.getHealthz)))
	mux.Handle("GET /healthz", self.withRequestLogging(http.HandlerFunc(self.getHealthz)))
	mux.Handle("POST /traffic", self.withRequestLogging(http.HandlerFunc(self.postTraffic)))
	mux.Handle("POST /cni", self.withRequestLogging(http.HandlerFunc(self.postCni)))
	mux.Handle("POST /podstats", self.withRequestLogging(http.HandlerFunc(self.postPodStats)))
	mux.Handle("POST /nodestats", self.withRequestLogging(http.HandlerFunc(self.postNodeStats)))

	moDebug, err := strconv.ParseBool(self.config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	if moDebug {
		self.logger.Debug("adding debug routes to http.ServeMux", "addr", addr)
		mux.Handle("GET /debug/sum-traffic", self.withRequestLogging(http.HandlerFunc(self.debugGetTrafficSum)))
		mux.Handle("GET /debug/traffic", self.withRequestLogging(http.HandlerFunc(self.debugGetTraffic)))
		mux.Handle("GET /debug/last-ns", self.withRequestLogging(http.HandlerFunc(self.debugGetLastNs)))
		mux.Handle("GET /debug/ns", self.withRequestLogging(http.HandlerFunc(self.debugGetNs)))
	}

	self.logger.Info("starting API server", "addr", addr)
	err = http.ListenAndServe(addr, mux)
	if err != nil {
		self.logger.Error("failed to start api server", "error", err)
	}
}

func (h *HttpService) getHealthz(w http.ResponseWriter, _ *http.Request) {
	healthStatus := map[string]string{
		"version": version.Ver,
		"branch":  version.Branch,
		"hash":    version.GitCommitHash,
		"buildAt": version.BuildTimestamp,
		"stage":   h.config.Get("MO_STAGE"),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(healthStatus)
	if err != nil {
		h.logger.Error("failed to json encode response", "error", err)
	}
}

func (h *HttpService) postTraffic(w http.ResponseWriter, r *http.Request) {
	debugMode, err := strconv.ParseBool(h.config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		return
	}

	if debugMode {
		var parsedJson interface{}
		err = json.Unmarshal(body, &parsedJson)
		if err != nil {
			h.logger.Error("failed to indent json", "error", err)
			return
		}
		h.logger.Debug("POST /traffic", "body", parsedJson)
	}

	stat := &structs.InterfaceStats{}
	err = structs.UnmarshalInterfaceStats(stat, body)
	if err != nil {
		h.logger.Error("failed to unmarshal interface stats", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		if err != nil {
			h.logger.Error("failed to json encode response", "error", err)
		}
		return
	}

	dbstats.AddInterfaceStatsToDb(*stat)
}

func (h *HttpService) postCni(w http.ResponseWriter, r *http.Request) {
	debugMode, err := strconv.ParseBool(h.config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		return
	}

	if debugMode {
		var parsedJson interface{}
		err = json.Unmarshal(body, &parsedJson)
		if err != nil {
			h.logger.Error("failed to indent json", "error", err)
			return
		}
		h.logger.Debug("POST /cni", "body", parsedJson)
	}

	cniData := &[]structs.CniData{}
	err = structs.UnmarshalCniData(cniData, body)
	if err != nil {
		h.logger.Error("failed to unmarshal cniData", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		if err != nil {
			h.logger.Error("failed to json encode response", "error", err)
		}
		return
	}

	dbstats.ReplaceCniData(*cniData)
}

func (h *HttpService) postPodStats(w http.ResponseWriter, r *http.Request) {
	debugMode, err := strconv.ParseBool(h.config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read request body", "error", err)
		return
	}

	if debugMode {
		var parsedJson interface{}
		err = json.Unmarshal(body, &parsedJson)
		if err != nil {
			h.logger.Error("failed to indent json", "error", err)
			return
		}
		h.logger.Debug("POST /podstats", "body", parsedJson)
	}

	stat := &structs.PodStats{}
	err = structs.UnmarshalPodStats(stat, body)
	if err != nil {
		h.logger.Error("failed to unmarshal interface stats", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		if err != nil {
			h.logger.Error("failed to json encode response", "error", err)
		}
		return
	}

	dbstats.AddPodStatsToDb(*stat)
}

func (self *HttpService) postNodeStats(w http.ResponseWriter, r *http.Request) {
	debugMode, err := strconv.ParseBool(self.config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		self.logger.Error("failed to read request body", "error", err)
		return
	}

	if debugMode {
		var parsedJson interface{}
		err = json.Unmarshal(body, &parsedJson)
		if err != nil {
			self.logger.Error("failed to indent json", "error", err)
			return
		}
		self.logger.Debug("POST /nodestats", "body", parsedJson)
	}

	stat := &structs.NodeStats{}
	err = structs.UnmarshalNodeStats(stat, body)
	if err != nil {
		self.logger.Error("failed to unmarshal interface stats", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		if err != nil {
			self.logger.Error("failed to json encode response", "error", err)
		}
		return
	}

	dbstats.AddNodeStatsToDb(*stat)
}

func (self *HttpService) debugGetTrafficSum(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")

	stats := dbstats.GetTrafficStatsEntriesSumForNamespace(ns)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *HttpService) debugGetLastNs(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	stats := dbstats.GetLastPodStatsEntriesForNamespace(ns)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *HttpService) debugGetTraffic(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	stats := dbstats.GetTrafficStatsEntriesForNamespace(ns)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *HttpService) debugGetNs(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	stats := dbstats.GetPodStatsEntriesForNamespace(ns)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}

func (self *HttpService) withRequestLogging(handler http.Handler) http.Handler {
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
