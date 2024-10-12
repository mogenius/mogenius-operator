package structs

import (
	"context"
	"mogenius-k8s-manager/utils"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	log "github.com/sirupsen/logrus"
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
var EventQueueConnection *websocket.Conn
var eventConnectionGuard = make(chan struct{}, 1)
var EventConnectionStatus chan bool = make(chan bool)
var eventDataQueue []EventData = []EventData{}
var EventConnectionUrl url.URL = url.URL{}

func ConnectToEventQueue() {
	interrupt := make(chan os.Signal, 1)
	defer close(interrupt)
	signal.Notify(interrupt, os.Interrupt)

	for {
		eventConnectionGuard <- struct{}{} // would block if guard channel is already filled
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			quit := make(chan struct{})

			go func() {
				for {
					select {
					case <-ticker.C:
						processEventQueueNow()
					case <-quit:
						// close go routine
						return
					}
				}
			}()

			ctx := context.Background()
			connectEvent(ctx)
			ctx.Done()
			<-eventConnectionGuard

			ticker.Stop()
			close(quit)
		}()

		select {
		case <-interrupt:
			log.Fatal("CTRL + C pressed. Terminating.")
		case <-time.After(RETRYTIMEOUT * time.Second):
		}

	}
}

func connectEvent(ctx context.Context) {
	EventConnectionUrl = url.URL{Scheme: utils.CONFIG.EventServer.Scheme, Host: utils.CONFIG.EventServer.Server, Path: utils.CONFIG.EventServer.Path}

	connection, _, err := websocket.DefaultDialer.Dial(EventConnectionUrl.String(), utils.HttpHeader(""))
	if err != nil {
		log.Errorf("Connection to EventServer failed (%s): %s\n", EventConnectionUrl.String(), err.Error())
		EventConnectionStatus <- false
	} else {
		log.Infof("Connected to EventServer: %s  (%s)\n", EventConnectionUrl.String(), connection.LocalAddr().String())
		EventQueueConnection = connection
		EventConnectionStatus <- true
		err := Ping(EventQueueConnection, &eventSendMutex)
		if err != nil {
			log.Error(err)
		}
	}

	defer func() {
		// reset everything if connection dies
		if EventQueueConnection != nil {
			EventQueueConnection.Close()
		}
		ctx.Done()
		EventConnectionStatus <- false
	}()
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

func processEventQueueNow() {
	eventSendMutex.Lock()
	defer eventSendMutex.Unlock()

	if EventQueueConnection != nil {
		for i := 0; i < len(eventDataQueue); i++ {
			element := eventDataQueue[i]

			err := EventQueueConnection.WriteJSON(element.Datagram)
			if err == nil {
				if element.K8sKind != "" && element.K8sReason != "" && element.K8sMessage != "" {
					if utils.CONFIG.Misc.LogKubernetesEvents {
						element.Datagram.DisplaySentSummaryEvent(element.K8sKind, element.K8sReason, element.K8sMessage, element.Count)
					}
				}
				eventDataQueue = RemoveEventIndex(eventDataQueue, i)
			} else {
				log.Error("Error sending data to EventServer: ", err.Error())
			}
		}
	}
}

func RemoveEventIndex(s []EventData, index int) []EventData {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}
