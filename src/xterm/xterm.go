package xterm

import (
	"context"
	"io"
	"log/slog"
	"mogenius-k8s-manager/src/logging"
	mirrorStore "mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	json "github.com/json-iterator/go"

	"github.com/creack/pty"

	v1 "k8s.io/api/core/v1"

	"github.com/gorilla/websocket"
)

var xtermLogger *slog.Logger
var store valkeyclient.ValkeyClient

func Setup(logManagerModule logging.SlogManager, storeModule valkeyclient.ValkeyClient) {
	xtermLogger = logManagerModule.CreateLogger("xterm")
	store = storeModule
}

const (
	MAX_TAIL_LINES = "100000"
)

type WsConnectionRequest struct {
	ChannelId       string `json:"channelId" validate:"required"`
	WebsocketScheme string `json:"websocketScheme" validate:"required"`
	WebsocketHost   string `json:"websocketHost" validate:"required"`
	NodeName        string `json:"nodeName"`
	CmdType         string `json:"cmdType"`
	PodName         string `json:"podName"`
	Workspace       string `json:"workspace"`
}

type PodCmdConnectionRequest struct {
	Namespace    string              `json:"namespace" validate:"required"`
	Controller   string              `json:"controller" validate:"required"`
	Pod          string              `json:"pod" validate:"required"`
	Container    string              `json:"container" validate:"required"`
	WsConnection WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
	LogTail      string              `json:"logTail"`
}

type ComponentLogConnectionRequest struct {
	WsConnection WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
	Component    string              `json:"component" validate:"required"`
	Namespace    *string             `json:"namespace,omitempty"`
	Controller   *string             `json:"controller,omitempty"`
	Release      *string             `json:"release,omitempty"`
}

type PodEventConnectionRequest struct {
	Namespace    string              `json:"namespace" validate:"required"`
	Controller   string              `json:"controller" validate:"required"`
	WsConnection WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
}

type CmdWindowSize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

type XtermReadMessages struct {
	MessageType int
	Data        []byte
	Err         error
}

var LogChannels = make(map[string]chan string)

func isPodAvailable(pod *v1.Pod, container string) bool {
	if pod.Status.InitContainerStatuses != nil {
		for _, cs := range pod.Status.InitContainerStatuses {
			if cs.Name != container {
				continue
			}
			if *cs.Started {
				return true
			}
		}
	}

	if pod.Status.ContainerStatuses != nil {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != container {
				continue
			}
			if *cs.Started {
				return true
			}
		}
	}

	switch pod.Status.Phase {
	case v1.PodRunning, v1.PodSucceeded, v1.PodFailed:
		return true
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func checkPodIsReady(ctx context.Context, namespace string, podName string, container string, conn *websocket.Conn, connWriteLock *sync.Mutex) {
	firstCount := false
	for {
		select {
		case <-ctx.Done():
			xtermLogger.Error("Context done.")
			return
		default:
			// refresh cache
			pod := mirrorStore.GetPod(namespace, podName)
			if pod == nil {
				xtermLogger.Error("Unable to find pod", "error", "pod not found", "namespace", namespace, "podName", podName)
				if conn != nil {
					// clear screen
					clearScreen(conn, connWriteLock)
					closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "POD_DOES_NOT_EXIST")
					connWriteLock.Lock()
					err := conn.WriteMessage(websocket.CloseMessage, closeMsg)
					connWriteLock.Unlock()
					if err != nil {
						xtermLogger.Debug("write close:", "error", err)
					}
				}
				return
			}

			if isPodAvailable(pod, container) {
				xtermLogger.Debug("Pod is ready", "podName", pod.Name)
				// clear screen
				clearScreen(conn, connWriteLock)
				return
			} else {
				// XtermLogger.Info("Pod is not ready, waiting.")
				msg := "."
				if !firstCount {
					firstCount = true
					msg = "Pod is not ready, waiting."
				}
				connWriteLock.Lock()
				err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
				connWriteLock.Unlock()
				if err != nil {
					xtermLogger.Error("WriteMessage", "error", err)
					ctx.Done()
					return
				}
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func clearScreen(conn *websocket.Conn, connWriteLock *sync.Mutex) {
	// clear screen
	connWriteLock.Lock()
	err := conn.WriteMessage(websocket.BinaryMessage, []byte("\u001b[2J\u001b[H"))
	connWriteLock.Unlock()
	if err != nil {
		xtermLogger.Error("WriteMessage", "error", err)
	}
}

func GenerateWsConnection(
	cmdType string,
	namespace string,
	controller string,
	podName string,
	container string,
	u url.URL,
	wsConnectionRequest WsConnectionRequest,
	ctx context.Context,
	cancel context.CancelFunc,
) (readMessages *chan XtermReadMessages, conn *websocket.Conn, connWriteLock *sync.Mutex, connReadLock *sync.Mutex, err error) {
	maxRetries := 6
	currentRetries := 0
	xtermMessages := make(chan XtermReadMessages)

	for {
		// add header
		headers := utils.HttpHeader("")
		headers.Add("x-channel-id", wsConnectionRequest.ChannelId)
		headers.Add("x-cmd", cmdType)
		headers.Add("x-namespace", namespace)
		headers.Add("x-controller", controller)
		headers.Add("x-pod-name", podName)
		headers.Add("x-container", container)
		headers.Add("x-type", "k8s")

		dialer := &websocket.Dialer{}
		conn, _, err := dialer.Dial(u.String(), headers)
		connWriteLock := &sync.Mutex{}
		connReadLock := &sync.Mutex{}
		if err != nil {
			xtermLogger.Error("failed to connect, retrying in 5 seconds", "error", err.Error())
			if currentRetries >= maxRetries {
				xtermLogger.Error("Max retries reached, exiting.")
				return nil, nil, nil, nil, err
			}
			time.Sleep(5 * time.Second)
			currentRetries++
			continue
		}

		// XtermLogger.Infof("Connected to %s", u.String())

		// API send ack when it is ready to receive messages.
		err = conn.SetReadDeadline(time.Now().Add(30 * time.Minute))
		if err != nil {
			xtermLogger.Error("failed to set read deadline", "error", err)
		}
		connReadLock.Lock()
		_, _, err = conn.ReadMessage()
		connReadLock.Unlock()
		if err != nil {
			xtermLogger.Error("failed to receive ack-ready, retrying in 5 seconds", "error", err)
			time.Sleep(5 * time.Second)
			if currentRetries >= maxRetries {
				xtermLogger.Error("Max retries reached, exiting.")
				return &xtermMessages, conn, connWriteLock, connReadLock, err
			}
			currentRetries++
			continue
		}

		// XtermLogger.Infof("Ready ack from connected stream endpoint: %s.", string(ack))

		// oncloseWs will close the connection and the context
		go oncloseWs(conn, connReadLock, ctx, cancel, xtermMessages)
		return &xtermMessages, conn, connWriteLock, connReadLock, nil
	}
}

func oncloseWs(conn *websocket.Conn, connReadLock *sync.Mutex, ctx context.Context, cancel context.CancelFunc, readMessages chan XtermReadMessages) {
	defer func() {
		cancel()
		if conn != nil {
			conn.Close()
		}
		if readMessages != nil {
			close(readMessages)
		}
		// XtermLogger.Info("[oncloseWs] Context done. Closing connection.")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			connReadLock.Lock()
			messageType, p, err := conn.ReadMessage()
			connReadLock.Unlock()
			if readMessages != nil {
				readMessages <- XtermReadMessages{MessageType: messageType, Data: p, Err: err}
			}
			if err != nil {
				if closeErr, ok := err.(*websocket.CloseError); ok {
					xtermLogger.Debug("[oncloseWs] WebSocket closed", "statusCode", closeErr.Code, "closeErr", closeErr.Text)
				} else {
					xtermLogger.Debug("[oncloseWs] Failed to read message. Connection closed.", "error", err)
				}
				return
			}
		}
	}
}

func wsPing(conn *websocket.Conn) error {
	err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second))
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
			xtermLogger.Info("The connection was closed")
			return err
		}
		xtermLogger.Info("failed to send ping", "error", err)
		return err
	}
	return nil
}

func cmdWait(cmd *exec.Cmd, conn *websocket.Conn, connWriteLock *sync.Mutex, tty *os.File) {
	err := cmd.Wait()
	if err != nil {
		// XtermLogger.Errorf("cmd wait: %s", err.Error())
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 137 {
					if conn != nil {
						connWriteLock.Lock()
						err := conn.WriteMessage(websocket.TextMessage, []byte("POD_DOES_NOT_EXIST"))
						connWriteLock.Unlock()
						if err != nil {
							xtermLogger.Error("WriteMessage", "error", err)
						}
					}
				}
			}
		}
	} else {
		closeConnection(conn, connWriteLock, cmd, tty)
	}
}

func cmdOutputToWebsocket(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, connWriteLock *sync.Mutex, tty *os.File, injectPreContent io.Reader) {
	if injectPreContent != nil {
		injectContent(injectPreContent, conn, connWriteLock)
	}

	defer func() {
		cancel()
		// XtermLogger.Info("[cmdOutputToWebsocket] Closing connection.")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf := make([]byte, 1024)
			read, err := tty.Read(buf)
			if err != nil {
				// XtermLogger.Errorf("Unable to read from pty/cmd: %s", err.Error())
				return
			}
			if conn != nil {
				connWriteLock.Lock()
				err := conn.WriteMessage(websocket.BinaryMessage, buf[:read])
				connWriteLock.Unlock()
				if err != nil {
					xtermLogger.Error("WriteMessage", "error", err)
				}
				continue
			}
			return
		}
	}
}

func websocketToCmdInput(readMessages <-chan XtermReadMessages, ctx context.Context, tty *os.File, cmdType *string) {
	for msg := range readMessages {
		select {
		case <-ctx.Done():
			return
		default:
			if msg.Err != nil {
				// log.Errorf("Unable to read from websocket: %s", msg.Err.Error())
				return
			}
			msgStr := string(msg.Data)

			if msgStr == "PEER_IS_READY" {
				continue
			}

			if tty != nil {
				if strings.HasPrefix(msgStr, "\x04") {
					str := strings.TrimPrefix(msgStr, "\x04")

					colsExists := strings.Contains(str, "\"cols\":")
					rowsExists := strings.Contains(str, "\"rows\":")

					if colsExists && rowsExists {
						var resizeMessage CmdWindowSize
						err := json.Unmarshal([]byte(str), &resizeMessage)
						if err == nil {
							if err := pty.Setsize(tty, &pty.Winsize{Rows: uint16(resizeMessage.Rows), Cols: uint16(resizeMessage.Cols)}); err != nil {
								xtermLogger.Error("Unable to resize", "error", err)
								continue
							}
							continue
						}
					}
				}

				if cmdType != nil {
					if *cmdType == "exec-sh" {
						_, err := tty.Write(msg.Data)
						if err != nil {
							xtermLogger.Error("failed to write in tty context", "error", err)
						}
					}
				}
			}
		}
	}
}

func closeConnection(conn *websocket.Conn, connWriteLock *sync.Mutex, cmd *exec.Cmd, tty *os.File) {
	if conn != nil {
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER")
		connWriteLock.Lock()
		err := conn.WriteMessage(websocket.CloseMessage, closeMsg)
		connWriteLock.Unlock()
		if err != nil {
			xtermLogger.Debug("write close:", "error", err)
		}
	}
	err := cmd.Process.Kill()
	if err != nil && !strings.Contains(err.Error(), "process already finished") {
		xtermLogger.Error("failed to kill process", "error", err)
	}
	_, err = cmd.Process.Wait()
	if err != nil && !strings.Contains(err.Error(), "no child processes") {
		xtermLogger.Error("failed to wait for process", "error", err)
	}
	err = tty.Close()
	if err != nil && !strings.Contains(err.Error(), "file already closed") {
		xtermLogger.Error("failed to close tty", "error", err)
	}
}
