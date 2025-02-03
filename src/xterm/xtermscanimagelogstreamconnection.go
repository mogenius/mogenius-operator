package xterm

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/kubernetes"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

func cmdScanImageLogOutputToWebsocket(ctx context.Context, cancel context.CancelFunc, scanImageType string, conn *websocket.Conn, connWriteLock *sync.Mutex, tty *os.File) {
	buildTimeout, err := strconv.Atoi(config.Get("MO_BUILDER_BUILD_TIMEOUT"))
	assert.Assert(err == nil, err)
	toolLoadingCtx, toolLoadingCancel := context.WithTimeout(context.Background(), time.Second*time.Duration(buildTimeout))

	defer func() {
		toolLoadingCancel()
		cancel()
	}()

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
							connWriteLock.Lock()
							err := conn.WriteMessage(websocket.TextMessage, []byte("."))
							connWriteLock.Unlock()
							if err != nil {
								xtermLogger.Error("WriteMessage", "error", err)
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
					// XtermLogger.Errorf("1 Unable to read from pty/cmd: %s", err.Error())
					return
				}
				if conn != nil {
					// loading
					if !streamBeginning {
						if len(string(buf[:read])) > 0 {
							re := regexp.MustCompile(`Vulnerability`)
							matches := re.FindAllString(string(buf[:read]), -1)

							if len(matches) > 0 {
								toolLoadingCancel()
								streamBeginning = true
							}
						}
					}
					connWriteLock.Lock()
					err := conn.WriteMessage(websocket.BinaryMessage, buf[:read])
					connWriteLock.Unlock()
					if err != nil {
						xtermLogger.Error("WriteMessage", "error", err)
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
		xtermLogger.Error("WebsocketScheme is empty")
		return
	}

	if wsConnectionRequest.WebsocketHost == "" {
		xtermLogger.Error("WebsocketHost is empty")
		return
	}

	websocketUrl := url.URL{Scheme: wsConnectionRequest.WebsocketScheme, Host: wsConnectionRequest.WebsocketHost, Path: "/xterm-stream"}
	// context
	buildTimeout, err := strconv.Atoi(config.Get("MO_BUILDER_BUILD_TIMEOUT"))
	assert.Assert(err == nil, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(buildTimeout))
	// websocket connection
	readMessages, conn, connWriteLock, _, err := generateWsConnection(cmdType, namespace, controller, "", container, websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		xtermLogger.Error("Unable to connect to websocket", "error", err)
		return
	}

	defer func() {
		// log.Info("[XTermScanImageLogStreamConnection] Closing connection.")
		cancel()
	}()

	containerImage, err := kubernetes.GetDeploymentImage(namespace, controller, container)
	if err != nil || containerImage == "" {
		return
	}

	cmdPull := fmt.Sprintf("docker pull %s", containerImage)
	var cmdScanType string
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

	go cmdWait(cmd, conn, connWriteLock, tty)

	go cmdScanImageLogOutputToWebsocket(ctx, cancel, scanImageType, conn, connWriteLock, tty)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
