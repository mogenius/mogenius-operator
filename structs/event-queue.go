package structs

import (
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

var eventSendMutex sync.Mutex
var eventQueueConnection *websocket.Conn
var eventDataQueue []EventData = []EventData{}

func ConnectToEventQueue() {
	interrupt := make(chan os.Signal, 1)
	defer close(interrupt)
	signal.Notify(interrupt, os.Interrupt)

	ticker := time.NewTicker(1 * time.Second)

	// ALWAYS PROCESS_EVENT_QUEUE
	go func() {
		for range ticker.C {
			_ = processEventQueueNow()
		}
	}()

	for {
		select {
		case <-interrupt:
			log.Fatal("CTRL + C pressed. Terminating.")
		case <-time.After(RETRYTIMEOUT * time.Second):
		}

		// connect
		connectEvent()
		time.Sleep(3 * time.Second)
	}
}

func connectEvent() {
	defer func() {
		// reset everything if connection dies
		if eventQueueConnection != nil {
			eventQueueConnection.Close()
		}
	}()

	scheme := "wss"
	if utils.CONFIG.Misc.Stage == "local" {
		scheme = "ws"
	}
	connectionUrl := url.URL{Scheme: scheme, Host: utils.CONFIG.EventServer.Server, Path: utils.CONFIG.EventServer.Path}

	connection, _, err := websocket.DefaultDialer.Dial(connectionUrl.String(), utils.HttpHeader(""))
	if err != nil {
		logger.Log.Errorf("Connection to EventServer failed (%s): %s\n", connectionUrl.String(), err.Error())
		return
	} else {
		logger.Log.Infof("Connected to EventServer: %s  (%s)\n", connectionUrl.String(), connection.LocalAddr().String())
		eventQueueConnection = connection
		Ping(eventQueueConnection, &eventSendMutex)
		return
	}
}

func EventServerSendData(datagram Datagram, k8sKind string, k8sReason string, k8sMessage string, count int32) {
	data := EventData{
		Datagram:   datagram,
		K8sKind:    k8sKind,
		K8sReason:  k8sReason,
		K8sMessage: k8sMessage,
		Count:      count,
	}
	eventDataQueue = append(eventDataQueue, data)
	processEventQueueNow()
}

func processEventQueueNow() error {
	eventSendMutex.Lock()
	defer eventSendMutex.Unlock()

	if eventQueueConnection != nil {
		for i := 0; i < len(eventDataQueue); i++ {
			element := eventDataQueue[i]

			err := eventQueueConnection.WriteJSON(element.Datagram)
			if err == nil {
				if element.K8sKind != "" && element.K8sReason != "" && element.K8sMessage != "" {
					if utils.CONFIG.Misc.LogKubernetesEvents {
						element.Datagram.DisplaySentSummaryEvent(element.K8sKind, element.K8sReason, element.K8sMessage, element.Count)
					}
				}
				eventDataQueue = RemoveEventIndex(eventDataQueue, i)
			} else {
				logger.Log.Error(err)
				return err
			}
		}
	} else {
		if utils.CONFIG.Misc.Debug {
			// logger.Log.Error("eventQueueConnection is nil.")
		}
		return fmt.Errorf("eventQueueConnection is nil")
	}
	return nil
}

func RemoveEventIndex(s []EventData, index int) []EventData {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}
