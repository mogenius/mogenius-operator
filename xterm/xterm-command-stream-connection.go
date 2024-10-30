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
)

func injectContent(content io.Reader, conn *websocket.Conn) {
	// Read full content for pre-injection
	input, err := io.ReadAll(content)
	if err != nil {
		XtermLogger.Error("failed to read data", "error", err)
	}

	// Encode for security reasons and send to pseudoterminal to be executed
	// Use pty as a bridge for correct formatting
	encodedData := base64.StdEncoding.EncodeToString(input)
	bash := exec.Command("bash", "-c", "echo \""+encodedData+"\" | base64 -d")
	ttytmp, err := pty.Start(bash)
	if err != nil {
		XtermLogger.Error("Unable to start tmp pty/cmd", "error", err)
		if conn != nil {
			err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			if err != nil {
				XtermLogger.Error("WriteMessage", "error", err)
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

			XtermLogger.Error("WriteMessage", "error", err)
			break
		}
		if conn != nil {
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				XtermLogger.Error("WriteMessage", "error", err)
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
		XtermLogger.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		XtermLogger.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30*time.Minute))
	// websocket connection
	readMessages, conn, err := generateWsConnection(cmdType, namespace, controller, podName, container, websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		XtermLogger.Error("Unable to connect to websocket", "error", err)
		return
	}

	defer func() {
		XtermLogger.Debug("[XTermCommandStreamConnection] Closing connection.")
		cancel()
	}()

	// Check if pod exists
	podExists := punq.PodExists(namespace, podName, nil)
	if !podExists.PodExists {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "POD_DOES_NOT_EXIST")
			if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
				XtermLogger.Debug("write close:", "error", err)
			}
		}
		XtermLogger.Error("Pod does not exist, closing connection.", "podName", podName)
		return
	}

	// kube provider
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		XtermLogger.Warn("Unable to create kube provider", "error", err)
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
		XtermLogger.Error("Unable to send ping", "error", err)
		return
	}

	// Start pty/cmd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	tty, err := pty.Start(cmd)
	if err != nil {
		XtermLogger.Error("Unable to start pty/cmd", "error", err)
		if conn != nil {
			err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
			if err != nil {
				XtermLogger.Error("WriteMessage", "error", err)
			}
		}
		return
	}

	defer func() {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER")
			if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
				XtermLogger.Debug("write close:", "error", err)
			}
		}
		err := cmd.Process.Kill()
		if err != nil {
			XtermLogger.Error("failed to kill process", "error", err)
		}
		_, err = cmd.Process.Wait()
		if err != nil {
			XtermLogger.Error("failed to wait for process", "error", err)
		}
		err = tty.Close()
		if err != nil {
			XtermLogger.Error("failed to close tty", "error", err)
		}
	}()

	// send cmd wait
	go cmdWait(cmd, conn, tty)

	// cmd output to websocket
	go cmdOutputToWebsocket(ctx, cancel, conn, tty, injectPreContent)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
