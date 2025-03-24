package structs

import (
	"context"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/websocket"
	"sync"
	"time"
)

type EventData struct {
	Datagram   Datagram
	K8sKind    string
	K8sReason  string
	K8sMessage string
	Count      int32
	EventType  string
}

const RETRYTIMEOUT time.Duration = 3

var eventDataQueue []EventData = []EventData{}
var eventSendMutex sync.Mutex
var eventConnectionGuard = make(chan struct{}, 1)

func ConnectToEventQueue(eventClient websocket.WebsocketClient) {
	eventQueueCtx, cancel := context.WithCancel(context.Background())
	shutdown.Add(cancel)
	for {
		eventConnectionGuard <- struct{}{} // would block if guard channel is already filled
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			quit := make(chan struct{})

			go func() {
				for {
					select {
					case <-eventQueueCtx.Done():
						return
					case <-quit:
						return
					case <-ticker.C:
						processEventQueueNow(eventClient)
					}
				}
			}()

			ctx := context.Background()
			ctx.Done()
			select {
			case <-eventQueueCtx.Done():
				return
			case <-eventConnectionGuard:
			}

			ticker.Stop()
			close(quit)
		}()

		select {
		case <-eventQueueCtx.Done():
			structsLogger.Debug("shutting down eventsqueue")
			return
		case <-time.After(RETRYTIMEOUT * time.Second):
		}
	}
}

func EventServerSendData(eventClient websocket.WebsocketClient, datagram Datagram, k8sKind string, k8sReason string, k8sMessage string, count int32, eventType string) {
	data := EventData{
		Datagram:   datagram,
		K8sKind:    k8sKind,
		K8sReason:  k8sReason,
		K8sMessage: k8sMessage,
		Count:      count,
		EventType:  eventType,
	}
	eventDataQueue = append(eventDataQueue, data)
	processEventQueueNow(eventClient)
}

func processEventQueueNow(eventClient websocket.WebsocketClient) {
	eventSendMutex.Lock()
	defer eventSendMutex.Unlock()

	for i := 0; i < len(eventDataQueue); i++ {
		element := eventDataQueue[i]

		err := eventClient.WriteJSON(element.Datagram)
		if err == nil {
			eventDataQueue = RemoveEventIndex(eventDataQueue, i)
		} else {
			structsLogger.Error("Error sending data to EventServer", "error", err)
		}
	}
}

func RemoveEventIndex(s []EventData, index int) []EventData {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}
