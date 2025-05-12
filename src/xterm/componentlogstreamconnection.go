package xterm

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/logging"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func ComponentStreamConnection(
	wsConnectionRequest WsConnectionRequest,
	component string,
	namespace *string,
	controllerName *string,
	release *string,
) {
	pubsub := store.SubscribeToBucket("logs", component)
	defer pubsub.Close()

	if wsConnectionRequest.WebsocketScheme == "" {
		xtermLogger.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		xtermLogger.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30*time.Minute))
	// websocket connection
	_, conn, connWriteLock, _, err := GenerateWsConnection("log", "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		xtermLogger.Error("Unable to connect to websocket", "error", err)
		return
	}

	defer func() {
		cancel()
	}()

	// send ping
	err = wsPing(conn)
	if err != nil {
		xtermLogger.Error("Unable to send ping", "error", err)
		return
	}

	data, err := valkeyclient.LastNEntryFromBucketWithType[logging.LogLine](store, 100, "logs", component)
	if err != nil {
		xtermLogger.Error("Error getting last 50 logs", "error", err)
	}

	logEntriesWritten := false
	for _, v := range data {
		messageStr := processLogLine(component, namespace, release, v)
		if messageStr == "" {
			continue
		}

		connWriteLock.Lock()
		err = conn.WriteMessage(websocket.TextMessage, []byte(messageStr))
		logEntriesWritten = true
		if err != nil {
			xtermLogger.Error("WriteMessage", "error", err)
		}
		connWriteLock.Unlock()
	}

	if !logEntriesWritten {
		connWriteLock.Lock()
		if component == "helm" {
			err = conn.WriteMessage(websocket.TextMessage, []byte("📝 No Log Entries Found\n🔍 This may occur due to the decentralized nature of Helm.\nIf the Helm chart was applied from a different machine, logs might not be available here.\n"))
		} else {
			err = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("[INFO] %s No recent log entries found.\n", utils.FormatJsonTimePrettyFromTime(time.Now()))))
		}
		if err != nil {
			xtermLogger.Error("WriteMessage", "error", err)
		}
		connWriteLock.Unlock()
	}

	for msg := range pubsub.Channel() {
		if conn != nil {
			var entry logging.LogLine
			err := json.Unmarshal([]byte(msg.Payload), &entry)
			if err != nil {
				xtermLogger.Error("Unmarshal", "error", err)
				continue
			}
			messageStr := processLogLine(component, namespace, release, entry)
			if messageStr == "" {
				continue
			}

			connWriteLock.Lock()
			err = conn.WriteMessage(websocket.TextMessage, []byte(messageStr))
			connWriteLock.Unlock()
			if err != nil {
				if strings.Contains(err.Error(), "broken pipe") {
					xtermLogger.Debug("write close:", "error", err)
					break
				}
				xtermLogger.Error("WriteMessage", "error", err)
			}
		}
	}

	if conn != nil {
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER")
		connWriteLock.Lock()
		err := conn.WriteMessage(websocket.CloseMessage, closeMsg)
		connWriteLock.Unlock()
		if err != nil {
			xtermLogger.Debug("write close:", "error", err)
		}
	}
}

func processLogLine(component string, namespace *string, release *string, line logging.LogLine) string {
	if line.Level == "debug" {
		return ""
	}

	messageStr := fmt.Sprintf("[%s] %s %s", line.Level, utils.FormatJsonTimePrettyFromTime(line.Time), line.Message)

	if component == "helm" {
		givenNs := ""
		if namespace != nil {
			givenNs = *namespace
		}
		givenRelease := ""
		if release != nil {
			givenRelease = *release
		}
		gatheredNs, _ := line.Payload["namespace"].(string)
		gatheredRelease, _ := line.Payload["releaseName"].(string)

		if gatheredNs == givenNs && gatheredRelease == givenRelease {
			if line.Payload["error"] != nil {
				return fmt.Sprintf("[%s] %s %s %s\n", line.Level, utils.FormatJsonTimePrettyFromTime(line.Time), line.Message, line.Payload["error"])
			} else {
				return fmt.Sprintf("[%s] %s %s\n", line.Level, utils.FormatJsonTimePrettyFromTime(line.Time), line.Message)
			}
		} else {
			return ""
		}
	}

	return messageStr
}
