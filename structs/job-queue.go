package structs

import (
	"context"
	"mogenius-k8s-manager/utils"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	punqUtils "github.com/mogenius/punq/utils"
)

var jobDataQueue []Datagram = []Datagram{}

// Public
var JobSendMutex sync.Mutex
var JobQueueConnection *websocket.Conn
var JobConnectionGuard = make(chan struct{}, 1)
var JobConnectionStatus chan bool = make(chan bool)
var JobConnectionUrl url.URL = url.URL{}

func ConnectToJobQueue() {
	for {
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

		<-time.After(RETRYTIMEOUT * time.Second)
	}
}

func connectJob(ctx context.Context) {
	JobConnectionUrl = url.URL{Scheme: utils.CONFIG.ApiServer.Ws_Scheme, Host: utils.CONFIG.ApiServer.Ws_Server, Path: utils.CONFIG.ApiServer.WS_Path}

	connection, _, err := websocket.DefaultDialer.Dial(JobConnectionUrl.String(), utils.HttpHeader(""))
	if err != nil {
		structsLogger.Error("Connection to JobServer failed", "url", JobConnectionUrl.String(), "error", err)
		JobConnectionStatus <- false
	} else {
		structsLogger.Info("Connected to JobServer", "url", JobConnectionUrl.String(), "localAddr", connection.LocalAddr().String())
		JobQueueConnection = connection
		JobConnectionStatus <- true
		err := Ping(JobQueueConnection, &JobSendMutex)
		if err != nil {
			structsLogger.Error("Error pinging job queue", "error", err)
		}
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
				element.DisplaySentSummary(i+1, len(jobDataQueue))
				if isSuppressed := punqUtils.Contains(SUPPRESSED_OUTPUT_PATTERN, element.Pattern); !isSuppressed {
					structsLogger.Debug("sent summary", "payload", element.Payload)
				}
				jobDataQueue = removeJobIndex(jobDataQueue, i)
			} else {
				structsLogger.Error("Error writing json in job queue", "error", err)
				return
			}
		}
	} else {
		structsLogger.Debug("jobQueueConnection is nil.")
	}
}

func removeJobIndex(s []Datagram, index int) []Datagram {
	if len(s) > index {
		return append(s[:index], s[index+1:]...)
	}
	return s
}
