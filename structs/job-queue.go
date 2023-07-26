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
var JobConnectionGuard = make(chan struct{}, 1)
var JobConnectionStatus chan bool = make(chan bool)
var JobConnectionUrl url.URL = url.URL{}

func ConnectToJobQueue() {
	interrupt := make(chan os.Signal, 1)
	defer close(interrupt)
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
			quit := make(chan struct{})

			go func() {
				for {
					select {
					case <-ticker.C:
						processJobNow()
					case <-quit:
						// close go routine
						return
					}
				}
			}()

			ctx := context.Background()
			connectJob(ctx)
			ctx.Done()
			<-JobConnectionGuard

			ticker.Stop()
			close(quit)
		}()
	}
}

func connectJob(ctx context.Context) {
	scheme := "wss"
	if utils.CONFIG.Misc.Stage == "local" {
		scheme = "ws"
	}
	JobConnectionUrl = url.URL{Scheme: scheme, Host: utils.CONFIG.ApiServer.Ws_Server, Path: utils.CONFIG.ApiServer.WS_Path}

	connection, _, err := websocket.DefaultDialer.Dial(JobConnectionUrl.String(), utils.HttpHeader(""))
	if err != nil {
		logger.Log.Errorf("Connection to JobServer failed (%s): %s\n", JobConnectionUrl.String(), err.Error())
		JobConnectionStatus <- false
	} else {
		logger.Log.Infof("Connected to JobServer: %s  (%s)\n", JobConnectionUrl.String(), connection.LocalAddr().String())
		JobQueueConnection = connection
		JobConnectionStatus <- true
		Ping(JobQueueConnection, &JobSendMutex)
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
