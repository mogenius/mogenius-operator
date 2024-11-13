package structs

import (
	"context"
	"mogenius-k8s-manager/src/utils"
	"net/url"
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
var EventQueueConnection *websocket.Conn
var eventConnectionGuard = make(chan struct{}, 1)
var EventConnectionStatus chan bool = make(chan bool)
var eventDataQueue []EventData = []EventData{}
var EventConnectionUrl url.URL = url.URL{}

func ConnectToEventQueue() {
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

		<-time.After(RETRYTIMEOUT * time.Second)
	}
}

func connectEvent(ctx context.Context) {
	EventConnectionUrl, err := url.Parse(config.Get("MO_EVENT_SERVER"))
	if err != nil {
		structsLogger.Error("failed to parse MO_EVENT_SERVER as URL", "error", err)
		EventConnectionStatus <- false
		return
	}

	connection, _, err := websocket.DefaultDialer.Dial(EventConnectionUrl.String(), utils.HttpHeader(""))
	if err != nil {
		structsLogger.Error("Connection to EventServer failed", "url", EventConnectionUrl.String(), "error", err)
		EventConnectionStatus <- false
	} else {
		structsLogger.Info("Connected to EventServer", "url", EventConnectionUrl.String(), "localAddr", connection.LocalAddr().String())
		EventQueueConnection = connection
		EventConnectionStatus <- true
		err := Ping(EventQueueConnection, &eventSendMutex)
		if err != nil {
			structsLogger.Error("Error pinging event queue", "error", err)
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
				eventDataQueue = RemoveEventIndex(eventDataQueue, i)
			} else {
				structsLogger.Error("Error sending data to EventServer", "error", err)
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
