package xterm

import (
	"context"
	"mogenius-k8s-manager/utils"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"

	"github.com/gorilla/websocket"
)

func XTermClusterToolStreamConnection(wsConnectionRequest WsConnectionRequest, cmdType string, tool string) {
	if wsConnectionRequest.WebsocketScheme == "" {
		XtermLogger.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		XtermLogger.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
	// websocket connection
	readMessages, conn, err := generateWsConnection(cmdType, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		XtermLogger.Errorf("Unable to connect to websocket: %s", err.Error())
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
		XtermLogger.Errorf("Tool not found: %s", tool)
		return
	}

	// Start pty/cmd
	XtermLogger.Info(cmdString)
	cmd := exec.Command("sh", "-c", cmdString)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	tty, err := pty.Start(cmd)
	if err != nil {
		XtermLogger.Errorf("Unable to start pty/cmd: %s", err.Error())
		if conn != nil {
			err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			if err != nil {
				XtermLogger.Errorf("WriteMessage: %s", err.Error())
			}
		}
		return
	}

	defer func() {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER")
			if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
				XtermLogger.Debug(err)
			}
		}
		err := cmd.Process.Kill()
		if err != nil {
			XtermLogger.Error(err)
		}
		_, err = cmd.Process.Wait()
		if err != nil {
			XtermLogger.Error(err)
		}
		err = tty.Close()
		if err != nil {
			XtermLogger.Error(err)
		}
	}()

	go cmdWait(cmd, conn, tty)

	// cmd output to websocket
	go cmdOutputToWebsocket(ctx, cancel, conn, tty, nil)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
