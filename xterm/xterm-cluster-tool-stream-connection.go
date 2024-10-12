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
	log "github.com/sirupsen/logrus"
)

func XTermClusterToolStreamConnection(wsConnectionRequest WsConnectionRequest, cmdType string, tool string) {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
	// websocket connection
	readMessages, conn, err := generateWsConnection(cmdType, "", "", "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}

	defer func() {
		// log.Info("[XTermClusterToolStreamConnection] Closing connection.")
		cancel()
	}()

	cmdString := ""
	switch tool {
	case "k9s":
		cmdString = "k9s --kubeconfig kubeconfig.yaml"
	default:
		log.Errorf("Tool not found: %s", tool)
		return
	}

	// Start pty/cmd
	log.Info(cmdString)
	cmd := exec.Command("sh", "-c", cmdString)
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
				log.Debug(err)
			}
		}
		err := cmd.Process.Kill()
		if err != nil {
			log.Error(err)
		}
		_, err = cmd.Process.Wait()
		if err != nil {
			log.Error(err)
		}
		err = tty.Close()
		if err != nil {
			log.Error(err)
		}
	}()

	go cmdWait(cmd, conn, tty)

	// cmd output to websocket
	go cmdOutputToWebsocket(ctx, cancel, conn, tty, nil)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
