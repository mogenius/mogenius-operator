package structs

import (
	"context"
	"fmt"
	"log"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type EventData struct {
	Datagram   Datagram
	K8sKind    string
	K8sReason  string
	K8sMessage string
	Count      int32
}

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var eventSendMutex sync.Mutex

var queueConnection *websocket.Conn

var dataQueue []EventData = []EventData{}

func init() {
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for range ticker.C {
			processQueueNow()
		}
	}()
}

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

	connection, _, err := websocket.DefaultDialer.Dial(connectionUrl.String(), utils.HttpHeader())
	if err != nil {
		logger.Log.Errorf("Connection to EventServer failed: %s\n", err.Error())
	} else {
		logger.Log.Infof("Connected to EventServer: %s \n", connectionUrl.String())
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

func EventServerSendData(datagram Datagram, k8sKind string, k8sReason string, k8sMessage string, count int32) {
	eventSendMutex.Lock()
	defer eventSendMutex.Unlock()

	data := EventData{
		Datagram:   datagram,
		K8sKind:    k8sKind,
		K8sReason:  k8sReason,
		K8sMessage: k8sMessage,
		Count:      count,
	}
	dataQueue = append(dataQueue, data)
}

func processQueueNow() {
	eventSendMutex.Lock()
	defer eventSendMutex.Unlock()

	for i := 0; i < len(dataQueue); i++ {
		element := dataQueue[i]
		if queueConnection != nil {
			err := queueConnection.WriteJSON(element)
			if err == nil {
				if element.K8sKind != "" && element.K8sReason != "" && element.K8sMessage != "" {
					if utils.CONFIG.Misc.Debug && utils.CONFIG.Misc.LogKubernetesEvents {
						element.Datagram.DisplaySentSummaryEvent(element.K8sKind, element.K8sReason, element.K8sMessage, element.Count)
					}
				}
				dataQueue = RemoveIndex(dataQueue, i)
			} else {
				logger.Log.Error(err)
				return
			}
		} else {
			logger.Log.Error("queueConnection is nil.")
		}
	}
}

func RemoveIndex(s []EventData, index int) []EventData {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}
