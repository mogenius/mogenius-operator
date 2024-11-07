package xterm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mogenius-k8s-manager/interfaces"
	"mogenius-k8s-manager/logging"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/creack/pty"

	v1 "k8s.io/api/core/v1"

	"github.com/gorilla/websocket"
)

var logManager interfaces.LogManager
var xtermLogger *slog.Logger

func Setup(interfaceLogManager interfaces.LogManager) {
	logManager = interfaceLogManager
	xtermLogger = logManager.CreateLogger("xterm")
}

const (
	MAX_TAIL_LINES = "100000"
)

type WsConnectionRequest struct {
	ChannelId       string `json:"channelId" validate:"required"`
	WebsocketScheme string `json:"websocketScheme" validate:"required"`
	WebsocketHost   string `json:"websocketHost" validate:"required"`
}

type PodCmdConnectionRequest struct {
	Namespace    string              `json:"namespace" validate:"required"`
	Controller   string              `json:"controller" validate:"required"`
	Pod          string              `json:"pod" validate:"required"`
	Container    string              `json:"container" validate:"required"`
	WsConnection WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
	LogTail      string              `json:"logTail"`
}

type BuildLogConnectionRequest struct {
	Namespace    string                  `json:"namespace" validate:"required"`
	Controller   string                  `json:"controller" validate:"required"`
	Container    string                  `json:"container" validate:"required"`
	BuildTask    structs.BuildPrefixEnum `json:"buildTask" validate:"required"` // clone, build, test, deploy, .....
	BuildId      uint64                  `json:"buildId" validate:"required"`
	WsConnection WsConnectionRequest     `json:"wsConnectionRequest" validate:"required"`
}

type OperatorLogConnectionRequest struct {
	Namespace    string              `json:"namespace" validate:"required"`
	Controller   string              `json:"controller" validate:"required"`
	WsConnection WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
	LogTail      string              `json:"logTail"`
}

type ComponentLogConnectionRequest struct {
	WsConnection WsConnectionRequest   `json:"wsConnectionRequest" validate:"required"`
	Component    structs.ComponentEnum `json:"component" validate:"required"`
	Namespace    *string               `json:"namespace,omitempty"`
	Controller   *string               `json:"controller,omitempty"`
	Release      *string               `json:"release,omitempty"`
}

type PodEventConnectionRequest struct {
	Namespace    string              `json:"namespace" validate:"required"`
	Controller   string              `json:"controller" validate:"required"`
	WsConnection WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
}

type ScanImageLogConnectionRequest struct {
	Namespace     string `json:"namespace" validate:"required"`
	Controller    string `json:"controller" validate:"required"`
	Container     string `json:"container" validate:"required"`
	CmdType       string `json:"cmdType" validate:"required"`
	ScanImageType string `json:"scanImageType" validate:"required"`

	ContainerRegistryUrl  string `json:"containerRegistryUrl"`
	ContainerRegistryUser string `json:"containerRegistryUser"`
	ContainerRegistryPat  string `json:"containerRegistryPat"`

	WsConnection WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
}

type LogEntry struct {
	ControllerName string                `json:"controllerName"`
	Level          string                `json:"level"`
	Namespace      string                `json:"namespace"`
	ReleaseName    string                `json:"releaseName"`
	Component      structs.ComponentEnum `json:"component"`
	Message        string                `json:"msg"`
	Time           string                `json:"time"`
}

func (p *ScanImageLogConnectionRequest) AddSecretsToRedaction() {
	logging.AddSecret(&p.ContainerRegistryUser)
	logging.AddSecret(&p.ContainerRegistryPat)
}

type ClusterToolConnectionRequest struct {
	CmdType string `json:"cmdType" validate:"required"`
	Tool    string `json:"tool" validate:"required"`

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

func isPodAvailable(pod *v1.Pod) bool {
	if pod.Status.Phase == v1.PodRunning {
		return true
	} else if pod.Status.Phase == v1.PodSucceeded {
		return true
	} else if pod.Status.Phase == v1.PodFailed {
		return true
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func checkPodIsReady(ctx context.Context, wg *sync.WaitGroup, provider *punq.KubeProvider, namespace string, podName string, conn *websocket.Conn) {
	defer func() {
		wg.Done()
	}()
	firstCount := false
	for {
		select {
		case <-ctx.Done():
			xtermLogger.Error("Context done.")
			return
		default:
			pod, err := provider.ClientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
			if err != nil {
				xtermLogger.Error("Unable to get pod", "error", err)
				if conn != nil {
					// clear screen
					clearScreen(conn)
					closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "POD_DOES_NOT_EXIST")
					if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
						xtermLogger.Debug("write close:", "error", err)
					}
				}
				return
			}

			if isPodAvailable(pod) {
				xtermLogger.Info("Pod is ready", "podName", pod.Name)
				// clear screen
				clearScreen(conn)
				return
			} else {
				// XtermLogger.Info("Pod is not ready, waiting.")
				msg := "."
				if !firstCount {
					firstCount = true
					msg = "Pod is not ready, waiting."
				}
				err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
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

func clearScreen(conn *websocket.Conn) {
	// clear screen
	err := conn.WriteMessage(websocket.BinaryMessage, []byte("\u001b[2J\u001b[H"))
	if err != nil {
		xtermLogger.Error("WriteMessage", "error", err)
	}
}

func generateWsConnection(
	cmdType string,
	namespace string,
	controller string,
	podName string,
	container string,
	u url.URL,
	wsConnectionRequest WsConnectionRequest,
	ctx context.Context,
	cancel context.CancelFunc,
) (readMessages *chan XtermReadMessages, conn *websocket.Conn, err error) {
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
		if err != nil {
			xtermLogger.Error("failed to connect, retrying in 5 seconds", "error", err.Error())
			if currentRetries >= maxRetries {
				xtermLogger.Error("Max retries reached, exiting.")
				return nil, nil, err
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
		_, _, err = conn.ReadMessage()
		if err != nil {
			xtermLogger.Error("failed to receive ack-ready, retrying in 5 seconds", "error", err)
			time.Sleep(5 * time.Second)
			if currentRetries >= maxRetries {
				xtermLogger.Error("Max retries reached, exiting.")
				return &xtermMessages, conn, err
			}
			currentRetries++
			continue
		}

		// XtermLogger.Infof("Ready ack from connected stream endpoint: %s.", string(ack))

		// oncloseWs will close the connection and the context
		go oncloseWs(conn, ctx, cancel, xtermMessages)
		return &xtermMessages, conn, nil
	}
}

func oncloseWs(conn *websocket.Conn, ctx context.Context, cancel context.CancelFunc, readMessages chan XtermReadMessages) {
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
			messageType, p, err := conn.ReadMessage()
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

func cmdWait(cmd *exec.Cmd, conn *websocket.Conn, tty *os.File) {
	err := cmd.Wait()
	if err != nil {
		// XtermLogger.Errorf("cmd wait: %s", err.Error())
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 137 {
					if conn != nil {
						err := conn.WriteMessage(websocket.TextMessage, []byte("POD_DOES_NOT_EXIST"))
						if err != nil {
							xtermLogger.Error("WriteMessage", "error", err)
						}
					}
				}
			}
		}
	} else {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER")
			err := conn.WriteMessage(websocket.CloseMessage, closeMsg)
			if err != nil {
				xtermLogger.Error("WriteMessage", "error", err.Error())
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
	}
}

func cmdOutputToWebsocket(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, tty *os.File, injectPreContent io.Reader) {
	if injectPreContent != nil {
		injectContent(injectPreContent, conn)
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
				err := conn.WriteMessage(websocket.BinaryMessage, buf[:read])
				if err != nil {
					xtermLogger.Error("WriteMessage", "error", err)
				}
				continue
			}
			return
		}
	}
}

func cmdOutputScannerToWebsocket(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, tty *os.File, injectPreContent io.Reader, component structs.ComponentEnum, namespace *string, controllerName *string, release *string) {
	_ = component
	if injectPreContent != nil {
		injectContent(injectPreContent, conn)
	}

	defer func() {
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			scanner := bufio.NewScanner(tty)
			for scanner.Scan() {
				line := scanner.Text()

				var entry LogEntry
				err := json.Unmarshal([]byte(line), &entry)
				if err != nil {
					continue
				}

				//if component != structs.ComponentAll {
				//	if entry.Component != component {
				//		continue
				//	}
				//}

				if namespace != nil {
					// log.Infof("namespace: %s", *namespace)
					if entry.Namespace != *namespace {
						continue
					}
				}

				if controllerName != nil {
					// log.Infof("controllerName: %s", *controllerName)
					if entry.ControllerName != *controllerName {
						continue
					}
				}

				if release != nil {
					// log.Infof("release: %s", *release)
					if entry.ReleaseName != *release {
						continue
					}
				}

				if conn != nil {
					if !(strings.HasSuffix(entry.Message, "\n") || strings.HasSuffix(entry.Message, "\n\r")) {
						entry.Message = entry.Message + "\n"
					}
					messageSt := fmt.Sprintf("[%s] %s %s", entry.Level, utils.FormatJsonTimePretty(entry.Time), entry.Message)
					err := conn.WriteMessage(websocket.BinaryMessage, []byte(messageSt))
					if err != nil {
						fmt.Println("WriteMessage", err.Error())
					}
					continue
				}
			}
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
