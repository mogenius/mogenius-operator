package structs

import (
	"context"
	"fmt"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var queueConnection *websocket.Conn

var dataQueue []Datagram = []Datagram{}

func ConnectToEventQueue() {
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

	queueConnection, _, err := websocket.DefaultDialer.Dial(connectionUrl.String(), http.Header{
		"x-authorization": []string{utils.CONFIG.Kubernetes.ApiKey},
		"x-cluster-id":    []string{utils.CONFIG.Kubernetes.ClusterId},
		"x-app":           []string{APP_NAME},
		"x-cluster-name":  []string{utils.CONFIG.Kubernetes.ClusterName}})
	if err != nil {
		logger.Log.Errorf("Connection to EventServer failed: %s\n", err.Error())
	} else {
		logger.Log.Infof("Connected to EventServer: %s \n", queueConnection.RemoteAddr())
		observeConnection(queueConnection)
	}

	defer func() {
		// reset everything if connection dies
		if queueConnection != nil {
			queueConnection.Close()
		}
		ctx.Done()
	}()
}

func observeConnection(connection *websocket.Conn) {
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

func EventServerSendData(datagram Datagram, eventName *string) {
	dataQueue = append(dataQueue, datagram)

	for i := 0; i < len(dataQueue); i++ {
		element := dataQueue[i]
		if queueConnection != nil {
			err := queueConnection.WriteJSON(element)
			if err == nil {
				dataQueue = RemoveIndex(dataQueue, i)
			} else {
				return
			}
			if eventName != nil {
				datagram.DisplaySentSummaryEvent(*eventName)
			}
		}
	}
}

func RemoveIndex(s []Datagram, index int) []Datagram {
	return append(s[:index], s[index+1:]...)
}
