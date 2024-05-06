package xterm

import (
	"context"
	"encoding/json"
	"github.com/creack/pty"
	"io"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
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

type ClusterToolConnectionRequest struct {
	CmdType string `json:"cmdType" validate:"required"`
	Tool    string `json:"tool" validate:"required"`

	WsConnection WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
}

type CmdWindowSize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

var LogChannels = make(map[string]chan string)

func isPodReady(pod *v1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func clearScreen(conn *websocket.Conn) {
	// clear screen
	err := conn.WriteMessage(websocket.BinaryMessage, []byte("\u001b[2J\u001b[H"))
	if err != nil {
		log.Errorf("WriteMessage: %s", err.Error())
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
) (*websocket.Conn, error) {
	maxRetries := 6
	currentRetries := 0
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
			log.Errorf("Failed to connect, retrying in 5 seconds: %s", err.Error())
			if currentRetries >= maxRetries {
				log.Errorf("Max retries reached, exiting.")
				return nil, err
			}
			time.Sleep(5 * time.Second)
			currentRetries++
			continue
		}

		// log.Infof("Connected to %s", u.String())

		// API send ack when it is ready to receive messages.
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _, err = conn.ReadMessage()
		if err != nil {
			log.Errorf("Failed to receive ack-ready, retrying in 5 seconds: %s", err.Error())
			time.Sleep(5 * time.Second)
			if currentRetries >= maxRetries {
				log.Errorf("Max retries reached, exiting.")
				return conn, err
			}
			currentRetries++
			continue
		}

		conn.SetReadDeadline(time.Time{})
		// log.Infof("Ready ack from connected stream endpoint: %s.", string(ack))

		//
		go oncloseWs(conn, ctx, cancel)
		return conn, nil
	}
}

func oncloseWs(conn *websocket.Conn, ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			if conn != nil {
				conn.Close()
			}
			log.Info("[oncloseWs] Context done. Closing connection.")
			return
		default:
			_, _, err := conn.ReadMessage()
			if err != nil {
				if closeErr, ok := err.(*websocket.CloseError); ok {
					log.Printf("[oncloseWs] WebSocket closed with status code %d and message: %s\n", closeErr.Code, closeErr.Text)
				} else {
					log.Printf("[oncloseWs] Error reading message: %v\n. Connection closed.", err)
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
			log.Println("The connection was closed")
			return err
		}
		log.Println("Send Ping error:", err)
		return err
	}
	return nil
}

func cmdWait(cmd *exec.Cmd, conn *websocket.Conn, tty *os.File) {
	err := cmd.Wait()
	if err != nil {
		log.Errorf("cmd wait: %s", err.Error())
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 137 {
					if conn != nil {
						err := conn.WriteMessage(websocket.TextMessage, []byte("POD_DOES_NOT_EXIST"))
						if err != nil {
							log.Errorf("WriteMessage: %s", err.Error())
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
				log.Errorf("WriteMessage: %s", err.Error())
			}
		}
		cmd.Process.Kill()
		cmd.Process.Wait()
		tty.Close()
	}
}

func cmdOutputToWebsocket(ctx context.Context, conn *websocket.Conn, tty *os.File, injectPreContent io.Reader) {
	if injectPreContent != nil {
		injectContent(injectPreContent, conn)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf := make([]byte, 1024)
			read, err := tty.Read(buf)
			if err != nil {
				log.Errorf("Unable to read from pty/cmd: %s", err.Error())
				return
			}
			if conn != nil {
				err := conn.WriteMessage(websocket.BinaryMessage, buf[:read])
				if err != nil {
					log.Errorf("WriteMessage: %s", err.Error())
				}
				continue
			}
			return
		}
	}
}

func websocketToCmdInput(ctx context.Context, conn *websocket.Conn, tty *os.File) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, reader, err := conn.ReadMessage()
			if err != nil {
				log.Errorf("Unable to read from websocket: %s", err.Error())
				return
			}

			if strings.HasPrefix(string(reader), "\x04") {
				str := strings.TrimPrefix(string(reader), "\x04")

				var resizeMessage CmdWindowSize
				err := json.Unmarshal([]byte(str), &resizeMessage)
				if err != nil {
					log.Errorf("%s", err.Error())
					continue
				}

				if err := pty.Setsize(tty, &pty.Winsize{Rows: uint16(resizeMessage.Rows), Cols: uint16(resizeMessage.Cols)}); err != nil {
					log.Errorf("Unable to resize: %s", err.Error())
					continue
				}
				continue
			}

			if string(reader) == "PEER_IS_READY" {
				continue
			}

			tty.Write(reader)
		}
	}
}
