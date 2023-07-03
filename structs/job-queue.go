package structs

import (
	"context"
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

var jobDataQueue []Datagram = []Datagram{}

// Public
var JobSendMutex sync.Mutex
var JobQueueConnection *websocket.Conn
var JobConnectionGuard = make(chan struct{}, CONCURRENTCONNECTIONS)
var JobConnectionStatus chan bool = make(chan bool)

func ConnectToJobQueue() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	for {
		select {
		case <-interrupt:
			log.Fatal("CTRL + C pressed. Terminating.")
		case <-time.After(RETRYTIMEOUT * time.Second):
		}

		JobConnectionGuard <- struct{}{} // would block if guard channel is already filled
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			go func() {
				defer ticker.Stop()
				for range ticker.C {
					processJobNow()
				}
			}()

			ctx := context.Background()
			connectJob(ctx)
			ctx.Done()
			<-JobConnectionGuard
		}()
	}
}

func connectJob(ctx context.Context) {
	scheme := "wss"
	if utils.CONFIG.Misc.Stage == "local" {
		scheme = "ws"
	}
	connectionUrl := url.URL{Scheme: scheme, Host: utils.CONFIG.ApiServer.Ws_Server, Path: utils.CONFIG.ApiServer.WS_Path}

	connection, _, err := websocket.DefaultDialer.Dial(connectionUrl.String(), utils.HttpHeader())
	if err != nil {
		logger.Log.Errorf("Connection to JobServer failed: %s\n", err.Error())
		JobConnectionStatus <- false
	} else {
		logger.Log.Infof("Connected to JobServer: %s  (%s)\n", connectionUrl.String(), connection.LocalAddr().String())
		JobQueueConnection = connection
		JobConnectionStatus <- true
		done := make(chan struct{})
		Ping(done, JobQueueConnection, &JobSendMutex)
	}

	defer func() {
		// reset everything if connection dies
		if JobQueueConnection != nil {
			JobQueueConnection.Close()
		}
		ctx.Done()
		JobConnectionStatus <- false
	}()
}

func JobServerSendData(datagram Datagram) {
	jobDataQueue = append(jobDataQueue, datagram)
	processJobNow()
}

func processJobNow() {
	JobSendMutex.Lock()
	defer JobSendMutex.Unlock()

	if JobQueueConnection != nil {
		for i := 0; i < len(jobDataQueue); i++ {
			element := jobDataQueue[i]

			err := JobQueueConnection.WriteJSON(element)
			if err == nil {
				jobDataQueue = RemoveJobIndex(jobDataQueue, i)
			} else {
				logger.Log.Error(err)
				return
			}
		}
	} else {
		if utils.CONFIG.Misc.Debug {
			logger.Log.Error("jobQueueConnection is nil.")
		}
	}
}

func RemoveJobIndex(s []Datagram, index int) []Datagram {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}
