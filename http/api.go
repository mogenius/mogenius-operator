package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	dbstats "mogenius-k8s-manager/db-stats"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/version"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func InitApi() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(CreateLogger("INTERNAL"))
	router.GET("/healtz", getHealtz)
	router.POST("/traffic", postTraffic)
	router.POST("/podstats", postPodStats)
	router.POST("/nodestats", postNodeStats)

	router.GET("/debug/last-traffic", debugGetLastTraffic)
	router.GET("/debug/traffic", debugGetTraffic)
	router.GET("/debug/last-ns", debugGetLastNs)
	router.GET("/debug/ns", debugGetNs)

	srv := &http.Server{
		Addr:    ":1337",
		Handler: router,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			log.Info("listen: %s\n", err.Error())
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be caught, so don't need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Warning("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Warning("Server forced to shutdown:", err)
	}

	log.Warning("Server exiting")
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

	if utils.CONFIG.Misc.LogIncomingStats {
		err := json.Indent(&out, []byte(body), "", "  ")
		if err != nil {
			log.Error(err)
		}
		log.Info(out.String())
	}

	stat := &structs.InterfaceStats{}
	err := structs.UnmarshalInterfaceStats(stat, out.Bytes())
	if err != nil {
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

	if utils.CONFIG.Misc.LogIncomingStats {
		err := json.Indent(&out, []byte(body), "", "  ")
		if err != nil {
			log.Error(err)
		}
		log.Info(out.String())
	}

	stat := &structs.PodStats{}
	err := structs.UnmarshalPodStats(stat, out.Bytes())
	if err != nil {
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

	if utils.CONFIG.Misc.LogIncomingStats {
		err := json.Indent(&out, []byte(body), "", "  ")
		if err != nil {
			log.Error(err)
		}
		log.Info(out.String())
	}

	stat := &structs.NodeStats{}
	err := structs.UnmarshalNodeStats(stat, out.Bytes())
	if err != nil {
		c.IndentedJSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	dbstats.AddNodeStatsToDb(*stat)
}

func debugGetLastTraffic(c *gin.Context) {
	ns := c.Query("ns")

	stats := dbstats.GetLastTrafficStatsEntriesForNamespace(ns)
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
