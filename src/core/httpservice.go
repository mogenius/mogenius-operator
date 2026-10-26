package core

import (
	"context"
	"errors"
	"log/slog"
	"mogenius-operator/src/assert"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/logging"
	moreconciler "mogenius-operator/src/reconciler"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/version"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"encoding/json"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HttpService interface {
	Run()
	Link(socketapi SocketApi, dbstats ValkeyStatsDb, apiModule Api, reconciler moreconciler.Reconciler)
}

type httpService struct {
	logger     *slog.Logger
	config     cfg.ConfigModule
	dbstats    ValkeyStatsDb
	api        Api
	reconciler moreconciler.Reconciler

	socketapi SocketApi
}

func NewHttpApi(
	logManagerModule logging.SlogManager,
	configModule cfg.ConfigModule,
) HttpService {
	assert.Assert(logManagerModule != nil)
	assert.Assert(configModule != nil)

	self := &httpService{}

	self.logger = logManagerModule.CreateLogger("http")
	self.config = configModule
	return self
}

func (self *httpService) Run() {
	assert.Assert(self.logger != nil)
	assert.Assert(self.config != nil)
	assert.Assert(self.socketapi != nil)

	addr := self.config.Get("MO_HTTP_ADDR")

	self.logger.Debug("initializing http.ServeMux", "addr", addr)
	mux := http.NewServeMux()

	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /healthz", self.withRequestLogging(http.HandlerFunc(self.getHealthz)))

	if os.Getenv("MO_PPROF") == "true" {
		mux.Handle("/debug/pprof/", http.DefaultServeMux)
		self.logger.Info("pprof enabled", "path", "/debug/pprof/")
	}

	mux.Handle("GET /status", self.withRequestLogging(http.HandlerFunc(self.getAppStatus)))

	mux.HandleFunc("/stats", self.serveNodeStatsHtml)

	self.addApiRoutes(mux)

	// ReadHeaderTimeout blocks slowloris-style attacks; IdleTimeout reaps
	// keep-alive connections from gone clients. We leave Read/WriteTimeout
	// unset because terminal/log-stream/websocket handlers legitimately run
	// for minutes/hours; per-handler timeouts handle those cases.
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Drain in-flight requests on shutdown instead of cutting the listener
	// mid-write. Hijacked connections (websocket/terminal/log streams) are not
	// tracked by Shutdown and end with the process, as before.
	shutdown.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			self.logger.Warn("http server graceful shutdown failed", "error", err)
		}
	})

	self.logger.Info("starting API server", "addr", addr)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			self.logger.Error("failed to start api server", "error", err)
		}
	}()
}

func (self *httpService) Link(socketapi SocketApi, dbstats ValkeyStatsDb, apiModule Api, reconciler moreconciler.Reconciler) {
	assert.Assert(socketapi != nil)
	assert.Assert(dbstats != nil)
	assert.Assert(apiModule != nil)
	assert.Assert(reconciler != nil)

	self.socketapi = socketapi
	self.dbstats = dbstats
	self.api = apiModule
	self.reconciler = reconciler
}

func (self *httpService) getHealthz(w http.ResponseWriter, _ *http.Request) {
	healthStatus := map[string]string{
		"version": version.Ver,
		"branch":  version.Branch,
		"hash":    version.GitCommitHash,
		"buildAt": version.BuildTimestamp,
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

func (self *httpService) getAppStatus(w http.ResponseWriter, _ *http.Request) {
	status := map[string]any{}
	status["reconciler"] = self.reconciler.Status()
	status["socketapi"] = self.socketapi.Status()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(status)
	if err != nil {
		self.logger.Error("failed to json encode response", "error", err)
	}
}
