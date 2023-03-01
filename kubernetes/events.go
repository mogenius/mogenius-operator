package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RETRYTIMEOUT time.Duration = 3

func ObserveKubernetesEvents() {
	for {
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
			watchEvents(connection, ctx)
		}

		// reset everything if connection dies
		if connection != nil {
			connection.Close()
		}
		ctx.Done()
		time.Sleep(RETRYTIMEOUT * time.Second)
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
