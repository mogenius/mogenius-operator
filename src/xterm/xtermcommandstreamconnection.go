package xterm

import (
	"context"
	"encoding/base64"
	"io"
	"mogenius-k8s-manager/src/kubernetes"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

func injectContent(content io.Reader, conn *websocket.Conn, connWriteLock *sync.Mutex) {
	// Read full content for pre-injection
	input, err := io.ReadAll(content)
	if err != nil {
		xtermLogger.Error("failed to read data", "error", err)
	}

	// Encode for security reasons and send to pseudoterminal to be executed
	// Use pty as a bridge for correct formatting
	encodedData := base64.StdEncoding.EncodeToString(input)
	sh := exec.Command("sh", "-c", "echo \""+encodedData+"\" | base64 -d")
	ttytmp, err := pty.Start(sh)
	if err != nil {
		xtermLogger.Error("Unable to start tmp pty/cmd", "error", err)
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
	defer func() { _ = ttytmp.Close() }()

	// Read from pseudoterminal and send to websocket
	buf := make([]byte, 1024)
	for {
		n, err := ttytmp.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}

			xtermLogger.Error("WriteMessage", "error", err)
			break
		}
		if conn != nil {
			connWriteLock.Lock()
			err := conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			connWriteLock.Unlock()
			if err != nil {
				xtermLogger.Error("WriteMessage", "error", err)
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
	readMessages, conn, connWriteLock, _, err := generateWsConnection(cmdType, namespace, controller, podName, container, websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		xtermLogger.Error("Unable to connect to websocket", "error", err)
		return
	}

	defer func() {
		xtermLogger.Debug("[XTermCommandStreamConnection] Closing connection.")
		cancel()
	}()

	// Check if pod exists
	podExists := kubernetes.PodExists(namespace, podName)
	if !podExists.PodExists {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "POD_DOES_NOT_EXIST")
			connWriteLock.Lock()
			err := conn.WriteMessage(websocket.CloseMessage, closeMsg)
			connWriteLock.Unlock()
			if err != nil {
				xtermLogger.Debug("write close:", "error", err)
			}
		}
		xtermLogger.Error("Pod does not exist, closing connection.", "podName", podName)
		return
	}

	// kube provider
	var wg sync.WaitGroup
	wg.Add(1)
	// check if pod is ready
	go checkPodIsReady(ctx, &wg, namespace, podName, conn, connWriteLock)
	wg.Wait()

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
	go cmdOutputToWebsocket(ctx, cancel, conn, connWriteLock, tty, injectPreContent)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
