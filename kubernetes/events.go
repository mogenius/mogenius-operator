package kubernetes

import (
	"context"
	"fmt"
	"log"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

func ObserveKubernetesEvents() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	connectionGuard := make(chan struct{}, CONCURRENTCONNECTIONS)

	for {
		select {
		case <-interrupt:
			log.Fatal("CTRL + C pressed. Terminating.")
		case <-time.After(RETRYTIMEOUT * time.Second):
		}

		connectionGuard <- struct{}{} // would block if guard channel is already filled
		go func() {
			connect()
			<-connectionGuard
		}()
	}
}

func connect() {
	ctx := context.Background()
	host := fmt.Sprintf("%s:%d", utils.CONFIG.EventServer.Server, utils.CONFIG.EventServer.Port)
	connectionUrl := url.URL{Scheme: "ws", Host: host, Path: utils.CONFIG.EventServer.Path}

	connection, _, err := websocket.DefaultDialer.Dial(connectionUrl.String(), http.Header{
		"x-authorization": []string{utils.CONFIG.Kubernetes.ApiKey},
		"x-cluster-id":    []string{utils.CONFIG.Kubernetes.ClusterId},
		"x-cluster-name":  []string{utils.CONFIG.Kubernetes.ClusterName}})
	if err != nil {
		logger.Log.Errorf("Connection to EventServer failed: %s\n", err.Error())
	} else {
		logger.Log.Infof("Connected to EventServer: %s \n", connection.RemoteAddr())
		go observceConnection(connection)
		watchEvents(connection, ctx)
	}

	defer func() {
		// reset everything if connection dies
		if connection != nil {
			connection.Close()
		}
		ctx.Done()
	}()
}

func observceConnection(connection *websocket.Conn) {
	for {
		if connection == nil {
			return
		}

		msgType, _, err := connection.ReadMessage()
		if err != nil {
			logger.Log.Error("websocket read err:", err)
			connection.Close()
			return
		}

		switch msgType {
		case websocket.CloseMessage:
			logger.Log.Warning("Received websocket.CloseMessage.")
			connection.Close()
			return
		}
	}
}

func watchEvents(connection *websocket.Conn, ctx context.Context) {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		logger.Log.Errorf("CreateDeployment ERROR: %s", err.Error())
	}

	watcher, err := kubeProvider.ClientSet.CoreV1().Events("").Watch(ctx, v1.ListOptions{Watch: true})
	defer watcher.Stop()

	if err != nil {
		logger.Log.Error(err.Error())
	}

	for {
		select {
		case event := <-watcher.ResultChan():
			eventDto := dtos.CreateEvent(string(event.Type), event.Object)
			datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto, connection)
			//structs.PrettyPrint(eventDto)
			datagram.Send()

		case <-ctx.Done():
			logger.Log.Error("Stopped watching events!")
			return
		}
	}
}
