package structs

import (
	"time"

	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/websocket"
)

var jobDataQueue []Datagram = []Datagram{}

func ConnectToJobQueue(jobClient websocket.WebsocketClient) {
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		for range ticker.C {
			processJobNow(jobClient)
		}
	}()
}

func JobServerSendData(jobClient websocket.WebsocketClient, datagram Datagram) {
	jobDataQueue = append(jobDataQueue, datagram)
	processJobNow(jobClient)
}

func processJobNow(jobClient websocket.WebsocketClient) {
	for i := 0; i < len(jobDataQueue); i++ {
		element := jobDataQueue[i]

		err := jobClient.WriteJSON(element)
		if err == nil {
			element.DisplaySentSummary(i+1, len(jobDataQueue))
			if isSuppressed := utils.Contains(SUPPRESSED_OUTPUT_PATTERN, element.Pattern); !isSuppressed {
				structsLogger.Debug("sent summary", "payload", element.Payload)
			}
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
