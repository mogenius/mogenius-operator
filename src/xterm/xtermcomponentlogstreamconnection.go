package xterm

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

func XTermComponentStreamConnection(
	wsConnectionRequest WsConnectionRequest,
	component string,
	namespace *string,
	controllerName *string,
	release *string,
) {
	cmdType := "log"

	filename, err := logManager.ComponentLogPath(string(component))
	if err != nil {
		xtermLogger.Error("couldnt get component logfile path", "component", component, "error", err)
		return
	}
	if _, err := os.Stat(filename); err != nil {
		xtermLogger.Error("component logfile does not exist", "component", component, "path", filename)
		return
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("tail -F -n %s %s", MAX_TAIL_LINES, filename))

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
	readMessages, conn, connWriteLock, _, err := generateWsConnection(cmdType, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
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

	// Start pty/cmd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	tty, err := pty.Start(cmd)
	if err != nil {
		xtermLogger.Error("Unable to start pty/cmd", "error", err)
		if conn != nil {
			connWriteLock.Lock()
			err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			connWriteLock.Unlock()
			if err != nil {
				xtermLogger.Error("WriteMessage", "error", err)
			}
		}
		return
	}

	defer closeConnection(conn, connWriteLock, cmd, tty)

	// send cmd wait
	go cmdWait(cmd, conn, connWriteLock, tty)

	// cmd output to websocket
	go cmdOutputScannerToWebsocket(ctx, cancel, conn, connWriteLock, tty, nil, component, namespace, controllerName, release)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
