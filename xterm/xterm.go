package xterm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	log "github.com/sirupsen/logrus"
)

var XtermLogger = log.WithField("component", structs.ComponentKubernetes)

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
	utils.AddSecret(&p.ContainerRegistryUser)
	utils.AddSecret(&p.ContainerRegistryPat)
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
			XtermLogger.Errorf("Context done.")
			return
		default:
			pod, err := provider.ClientSet.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
			if err != nil {
				XtermLogger.Errorf("Unable to get pod: %s", err.Error())
				if conn != nil {
					// clear screen
					clearScreen(conn)
					closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "POD_DOES_NOT_EXIST")
					if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
						XtermLogger.Debug("write close:", err)
					}
				}
				return
			}

			if isPodAvailable(pod) {
				XtermLogger.Infof("Pod %s is ready.", pod.Name)
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
					XtermLogger.Errorf("WriteMessage: %s", err.Error())
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
		XtermLogger.Errorf("WriteMessage: %s", err.Error())
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
			XtermLogger.Errorf("Failed to connect, retrying in 5 seconds: %s", err.Error())
			if currentRetries >= maxRetries {
				XtermLogger.Errorf("Max retries reached, exiting.")
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
			XtermLogger.Error(err)
		}
		_, _, err = conn.ReadMessage()
		if err != nil {
			XtermLogger.Errorf("Failed to receive ack-ready, retrying in 5 seconds: %s", err.Error())
			time.Sleep(5 * time.Second)
			if currentRetries >= maxRetries {
				XtermLogger.Errorf("Max retries reached, exiting.")
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
					XtermLogger.Debugf("[oncloseWs] WebSocket closed with status code %d and message: %s\n", closeErr.Code, closeErr.Text)
				} else {
					XtermLogger.Debugf("[oncloseWs] Error reading message: %v\n. Connection closed.", err)
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
			XtermLogger.Println("The connection was closed")
			return err
		}
		XtermLogger.Println("Send Ping error:", err)
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
							XtermLogger.Errorf("WriteMessage: %s", err.Error())
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
				XtermLogger.Errorf("WriteMessage: %s", err.Error())
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
	}
}

func cmdOutputToWebsocketForOperatorLog(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, tty *os.File, injectPreContent io.Reader, namespace *string, controller *string) {
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
			reader := bufio.NewReader(tty)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err.Error() == "EOF" {
						break
					}
					return
				}

				// filter log lines by namespace and controller
				if namespace != nil && controller != nil {
					var entry LogEntry
					err := json.Unmarshal([]byte(line), &entry)
					if err != nil {
						continue
					}
					if entry.Namespace != *namespace || entry.ControllerName != *controller {
						continue
					}

					if conn != nil {
						var messageSt string
						if strings.HasSuffix(entry.Message, "\n") {
							messageSt = fmt.Sprintf("[%s] %s %s", entry.Level, utils.FormatJsonTimePretty(entry.Time), entry.Message)
						} else {
							messageSt = fmt.Sprintf("[%s] %s %s\n", entry.Level, utils.FormatJsonTimePretty(entry.Time), entry.Message)
						}
						err := conn.WriteMessage(websocket.BinaryMessage, []byte(messageSt))
						if err != nil {
							fmt.Println("WriteMessage", err.Error())
						}
						continue
					}
					continue
				}

				if conn != nil {
					err := conn.WriteMessage(websocket.BinaryMessage, []byte(line))
					if err != nil {
						XtermLogger.Errorf("WriteMessage: %s", err.Error())
					}
					continue
				}
				return
			}
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
					XtermLogger.Errorf("WriteMessage: %s", err.Error())
				}
				continue
			}
			return
		}
	}
}

func cmdOutputScannerToWebsocket(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, tty *os.File, injectPreContent io.Reader, component structs.ComponentEnum, namespace *string, controllerName *string, release *string) {
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
					messageSt := fmt.Sprintf("[%s] %s %s\n", entry.Level, utils.FormatJsonTimePretty(entry.Time), entry.Message)
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
								XtermLogger.Errorf("Unable to resize: %s", err.Error())
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
							XtermLogger.Error(err)
						}
					}
				}
			}
		}
	}
}
