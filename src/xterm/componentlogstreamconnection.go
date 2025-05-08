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

	data, err := valkeyclient.LastNEntryFromBucketWithType[logging.LogLine](store, 50, "logs", component)
	if err != nil {
		xtermLogger.Error("Error getting last 50 logs", "error", err)
	}
	for _, v := range data {
		messageStr := fmt.Sprintf("[%s] %s %s", v.Level, utils.FormatJsonTimePrettyFromTime(v.Time), v.Message)
		connWriteLock.Lock()
		err = conn.WriteMessage(websocket.TextMessage, []byte(messageStr))
		if err != nil {
			xtermLogger.Error("WriteMessage", "error", err)
		}
		connWriteLock.Unlock()
	}

	if len(data) == 0 {
		connWriteLock.Lock()
		err = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("[INFO] %s No recent log entries found.\n", utils.FormatJsonTimePrettyFromTime(time.Now()))))
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
			messageSt := fmt.Sprintf("[%s] %s %s", entry.Level, utils.FormatJsonTimePrettyFromTime(entry.Time), entry.Message)

			connWriteLock.Lock()
			err = conn.WriteMessage(websocket.TextMessage, []byte(messageSt))
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
