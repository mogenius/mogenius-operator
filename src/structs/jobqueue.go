package structs

import (
	"context"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/websocket"
	"sync"
	"time"
)

var jobDataQueue []Datagram = []Datagram{}
var jobSendMutex sync.Mutex
var jobConnectionGuard = make(chan struct{}, 1)

func ConnectToJobQueue(jobClient websocket.WebsocketClient) {
	jobQueueCtx, cancel := context.WithCancel(context.Background())
	shutdown.Add(cancel)
	for {
		jobConnectionGuard <- struct{}{} // would block if guard channel is already filled
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			quit := make(chan struct{})
			go func() {
				for {
					select {
					case <-jobQueueCtx.Done():
						return
					case <-quit:
						return
					case <-ticker.C:
						processJobNow(jobClient)
					}
				}
			}()
			select {
			case <-jobQueueCtx.Done():
				return
			case <-jobConnectionGuard:
			}
			ticker.Stop()
			close(quit)
		}()
		select {
		case <-jobQueueCtx.Done():
			structsLogger.Debug("shutting down jobqueue")
			return
		case <-time.After(RETRYTIMEOUT * time.Second):
		}
	}
}

func processJobNow(jobClient websocket.WebsocketClient) {
	jobSendMutex.Lock()
	defer jobSendMutex.Unlock()

	for i := 0; i < len(jobDataQueue); i++ {
		element := jobDataQueue[i]
		err := jobClient.WriteJSON(element)
		if err == nil {
			element.DisplaySentSummary(structsLogger, i+1, len(jobDataQueue))
			structsLogger.Debug("sent summary", "payload", element.Payload)
			jobDataQueue = removeJobIndex(jobDataQueue, i)
		} else {
			structsLogger.Error("Error writing json in job queue", "error", err)
			return
		}
	}
}

func removeJobIndex(s []Datagram, index int) []Datagram {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}
