package utils

import (
	"encoding/json"
	"log"
	"mogenius-k8s-manager/logger"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
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

func XtermCommandStreamWsConnection(u url.URL, cmdConnectionRequest CmdConnectionRequest) (*websocket.Conn, error) {
	maxRetries := 6
	currentRetries := 0
	for {
		// add header
		headers := HttpHeader("")
		headers.Add("x-channel-id", cmdConnectionRequest.ChannelId)

		dialer := &websocket.Dialer{}
		c, _, err := dialer.Dial(u.String(), headers)
		if err != nil {
			logger.Log.Errorf("Failed to connect, retrying in 5 seconds: %s", err.Error())
			if currentRetries >= maxRetries {
				logger.Log.Errorf("Max retries reached, exiting.")
				return nil, err
			}
			time.Sleep(5 * time.Second)
			currentRetries++
			continue
		}

		// logger.Log.Infof("Connected to %s", u.String())

		// API send ack when it is ready to receive messages.
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _, err = c.ReadMessage()
		if err != nil {
			logger.Log.Errorf("Failed to receive ack-ready, retrying in 5 seconds: %s", err.Error())
			time.Sleep(5 * time.Second)
			if currentRetries >= maxRetries {
				logger.Log.Errorf("Max retries reached, exiting.")
				return c, err
			}
			currentRetries++
			continue
		}

		c.SetReadDeadline(time.Time{})
		// logger.Log.Infof("Ready ack from connected stream endpoint: %s.", string(ack))
		return c, nil
	}
}

func XTermCommandStreamConnection(cmdConnectionRequest CmdConnectionRequest, cmd *exec.Cmd) {
	if cmdConnectionRequest.WebsocketScheme == "" {
		logger.Log.Error("WebsocketScheme is empty")
		return
	}

	if cmdConnectionRequest.WebsocketHost == "" {
		logger.Log.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: cmdConnectionRequest.WebsocketScheme, Host: cmdConnectionRequest.WebsocketHost, Path: "/xterm-stream"}

	con, err := XtermCommandStreamWsConnection(websocketUrl, cmdConnectionRequest)
	defer func() {
		if con != nil {
			con.Close()
		}
	}()
	if err != nil {
		logger.Log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}

	cmd.Env = append(os.Environ(), "TERM=xterm-color")

	tty, err := pty.Start(cmd)
	if err != nil {
		log.Printf("Unable to start pty/cmd: %s", err.Error())
		if con != nil {
			con.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		}
		return
	}

	defer func() {
		if con != nil {
			con.WriteMessage(websocket.TextMessage, []byte("TERMINAL_CLOSED"))
		}
		cmd.Process.Kill()
		cmd.Process.Wait()
		tty.Close()
		con.Close()
	}()

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("cmd wait: %s", err.Error())
		} else {
			log.Printf("Terminal closed.")
		}
	}()

	go func() {
		for {
			buf := make([]byte, 1024)
			read, err := tty.Read(buf)
			if err != nil {
				log.Printf("Unable to read from pty/cmd: %s", err.Error())
				return
			}
			if con != nil {
				con.WriteMessage(websocket.BinaryMessage, buf[:read])
			} else {
				return
			}
		}
	}()

	for {
		_, reader, err := con.ReadMessage()
		if err != nil {
			log.Printf("Unable to grab next reader: %s", err.Error())
			return
		}

		if strings.HasPrefix(string(reader), "\x04") {
			str := strings.TrimPrefix(string(reader), "\x04")

			var resizeMessage CmdWindowSize
			err := json.Unmarshal([]byte(str), &resizeMessage)
			if err != nil {
				log.Printf("%s", err.Error())
				continue
			}

			if err := pty.Setsize(tty, &pty.Winsize{Rows: uint16(resizeMessage.Rows), Cols: uint16(resizeMessage.Cols)}); err != nil {
				log.Printf("Unable to resize: %s", err.Error())
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
