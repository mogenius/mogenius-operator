package xterm

import (
	"context"
	"encoding/base64"
	"io"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	punq "github.com/mogenius/punq/kubernetes"
	log "github.com/sirupsen/logrus"
)

func injectContent(content io.Reader, conn *websocket.Conn) {
	// Read full content for pre-injection
	input, err := io.ReadAll(content)
	if err != nil {
		log.Errorf("failed to read data: %v", err)
	}

	// Encode for security reasons and send to pseudoterminal to be executed
	// Use pty as a bridge for correct formatting
	encodedData := base64.StdEncoding.EncodeToString(input)
	bash := exec.Command("bash", "-c", "echo \""+encodedData+"\" | base64 -d")
	ttytmp, err := pty.Start(bash)
	if err != nil {
		log.Errorf("Unable to start tmp pty/cmd: %s", err.Error())
		if conn != nil {
			err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			if err != nil {
				log.Errorf("WriteMessage: %s", err.Error())
			}
		}
		return
	}
	defer func() { _ = ttytmp.Close() }()

	// Read from pseudoterminal and send to websocket
	buf := make([]byte, 1024)
	for {
		n, err := ttytmp.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Errorf("WriteMessage: %s", err.Error())
			break
		}
		if conn != nil {
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				log.Errorf("WriteMessage: %s", err.Error())
				break
			}
		} else {
			break
		}
	}
}

func XTermCommandStreamConnection(
	cmdType string,
	wsConnectionRequest WsConnectionRequest,
	namespace string,
	controller string,
	podName string,
	container string,
	cmd *exec.Cmd,
	injectPreContent io.Reader,
) {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(5*time.Second))
	// websocket connection
	readMessages, conn, err := generateWsConnection(cmdType, namespace, controller, podName, container, websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}

	defer func() {
		// log.Info("[XTermCommandStreamConnection] Closing connection.")
		cancel()
	}()

	// Check if pod exists
	podExists := punq.PodExists(namespace, podName, nil)
	if !podExists.PodExists {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "POD_DOES_NOT_EXIST")
			if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
				// log.Error("write close:", err)
			}
		}
		log.Errorf("Pod %s does not exist, closing connection.", podName)
		return
	}

	// kube provider
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		log.Warningf("Unable to create kube provider: %s", err.Error())
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	// check if pod is ready
	go checkPodIsReady(ctx, &wg, provider, namespace, podName, conn)
	wg.Wait()

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
	go cmdOutputToWebsocket(ctx, cancel, conn, tty, injectPreContent)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
