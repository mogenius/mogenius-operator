package xterm

import (
	"context"
	"fmt"
	punq "github.com/mogenius/punq/kubernetes"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

func XTermOperatorStreamConnection(
	wsConnectionRequest WsConnectionRequest,
	namespace string,
	controller string,
	logTail string,
) {
	cmdType := "log"

	k8sManagerNamespace := "mogenius"
	k8sManagerController := "mogenius-k8s-manager"
	podName := ""

	podList := punq.PodIdsFor(k8sManagerNamespace, &k8sManagerController, nil)
	if len(podList) > 0 {
		podName = podList[0]
	}
	cmd := exec.Command("kubectl", "logs", "-f", podName, fmt.Sprintf("--tail=%s", logTail), "-c", k8sManagerController, "-n", k8sManagerNamespace)

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30*time.Minute))
	// websocket connection
	readMessages, conn, err := generateWsConnection(cmdType, k8sManagerNamespace, k8sManagerController, "", "", websocketUrl, wsConnectionRequest, ctx, cancel)
	if err != nil {
		log.Errorf("Unable to connect to websocket: %s", err.Error())
		return
	}

	defer func() {
		// log.Info("[XTermCommandStreamConnection] Closing connection.")
		cancel()
	}()

	// Check if pod exists
	podExists := punq.PodExists(k8sManagerNamespace, podName, nil)
	if !podExists.PodExists {
		if conn != nil {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "POD_DOES_NOT_EXIST")
			if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
				// log.Error("write close:", err)
			}
		}
		log.Errorf("Pod %s does not exist, closing connection.", podName)
		return
	}

	// kube provider
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		log.Warningf("Unable to create kube provider: %s", err.Error())
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	// check if pod is ready
	go checkPodIsReady(ctx, &wg, provider, k8sManagerNamespace, podName, conn)
	wg.Wait()

	// send ping
	err = wsPing(conn)
	if err != nil {
		log.Errorf("Unable to send ping: %s", err.Error())
		return
	}

	// Start pty/cmd
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
				// log.Error("write close:", err)
			}
		}
		cmd.Process.Kill()
		cmd.Process.Wait()
		tty.Close()
	}()

	// send cmd wait
	go cmdWait(cmd, conn, tty)

	// cmd output to websocket
	go cmdOutputToWebsocket(ctx, cancel, conn, tty, nil, &namespace, &controller)

	// websocket to cmd input
	websocketToCmdInput(*readMessages, ctx, tty, &cmdType)
}
