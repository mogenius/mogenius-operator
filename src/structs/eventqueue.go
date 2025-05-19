package structs

import (
	"context"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/websocket"
	"sync"
	"time"
)

// TODO: @daniel -> use channels and goroutine-local queue instead

const RETRYTIMEOUT time.Duration = 3

var eventDataQueue []Datagram = []Datagram{}
var eventDataQueueMutex sync.Mutex
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

func EventServerSendData(eventClient websocket.WebsocketClient, datagram Datagram) {
	eventDataQueueMutex.Lock()
	eventDataQueue = append(eventDataQueue, datagram)
	eventDataQueueMutex.Unlock()
	processEventQueueNow(eventClient)
}

func processEventQueueNow(eventClient websocket.WebsocketClient) {
	eventSendMutex.Lock()
	defer eventSendMutex.Unlock()

	for i := 0; i < len(eventDataQueue); i++ {
		element := eventDataQueue[i]

		err := eventClient.WriteJSON(element)
		if err != nil {
			structsLogger.Error("Error sending data to EventServer", "error", err)
			continue
		}

		eventDataQueueMutex.Lock()
		eventDataQueue = RemoveEventIndex(eventDataQueue, i)
		eventDataQueueMutex.Unlock()
	}
}

func RemoveEventIndex(s []Datagram, index int) []Datagram {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}
