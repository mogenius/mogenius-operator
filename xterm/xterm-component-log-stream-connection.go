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
	log "github.com/sirupsen/logrus"
)

func XTermComponentStreamConnection(
	wsConnectionRequest WsConnectionRequest,
	component structs.ComponentEnum,
) {
	cmdType := "log"

	cmd := exec.Command("bash", "-c", fmt.Sprintf("tail -F -n %s %s", MAX_TAIL_LINES, utils.MainLogPath()))

	if wsConnectionRequest.WebsocketScheme == "" {
		log.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		log.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30*time.Minute))
	// websocket connection
	readMessages, conn, err := generateWsConnection(cmdType, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}

	defer func() {
		cancel()
	}()

	// send ping
	err = wsPing(conn)
	if err != nil {
		log.Errorf("Unable to send ping: %s", err.Error())
		return
	}

	// Start pty/cmd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	tty, err := pty.Start(cmd)
	if err != nil {
		log.Errorf("Unable to start pty/cmd: %s", err.Error())
		if conn != nil {
			err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			if err != nil {
				log.Errorf("WriteMessage: %s", err.Error())
			}
		}
		return
	}

	defer func() {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER")
			if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
				// log.Error("write close:", err)
			}
		}
		cmd.Process.Kill()
		cmd.Process.Wait()
		tty.Close()
	}()

	// send cmd wait
	go cmdWait(cmd, conn, tty)

	// cmd output to websocket
	go cmdOutputScannerToWebsocket(ctx, cancel, conn, tty, nil, component)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
