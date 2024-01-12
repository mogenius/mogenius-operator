package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	dbstats "mogenius-k8s-manager/db-stats"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/version"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func InitApi() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(CreateLogger("INTERNAL"))
	router.GET("/healtz", getHealtz)
	router.POST("/traffic", postTraffic)
	router.POST("/podstats", postPodStats)

	router.GET("/debug/last-traffic", debugGetLastTraffic)
	router.GET("/debug/last-pod", debugGetLastPod)

	srv := &http.Server{
		Addr:    ":1337",
		Handler: router,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			logger.Log.Info("listen: %s\n", err)
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
	logger.Log.Warning("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Warning("Server forced to shutdown:", err)
	}

	logger.Log.Warning("Server exiting")
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

	if utils.CONFIG.Misc.Debug {
		err := json.Indent(&out, []byte(body), "", "  ")
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(string(out.Bytes()))
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

	if utils.CONFIG.Misc.Debug {
		err := json.Indent(&out, []byte(body), "", "  ")
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(string(out.Bytes()))
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

func debugGetLastTraffic(c *gin.Context) {
	ns := c.Query("ns")
	pod := c.Query("pod")

	controller := kubernetes.ControllerForPod(ns, pod)
	if controller == nil {
		c.IndentedJSON(http.StatusNotFound, map[string]string{
			"error": "controller not found",
		})
		return
	}
	stats := dbstats.GetLastTrafficStatsEntryForController(*controller)
	c.IndentedJSON(http.StatusOK, stats)
}

func debugGetLastPod(c *gin.Context) {
	ns := c.Query("ns")
	pod := c.Query("pod")

	controller := kubernetes.ControllerForPod(ns, pod)
	if controller == nil {
		c.IndentedJSON(http.StatusNotFound, map[string]string{
			"error": "controller not found",
		})
		return
	}
	stats := dbstats.GetLastPodStatsEntryForController(*controller)
	c.IndentedJSON(http.StatusOK, stats)
}
