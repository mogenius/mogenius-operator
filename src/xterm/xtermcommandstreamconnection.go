package xterm

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mogenius-k8s-manager/src/kubernetes"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"k8s.io/client-go/rest"
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
				xtermLogger.Error("failed to write websocket message", "error", err)
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
			xtermLogger.Error("failed to read from pseudoterminal", "error", err)
			break
		}
		if conn != nil {
			connWriteLock.Lock()
			err := conn.WriteMessage(websocket.BinaryMessage, buf[:n])
			connWriteLock.Unlock()
			if err != nil {
				xtermLogger.Error("failed to write websocket message", "error", err)
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
	readMessages, conn, connWriteLock, _, err := GenerateWsConnection(cmdType, namespace, controller, podName, container, websocketUrl, wsConnectionRequest, ctx, cancel)
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

	// check if pod is ready
	checkPodIsReady(ctx, namespace, podName, container, conn, connWriteLock)

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

func GetPreviousLogContent(podCmdConnectionRequest PodCmdConnectionRequest) io.Reader {
	ctx := context.Background()
	cancelCtx, endGofunc := context.WithCancel(ctx)
	defer endGofunc()

	pod := kubernetes.PodStatus(podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod, false)
	terminatedState := kubernetes.LastTerminatedStateIfAny(pod)

	var previousRestReq *rest.Request
	if terminatedState != nil {
		tmpPreviousResReq, err := kubernetes.StreamPreviousLog(podCmdConnectionRequest.Namespace, podCmdConnectionRequest.Pod)
		if err != nil {
			xtermLogger.Error(err.Error())
		} else {
			previousRestReq = tmpPreviousResReq
		}
	}

	if previousRestReq == nil {
		return nil
	}

	var previousStream io.ReadCloser
	tmpPreviousStream, err := previousRestReq.Stream(cancelCtx)
	if err != nil {
		xtermLogger.Error(err.Error())
		previousStream = io.NopCloser(strings.NewReader(fmt.Sprintln(err.Error())))
	} else {
		previousStream = tmpPreviousStream
	}

	data, err := io.ReadAll(previousStream)
	if err != nil {
		xtermLogger.Error("failed to read data", "error", err)
	}

	lastState := kubernetes.LastTerminatedStateToString(terminatedState)

	nl := strings.NewReader("\r\n")
	previousState := strings.NewReader(lastState)
	headlineLastLog := strings.NewReader("Last Log:\r\n")
	headlineCurrentLog := strings.NewReader("\r\nCurrent Log:\r\n")

	return io.MultiReader(previousState, nl, headlineLastLog, strings.NewReader(string(data)), nl, headlineCurrentLog)
}
