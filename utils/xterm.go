package utils

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	punq "github.com/mogenius/punq/kubernetes"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type CmdConnectionRequest struct {
	ChannelId       string `json:"channelId" validate:"required"`
	WebsocketScheme string `json:"websocketScheme" validate:"required"`
	WebsocketHost   string `json:"websocketHost" validate:"required"`
}

type CmdWindowSize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

func WsConnection(cmdType string, namespace string, pod string, container string, u url.URL, cmdConnectionRequest CmdConnectionRequest) (*websocket.Conn, error) {
	maxRetries := 6
	currentRetries := 0
	for {
		// add header
		headers := HttpHeader("")
		headers.Add("x-channel-id", cmdConnectionRequest.ChannelId)
		headers.Add("x-cmd", cmdType)
		headers.Add("x-namespace", namespace)
		headers.Add("x-pod-name", pod)
		headers.Add("x-container", container)

		dialer := &websocket.Dialer{}
		c, _, err := dialer.Dial(u.String(), headers)
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
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _, err = c.ReadMessage()
		if err != nil {
			log.Errorf("Failed to receive ack-ready, retrying in 5 seconds: %s", err.Error())
			time.Sleep(5 * time.Second)
			if currentRetries >= maxRetries {
				log.Errorf("Max retries reached, exiting.")
				return c, err
			}
			currentRetries++
			continue
		}

		c.SetReadDeadline(time.Time{})
		// log.Infof("Ready ack from connected stream endpoint: %s.", string(ack))
		return c, nil
	}
}

func InjectContent(content io.Reader, conn *websocket.Conn) {
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

func XTermCommandStreamConnection(cmdType string, cmdConnectionRequest CmdConnectionRequest, namespace string, pod string, container string, cmd *exec.Cmd, injectPreContent io.Reader) {
	if cmdConnectionRequest.WebsocketScheme == "" {
		log.Error("WebsocketScheme is empty")
		return
	}

	if cmdConnectionRequest.WebsocketHost == "" {
		log.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: cmdConnectionRequest.WebsocketScheme, Host: cmdConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	conn, err := WsConnection(cmdType, namespace, pod, container, websocketUrl, cmdConnectionRequest)

	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}
	//log.Infof("Connected to %s", websocketUrl.String())

	// Check if pod exists
	podExists := punq.PodExists(namespace, pod, nil)
	if !podExists.PodExists {
		if conn != nil {
			err := conn.WriteMessage(websocket.TextMessage, []byte("POD_DOES_NOT_EXIST"))
			if err != nil {
				log.Errorf("WriteMessage: %s", err.Error())
			}
		}
		log.Errorf("Pod %s does not exist, closing connection.", pod)
		return
	}

	// Start pty/cmd
	cmd.Env = append(os.Environ(), "TERM=xterm-color")
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
				log.Error("write close:", err)
			}
		}
		cmd.Process.Kill()
		cmd.Process.Wait()
		tty.Close()
	}()

	go func() {
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
			log.Info("Terminal closed.")
		}
	}()

	go func() {
		if injectPreContent != nil {
			InjectContent(injectPreContent, conn)
		}

		for {
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
	}()

	for {
		_, reader, err := conn.ReadMessage()
		if err != nil {
			log.Errorf("Unable to grab next reader: %s", err.Error())
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
