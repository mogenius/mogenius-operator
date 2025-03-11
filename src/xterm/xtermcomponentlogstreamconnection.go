package xterm

import (
	"context"
	"encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/utils"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func XTermComponentStreamConnection(
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

	for msg := range pubsub.Channel() {
		if conn != nil {
			connWriteLock.Lock()
			var entry LogEntry
			err := json.Unmarshal([]byte(msg.Payload), &entry)
			if err != nil {
				xtermLogger.Error("Unmarshal", "error", err)
				continue
			}
			if !(strings.HasSuffix(entry.Message, "\n") || strings.HasSuffix(entry.Message, "\n\r")) {
				entry.Message = entry.Message + "\n"
			}
			messageSt := fmt.Sprintf("[%s] %s %s", entry.Level, utils.FormatJsonTimePretty(entry.Time), entry.Message)

			fmt.Println("msg", messageSt)
			err = conn.WriteMessage(websocket.TextMessage, []byte(messageSt))
			connWriteLock.Unlock()
			if err != nil {
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
