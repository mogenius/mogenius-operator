package api

import (
	"encoding/json"
	"io"
	"mogenius-k8s-manager/src/assert"
	dbstats "mogenius-k8s-manager/src/db-stats"
	iacmanager "mogenius-k8s-manager/src/iac-manager"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/version"
	"net/http"
	"strconv"

	punq "github.com/mogenius/punq/kubernetes"
)

func InitApi() {
	mux := http.NewServeMux()

	// Deprecated: Typo in `GET /healtz`. Use `GET /healthz` instead.
	mux.Handle("GET /healtz", withRequestLogging(http.HandlerFunc(getHealthz)))
	mux.Handle("GET /healthz", withRequestLogging(http.HandlerFunc(getHealthz)))
	mux.Handle("POST /traffic", withRequestLogging(http.HandlerFunc(postTraffic)))
	mux.Handle("POST /podstats", withRequestLogging(http.HandlerFunc(postPodStats)))
	mux.Handle("POST /nodestats", withRequestLogging(http.HandlerFunc(postNodeStats)))

	moDebug, err := strconv.ParseBool(config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	if moDebug {
		mux.Handle("GET /debug/sum-traffic", withRequestLogging(http.HandlerFunc(debugGetTrafficSum)))
		mux.Handle("GET /debug/traffic", withRequestLogging(http.HandlerFunc(debugGetTraffic)))
		mux.Handle("GET /debug/last-ns", withRequestLogging(http.HandlerFunc(debugGetLastNs)))
		mux.Handle("GET /debug/ns", withRequestLogging(http.HandlerFunc(debugGetNs)))
		mux.Handle("GET /debug/iac", withRequestLogging(http.HandlerFunc(debugIac)))
		mux.Handle("GET /debug/list-templates", withRequestLogging(http.HandlerFunc(debugListTemplates)))
	}

	port := "1337"
	httpLogger.Info("Starting API server...", "port", port)
	err = http.ListenAndServe(":"+port, mux)
	if err != nil {
		httpLogger.Error("failed to start api server", "error", err)
	}
}

func getHealthz(w http.ResponseWriter, _ *http.Request) {
	healthStatus := map[string]string{
		"version": version.Ver,
		"branch":  version.Branch,
		"hash":    version.GitCommitHash,
		"buildAt": version.BuildTimestamp,
		"stage":   config.Get("MO_STAGE"),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(healthStatus)
	if err != nil {
		httpLogger.Error("failed to json encode response", "error", err)
	}
}

func postTraffic(w http.ResponseWriter, r *http.Request) {
	debugMode, err := strconv.ParseBool(config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpLogger.Error("failed to read request body", "error", err)
		return
	}

	if debugMode {
		var parsedJson interface{}
		err = json.Unmarshal(body, &parsedJson)
		if err != nil {
			httpLogger.Error("failed to indent json", "error", err)
			return
		}
		httpLogger.Debug("POST /traffic", "body", parsedJson)
	}

	stat := &structs.InterfaceStats{}
	err = structs.UnmarshalInterfaceStats(stat, body)
	if err != nil {
		httpLogger.Error("failed to unmarshal interface stats", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		if err != nil {
			httpLogger.Error("failed to json encode response", "error", err)
		}
		return
	}

	dbstats.AddInterfaceStatsToDb(*stat)
}

func postPodStats(w http.ResponseWriter, r *http.Request) {
	debugMode, err := strconv.ParseBool(config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpLogger.Error("failed to read request body", "error", err)
		return
	}

	if debugMode {
		var parsedJson interface{}
		err = json.Unmarshal(body, &parsedJson)
		if err != nil {
			httpLogger.Error("failed to indent json", "error", err)
			return
		}
		httpLogger.Debug("POST /podstats", "body", parsedJson)
	}

	stat := &structs.PodStats{}
	err = structs.UnmarshalPodStats(stat, body)
	if err != nil {
		httpLogger.Error("failed to unmarshal interface stats", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		if err != nil {
			httpLogger.Error("failed to json encode response", "error", err)
		}
		return
	}

	dbstats.AddPodStatsToDb(*stat)
}

func postNodeStats(w http.ResponseWriter, r *http.Request) {
	debugMode, err := strconv.ParseBool(config.Get("MO_DEBUG"))
	assert.Assert(err == nil)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		httpLogger.Error("failed to read request body", "error", err)
		return
	}

	if debugMode {
		var parsedJson interface{}
		err = json.Unmarshal(body, &parsedJson)
		if err != nil {
			httpLogger.Error("failed to indent json", "error", err)
			return
		}
		httpLogger.Debug("POST /nodestats", "body", parsedJson)
	}

	stat := &structs.NodeStats{}
	err = structs.UnmarshalNodeStats(stat, body)
	if err != nil {
		httpLogger.Error("failed to unmarshal interface stats", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		if err != nil {
			httpLogger.Error("failed to json encode response", "error", err)
		}
		return
	}

	dbstats.AddNodeStatsToDb(*stat)
}

func debugGetTrafficSum(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")

	stats := dbstats.GetTrafficStatsEntriesSumForNamespace(ns)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		httpLogger.Error("failed to json encode response", "error", err)
	}
}

func debugGetLastNs(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	stats := dbstats.GetLastPodStatsEntriesForNamespace(ns)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		httpLogger.Error("failed to json encode response", "error", err)
	}
}

func debugGetTraffic(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	stats := dbstats.GetTrafficStatsEntriesForNamespace(ns)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		httpLogger.Error("failed to json encode response", "error", err)
	}
}

func debugGetNs(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	stats := dbstats.GetPodStatsEntriesForNamespace(ns)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(stats)
	if err != nil {
		httpLogger.Error("failed to json encode response", "error", err)
	}
}

func debugIac(w http.ResponseWriter, _ *http.Request) {
	json := iacmanager.GetDataModelJson()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(json))
	if err != nil {
		httpLogger.Debug("failed to write response", "error", err)
		return
	}
}

func debugListTemplates(w http.ResponseWriter, _ *http.Request) {
	data := punq.ListCreateTemplates()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		httpLogger.Error("failed to json encode response", "error", err)
	}
}
