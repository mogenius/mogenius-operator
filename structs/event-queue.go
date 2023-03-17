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
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var eventSendMutex sync.Mutex

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
			ctx := context.Background()
			connect(ctx)
			ctx.Done()
			<-connectionGuard
		}()
	}
}

func connect(ctx context.Context) {
	host := fmt.Sprintf("%s:%d", utils.CONFIG.EventServer.Server, utils.CONFIG.EventServer.Port)
	connectionUrl := url.URL{Scheme: "ws", Host: host, Path: utils.CONFIG.EventServer.Path}

	connection, _, err := websocket.DefaultDialer.Dial(connectionUrl.String(), http.Header{
		"x-authorization":  []string{utils.CONFIG.Kubernetes.ApiKey},
		"x-cluster-mfa-id": []string{utils.CONFIG.Kubernetes.ClusterMfaId},
		"x-app":            []string{APP_NAME},
		"x-cluster-name":   []string{utils.CONFIG.Kubernetes.ClusterName}})
	if err != nil {
		logger.Log.Errorf("Connection to EventServer failed: %s\n", err.Error())
	} else {
		logger.Log.Infof("Connected to EventServer: %s \n", connection.RemoteAddr())
		queueConnection = connection
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
			eventSendMutex.Lock()
			err := queueConnection.WriteJSON(element)
			eventSendMutex.Unlock()
			if err == nil {
				if eventName != nil {
					datagram.DisplaySentSummaryEvent(*eventName)
				}
				dataQueue = RemoveIndex(dataQueue, i)
			} else {
				return
			}
		}
	}
}

func RemoveIndex(s []Datagram, index int) []Datagram {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}
