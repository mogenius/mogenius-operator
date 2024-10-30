package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	dbstats "mogenius-k8s-manager/db-stats"
	iacmanager "mogenius-k8s-manager/iac-manager"
	"mogenius-k8s-manager/logging"
	"mogenius-k8s-manager/services"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/version"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	punq "github.com/mogenius/punq/kubernetes"

	"github.com/gin-gonic/gin"
)

var HttpLogger = logging.CreateLogger("http")

func InitApi() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(CreateLogger("INTERNAL"))
	router.GET("/healtz", getHealtz)
	router.POST("/traffic", postTraffic)
	router.POST("/podstats", postPodStats)
	router.POST("/nodestats", postNodeStats)

	if utils.CONFIG.Misc.Debug {
		router.GET("/debug/sum-traffic", debugGetTrafficSum)
		router.GET("/debug/traffic", debugGetTraffic)
		router.GET("/debug/last-ns", debugGetLastNs)
		router.GET("/debug/ns", debugGetNs)
		router.GET("/debug/chart", debugChart)
		router.GET("/debug/iac", debugIac)
		router.GET("/debug/list-templates", debugListTemplates)
	}

	srv := &http.Server{
		Addr:    ":1337",
		Handler: router,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			HttpLogger.Error("failed to server", "error", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be caught, so don't need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	HttpLogger.Warn("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		HttpLogger.Warn("Server forced to shutdown", "error", err)
	}

	HttpLogger.Warn("Server exiting")
}

func getHealtz(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, map[string]string{
		"version": version.Ver,
		"branch":  version.Branch,
		"hash":    version.GitCommitHash,
		"buildAt": version.BuildTimestamp,
		"stage":   utils.CONFIG.Misc.Stage,
	})
}

func postTraffic(c *gin.Context) {
	var out bytes.Buffer
	body, _ := io.ReadAll(c.Request.Body)

	err := json.Indent(&out, []byte(body), "", "  ")
	if err != nil {
		HttpLogger.Error("Error indenting json", "error", err)
	}
	if utils.CONFIG.Misc.LogIncomingStats {
		HttpLogger.Info(out.String())
	}

	stat := &structs.InterfaceStats{}
	err = structs.UnmarshalInterfaceStats(stat, out.Bytes())
	if err != nil {
		HttpLogger.Error("Error unmarshalling interface stats", "error", err)
		c.IndentedJSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	dbstats.AddInterfaceStatsToDb(*stat)
}

func postPodStats(c *gin.Context) {
	var out bytes.Buffer
	body, _ := io.ReadAll(c.Request.Body)

	err := json.Indent(&out, []byte(body), "", "  ")
	if err != nil {
		HttpLogger.Error("Error indenting json", "error", err)
	}
	if utils.CONFIG.Misc.LogIncomingStats {
		HttpLogger.Info(out.String())
	}

	stat := &structs.PodStats{}
	err = structs.UnmarshalPodStats(stat, out.Bytes())
	if err != nil {
		HttpLogger.Error("Error unmarshalling pod stats", "error", err.Error())
		c.IndentedJSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	dbstats.AddPodStatsToDb(*stat)
}

func postNodeStats(c *gin.Context) {
	var out bytes.Buffer
	body, _ := io.ReadAll(c.Request.Body)

	err := json.Indent(&out, []byte(body), "", "  ")
	if err != nil {
		HttpLogger.Error("Error indenting json", "error", err)
	}
	if utils.CONFIG.Misc.LogIncomingStats {
		HttpLogger.Info(out.String())
	}

	stat := &structs.NodeStats{}
	err = structs.UnmarshalNodeStats(stat, out.Bytes())
	if err != nil {
		HttpLogger.Error("Error unmarshalling node stats", "error", err.Error())
		c.IndentedJSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	dbstats.AddNodeStatsToDb(*stat)
}

func debugGetTrafficSum(c *gin.Context) {
	ns := c.Query("ns")

	stats := dbstats.GetTrafficStatsEntriesSumForNamespace(ns)
	c.IndentedJSON(http.StatusOK, stats)
}

func debugGetLastNs(c *gin.Context) {
	ns := c.Query("ns")
	stats := dbstats.GetLastPodStatsEntriesForNamespace(ns)
	c.IndentedJSON(http.StatusOK, stats)
}

func debugGetTraffic(c *gin.Context) {
	ns := c.Query("ns")
	stats := dbstats.GetTrafficStatsEntriesForNamespace(ns)
	c.IndentedJSON(http.StatusOK, stats)
}

func debugGetNs(c *gin.Context) {
	ns := c.Query("ns")
	stats := dbstats.GetPodStatsEntriesForNamespace(ns)
	c.IndentedJSON(http.StatusOK, stats)
}

func debugChart(c *gin.Context) {
	ns := c.Query("namespace")
	podname := c.Query("podname")
	html := services.RenderPodNetworkTreePageHtml(ns, podname)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func debugIac(c *gin.Context) {
	json := iacmanager.GetDataModelJson()
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(json))
}

func debugListTemplates(c *gin.Context) {
	data := punq.ListCreateTemplates()

	c.IndentedJSON(http.StatusOK, data)
}
