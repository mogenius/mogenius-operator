package xterm

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

func XTermComponentStreamConnection(
	wsConnectionRequest WsConnectionRequest,
	component structs.ComponentEnum,
	namespace *string,
	controllerName *string,
	release *string,
) {
	cmdType := "log"

	filename := logManager.CombinedLogPath()
	if component != structs.ComponentAll {
		filename = fmt.Sprintf("%s/%s.log", utils.CONFIG.Kubernetes.LogDataPath, component)
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf("tail -F -n %s %s", MAX_TAIL_LINES, filename))

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
	readMessages, conn, err := generateWsConnection(cmdType, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
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
				xtermLogger.Debug("write close:", "error", err)
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

	// send cmd wait
	go cmdWait(cmd, conn, tty)

	// cmd output to websocket
	go cmdOutputScannerToWebsocket(ctx, cancel, conn, tty, nil, component, namespace, controllerName, release)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
