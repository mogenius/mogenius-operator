package xterm

import (
	"context"
	"mogenius-k8s-manager/src/utils"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
	// websocket connection
	readMessages, conn, err := generateWsConnection(cmdType, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		xtermLogger.Error("Unable to connect to websocket", "error", err)
		return
	}

	defer func() {
		// XtermLogger.Info("[XTermClusterToolStreamConnection] Closing connection.")
		cancel()
	}()

	cmdString := ""
	switch tool {
	case "k9s":
		cmdString = "k9s --kubeconfig kubeconfig.yaml"
	default:
		xtermLogger.Error("Tool not found", "tool", tool)
		return
	}

	// Start pty/cmd
	xtermLogger.Info(cmdString)
	cmd := exec.Command("sh", "-c", cmdString)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	tty, err := pty.Start(cmd)
	if err != nil {
		xtermLogger.Error("Unable to start pty/cmd", "error", err)
		if conn != nil {
			err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			if err != nil {
				xtermLogger.Error("WriteMessage", "error", err)
			}
		}
		return
	}

	defer func() {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER")
			if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
				xtermLogger.Debug("failed to write message", "error", err)
			}
		}
		err := cmd.Process.Kill()
		if err != nil {
			xtermLogger.Error("failed to kill process", "error", err)
		}
		_, err = cmd.Process.Wait()
		if err != nil {
			xtermLogger.Error("failed to wait for process", "error", err)
		}
		err = tty.Close()
		if err != nil {
			xtermLogger.Error("failed to close tty", "error", err)
		}
	}()

	go cmdWait(cmd, conn, tty)

	// cmd output to websocket
	go cmdOutputToWebsocket(ctx, cancel, conn, tty, nil)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
