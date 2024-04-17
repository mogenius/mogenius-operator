package xterm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mogenius-k8s-manager/db"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	punq "github.com/mogenius/punq/kubernetes"

	"github.com/creack/pty"
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

type CmdWindowSize struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

func WsConnection(cmdType string, namespace string, controller string, pod string, container string, u url.URL, wsConnectionRequest WsConnectionRequest) (*websocket.Conn, error) {
	maxRetries := 6
	currentRetries := 0
	for {
		// add header
		headers := utils.HttpHeader("")
		headers.Add("x-channel-id", wsConnectionRequest.ChannelId)
		headers.Add("x-cmd", cmdType)
		headers.Add("x-namespace", namespace)
		headers.Add("x-controller", controller)
		headers.Add("x-pod-name", pod)
		headers.Add("x-container", container)
		headers.Add("x-type", "k8s")

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

func XTermCommandStreamConnection(
	cmdType string,
	wsConnectionRequest WsConnectionRequest,
	namespace string,
	controller string,
	pod string,
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
	conn, err := WsConnection(cmdType, namespace, controller, pod, container, websocketUrl, wsConnectionRequest)

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

func XTermBuildLogStreamConnection(wsConnectionRequest WsConnectionRequest, namespace string, controller string, container string, buildTask structs.BuildPrefixEnum, buildId uint64) {
	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))

	if wsConnectionRequest.WebsocketScheme == "" {
		log.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		log.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	conn, err := WsConnection("build-logs", namespace, controller, "", container, websocketUrl, wsConnectionRequest)

	defer func() {
		if conn != nil {
			conn.Close()
			// cancel()
		}
	}()

	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}
	//log.Infof("Connected to %s", websocketUrl.String())

	// Check if pod exists
	//podExists := punq.PodExists(namespace, pod, nil)
	//if !podExists.PodExists {
	//	if conn != nil {
	//		err := conn.WriteMessage(websocket.TextMessage, []byte("POD_DOES_NOT_EXIST"))
	//		if err != nil {
	//			log.Errorf("WriteMessage: %s", err.Error())
	//		}
	//	}
	//	log.Errorf("Pod %s does not exist, closing connection.", pod)
	//	return
	//}

	//buildIdNum, err := strconv.ParseUint(buildId, 10, 64)
	//if err != nil {
	//	log.Errorf(err.Error())
	//}

	key := structs.BuildJobInfoEntryKey(buildId, buildTask, namespace, controller, container)

	// init
	data := db.GetItemByKey(key)
	build := structs.CreateBuildJobEntryFromData(data)
	//conn.WriteMessage(websocket.TextMessage, []byte(strings.Replace(build.Result, "\n", "\n\r", -1)))
	conn.WriteMessage(websocket.TextMessage, []byte(build.Result))
	if err != nil {
		log.Errorf("WriteMessage: %s", err.Error())
	}

	defer func() {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "CLOSE_CONNECTION_FROM_PEER")
			if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
				log.Error("write close:", err)
			}
		}
	}()

	go func() {
		for {
			//ch, exists := builder.Channels[key]
			//
			//if !exists {
			//	log.Info("Channel does not exist.")
			//	select {
			//	case <-ctx.Done():
			//		return
			//	default:
			//		time.Sleep(1 * time.Second)
			//		continue
			//	}
			//}
			//
			//for message := range ch {
			//	if conn != nil {
			//		err := conn.WriteMessage(websocket.TextMessage, []byte(strings.Replace(message, "\n", "\n\r", -1)))
			//
			//		// err := conn.WriteMessage(websocket.BinaryMessage, []byte(message))
			//		if err != nil {
			//			log.Errorf("WriteMessage: %s", err.Error())
			//		}
			//		continue
			//	}
			//}
			log.Info("Channel closed.")
			return
		}
	}()

	for {
		_, reader, err := conn.ReadMessage()
		if err != nil {
			log.Errorf("Unable to grab next reader: %s", err.Error())
			return
		}

		// resize
		//if strings.HasPrefix(string(reader), "\x04") {
		//	str := strings.TrimPrefix(string(reader), "\x04")
		//
		//	var resizeMessage CmdWindowSize
		//	err := json.Unmarshal([]byte(str), &resizeMessage)
		//	if err != nil {
		//		log.Errorf("%s", err.Error())
		//		continue
		//	}
		//
		//	if err := pty.Setsize(tty, &pty.Winsize{Rows: uint16(resizeMessage.Rows), Cols: uint16(resizeMessage.Cols)}); err != nil {
		//		log.Errorf("Unable to resize: %s", err.Error())
		//		continue
		//	}
		//	continue
		//}

		if string(reader) == "PEER_IS_READY" {
			continue
		}
	}
}

func XTermScanImageLogStreamConnection(
	wsConnectionRequest WsConnectionRequest,
	namespace string,
	controller string,
	container string,
	cmdType string,
	scanImageType string,
	containerRegistryUrl string,
	containerRegistryUser *string,
	containerRegistryPat *string,
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
	conn, err := WsConnection("scan-image-logs", namespace, controller, "", container, websocketUrl, wsConnectionRequest)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))

	defer func() {
		cancel()
		if conn != nil {
			conn.Close()
		}
	}()

	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}

	containerImage, err := kubernetes.GetDeploymentImage(namespace, controller, container)
	if err != nil || containerImage == "" {
		return
	}

	cmdPull := fmt.Sprintf("docker pull %s", containerImage)
	cmdScanType := fmt.Sprintf("grype %s", containerImage)
	switch scanImageType {
	case "grype":
		cmdScanType = fmt.Sprintf("grype %s", containerImage)
	case "dive":
		cmdScanType = fmt.Sprintf("dive %s", containerImage)
	case "trivy":
		cmdScanType = fmt.Sprintf("trivy image %s", containerImage)
	}
	cmdString := fmt.Sprintf("%s && %s", cmdPull, cmdScanType)
	if containerRegistryUser != nil && containerRegistryPat != nil {
		if *containerRegistryUser != "" && *containerRegistryPat != "" {
			cmdString = fmt.Sprintf(
				`echo '%s' | docker login %s -u %s --password-stdin && %s && %s`,
				*containerRegistryPat, containerRegistryUrl, *containerRegistryUser, cmdPull, cmdScanType)

		}
	}

	// Start pty/cmd
	cmd := exec.Command("sh", "-c", cmdString)
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
		cancel()
	}()

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Errorf("cmd wait: %s", err.Error())
			if exiterr, ok := err.(*exec.ExitError); ok {
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					log.Error(status.ExitStatus())
					if status.ExitStatus() == 137 {
						if conn != nil {
							err := conn.WriteMessage(websocket.TextMessage, []byte("POD_DOES_NOT_EXIST"))
							if err != nil {
								log.Errorf("WriteMessage: %s", err.Error())
							}
						}
					} else if status.ExitStatus() == 1 {
						cmd.Process.Kill()
						cmd.Process.Wait()
						tty.Close()
						cancel()
						log.Info("Terminal closed.")
						return
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
			cancel()
			log.Info("Terminal closed.")
		}
	}()

	streamBeginning := false
	go func() {
		buf := make([]byte, 1024)
		for {
			read, err := tty.Read(buf)
			if err != nil {
				log.Errorf("Unable to read from pty/cmd: %s", err.Error())
				return
			}
			if conn != nil {
				if streamBeginning == false {
					if len(string(buf[:read])) > 0 {
						re := regexp.MustCompile(`Vulnerability`)
						matches := re.FindAllString(string(buf[:read]), -1)

						if len(matches) > 0 {
							cancel()
							streamBeginning = true
						}
					}
				}

				err := conn.WriteMessage(websocket.BinaryMessage, buf[:read])
				if err != nil {
					log.Errorf("WriteMessage: %s", err.Error())
				}
				continue
			}
			return
		}
	}()

	if scanImageType == "grype" {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					time.Sleep(1 * time.Second)
					err := conn.WriteMessage(websocket.TextMessage, []byte("."))
					if err != nil {
						log.Errorf("WriteMessage: %s", err.Error())
					}
					continue
				}
			}
		}()
	}

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
		if cmdType == "exec-sh" {
			tty.Write(reader)
		}
	}
}
