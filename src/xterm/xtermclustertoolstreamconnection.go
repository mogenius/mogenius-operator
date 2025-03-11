package xterm

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"

	"github.com/gorilla/websocket"
)

func XTermClusterToolStreamConnection(wsConnectionRequest WsConnectionRequest, cmdType string, tool string) {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(600))
	// websocket connection
	readMessages, conn, connWriteLock, _, err := GenerateWsConnection(cmdType, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		xtermLogger.Error("Unable to connect to websocket", "error", err)
		return
	}

	defer func() {
		// XtermLogger.Info("[XTermClusterToolStreamConnection] Closing connection.")
		cancel()
	}()

	// Start pty/cmd
	xtermLogger.Info(tool)
	cmd := exec.Command("sh", "-c", tool)
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

	go cmdWait(cmd, conn, connWriteLock, tty)

	// cmd output to websocket
	go cmdOutputToWebsocket(ctx, cancel, conn, connWriteLock, tty, nil)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
