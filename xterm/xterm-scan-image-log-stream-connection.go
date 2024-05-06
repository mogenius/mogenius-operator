package xterm

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/utils"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func cmdScanImageLogOutputToWebsocket(ctx context.Context, scanImageType string, conn *websocket.Conn, tty *os.File) {
	toolLoadingCtx, toolLoadingCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))

	for {
		select {
		case <-ctx.Done():
			toolLoadingCancel()
			return
		default:
			// loading
			streamBeginning := false
			if scanImageType == "grype" {
				go func() {
					for {
						select {
						case <-toolLoadingCtx.Done():
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

			buf := make([]byte, 1024)
			for {
				read, err := tty.Read(buf)
				if err != nil {
					log.Errorf("Unable to read from pty/cmd: %s", err.Error())
					return
				}
				if conn != nil {

					// loading
					if streamBeginning == false {
						if len(string(buf[:read])) > 0 {
							re := regexp.MustCompile(`Vulnerability`)
							matches := re.FindAllString(string(buf[:read]), -1)

							if len(matches) > 0 {
								toolLoadingCancel()
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
			}
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
	// context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(utils.CONFIG.Builder.BuildTimeout))
	// websocket connection
	conn, err := generateWsConnection(cmdType, namespace, controller, "", container, websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}

	defer func() {
		log.Info("[XTermScanImageLogStreamConnection] Closing connection.")
		cancel()
	}()

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
	default:
		cmdScanType = fmt.Sprintf("grype %s", containerImage)
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
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
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

	go cmdWait(cmd, conn, tty)

	go cmdScanImageLogOutputToWebsocket(ctx, scanImageType, conn, tty)

	for {
		select {
		case <-ctx.Done():
			return
		default:
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
}
