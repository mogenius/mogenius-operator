package xterm

import (
	"context"
	"fmt"
	"io"
	"mogenius-operator/src/utils"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Control message prefixes (text WS frames)
const (
	pfmOpenPrefix  = "PFM:O:"
	pfmClosePrefix = "PFM:C:"
)

var pfRestConfig *rest.Config
var pfClientset k8s.Interface

// SetupPortForward stores K8s client dependencies needed for port-forward tunneling.
func SetupPortForward(restConfig *rest.Config, clientset k8s.Interface) {
	pfRestConfig = restConfig
	pfClientset = clientset
}

type PortForwardConnectionRequest struct {
	Namespace    string              `json:"namespace" validate:"required"`
	RemotePort   int                 `json:"remotePort" validate:"required"`
	Kind         string              `json:"kind" validate:"required"`
	WorkloadName string              `json:"workloadName" validate:"required"`
	WsConnection WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
}

// PortForwardStreamConnection establishes a port-forward tunnel through the stream gateway.
//
// Protocol:
//   - Text frames for control: PFM:O:<connID>, PFM:C:<connID>, PEER_IS_READY, BROWSER_PING
//   - Binary frames for data:  [1 byte connID length][connID as ASCII][raw TCP bytes]
func PortForwardStreamConnection(request PortForwardConnectionRequest) {
	logger := xtermLogger.With("scope", "PortForwardStreamConnection")

	logger.Info("=== PORT-FORWARD REQUEST RECEIVED ===",
		"namespace", request.Namespace,
		"remotePort", request.RemotePort,
		"kind", request.Kind,
		"targetName", request.WorkloadName,
		"wsScheme", request.WsConnection.WebsocketScheme,
		"wsHost", request.WsConnection.WebsocketHost,
		"channelId", request.WsConnection.ChannelId,
	)

	if pfRestConfig == nil || pfClientset == nil {
		logger.Error("Port-forward not initialized. Call SetupPortForward() first.")
		return
	}

	// Step 1: Resolve target to a pod name
	podName, err := resolvePortForwardTarget(pfClientset, request)
	if err != nil {
		logger.Error("Failed to resolve target to pod", "error", err)
		return
	}
	logger.Info("Resolved target", "pod", podName, "namespace", request.Namespace, "port", request.RemotePort)

	// Step 2: Connect to the stream gateway
	wsReq := request.WsConnection
	wsURL := url.URL{
		Scheme: wsReq.WebsocketScheme,
		Host:   wsReq.WebsocketHost,
		Path:   "/xterm-stream",
	}

	headers := utils.HttpHeader("")
	headers.Add("x-channel-id", wsReq.ChannelId)
	headers.Add("x-cmd", "port-forward")
	headers.Add("x-namespace", request.Namespace)
	headers.Add("x-pod-name", podName)
	headers.Add("x-type", "k8s")

	logger.Info("Connecting to stream gateway", "url", wsURL.String())
	dialer := &websocket.Dialer{
		EnableCompression: true,
	}
	wsConn, _, err := dialer.Dial(wsURL.String(), headers)
	if err != nil {
		logger.Error("Failed to connect to stream gateway", "error", err)
		return
	}
	wsMu := &sync.Mutex{}
	defer func() {
		wsMu.Lock()
		wsConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		wsMu.Unlock()
		wsConn.Close()
	}()

	// Wait for ack-ready from stream gateway
	wsConn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, ackMsg, err := wsConn.ReadMessage()
	if err != nil {
		logger.Error("Failed to receive ack from stream gateway", "error", err)
		return
	}
	logger.Info("Stream gateway ack received", "msg", string(ackMsg))
	wsConn.SetReadDeadline(time.Time{})

	// Step 3: Start k8s port-forward on a random local port
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logger.Error("Failed to create local listener", "error", err)
		return
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port
	localListener.Close()

	stopChan := make(chan struct{})
	readyChan := make(chan struct{})
	errChan := make(chan error, 1)

	go func() {
		pfURL, err := url.Parse(fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/portforward",
			pfRestConfig.Host, request.Namespace, podName))
		if err != nil {
			errChan <- fmt.Errorf("invalid URL: %w", err)
			return
		}

		transport, upgrader, err := spdy.RoundTripperFor(pfRestConfig)
		if err != nil {
			errChan <- fmt.Errorf("SPDY transport: %w", err)
			return
		}

		spdyDialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, pfURL)
		ports := []string{fmt.Sprintf("%d:%d", localPort, request.RemotePort)}

		fw, err := portforward.New(spdyDialer, ports, stopChan, readyChan, io.Discard, io.Discard)
		if err != nil {
			errChan <- fmt.Errorf("portforward create: %w", err)
			return
		}

		if err := fw.ForwardPorts(); err != nil {
			errChan <- err
		}
	}()

	// Wait for port-forward to be ready
	select {
	case <-readyChan:
		logger.Info("K8s port-forward established", "localPort", localPort, "remotePort", request.RemotePort)
	case err := <-errChan:
		logger.Error("K8s port-forward failed", "error", err)
		wsMu.Lock()
		wsConn.WriteMessage(websocket.TextMessage, []byte("ERROR: "+err.Error()))
		wsMu.Unlock()
		return
	case <-time.After(30 * time.Second):
		logger.Error("K8s port-forward timeout")
		close(stopChan)
		return
	}

	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	logger.Info("Tunnel ready, waiting for client data", "localAddr", localAddr)

	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }

	// Connection map: connID → local TCP connection to k8s port-forward
	conns := make(map[string]net.Conn)
	connsMu := &sync.RWMutex{}

	// wsSendText sends a text WS frame (control messages).
	wsSendText := func(msg string) error {
		wsMu.Lock()
		defer wsMu.Unlock()
		return wsConn.WriteMessage(websocket.TextMessage, []byte(msg))
	}

	// wsSendBinary sends a binary WS frame: [1 byte connID len][connID][raw data].
	wsSendBinary := func(connID string, data []byte) error {
		connIDBytes := []byte(connID)
		frame := make([]byte, 1+len(connIDBytes)+len(data))
		frame[0] = byte(len(connIDBytes))
		copy(frame[1:], connIDBytes)
		copy(frame[1+len(connIDBytes):], data)

		wsMu.Lock()
		defer wsMu.Unlock()
		return wsConn.WriteMessage(websocket.BinaryMessage, frame)
	}

	// readLocalAndSend reads from a local TCP connection and sends binary WS frames.
	readLocalAndSend := func(connID string, localConn net.Conn) {
		defer func() {
			connsMu.Lock()
			delete(conns, connID)
			connsMu.Unlock()
			localConn.Close()
			wsSendText(pfmClosePrefix + connID)
			logger.Info("Sub-connection closed", "connID", connID)
		}()

		buf := make([]byte, 32*1024)
		for {
			n, err := localConn.Read(buf)
			if n > 0 {
				if writeErr := wsSendBinary(connID, buf[:n]); writeErr != nil {
					logger.Info("WS write error", "connID", connID, "error", writeErr)
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					logger.Info("Local TCP read ended", "connID", connID, "error", err)
				} else {
					logger.Info("Local TCP EOF", "connID", connID)
				}
				return
			}
		}
	}

	// WebSocket read loop — binary frames = data, text frames = control
	go func() {
		defer closeDone()
		for {
			msgType, msg, err := wsConn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					logger.Info("WebSocket read ended", "error", err)
				} else {
					logger.Info("WebSocket closed normally by peer")
				}
				return
			}

			// Binary frame: [1 byte connID len][connID][raw TCP data]
			if msgType == websocket.BinaryMessage {
				if len(msg) < 2 {
					continue
				}
				connIDLen := int(msg[0])
				if len(msg) < 1+connIDLen {
					continue
				}
				connID := string(msg[1 : 1+connIDLen])
				data := msg[1+connIDLen:]

				connsMu.RLock()
				localConn := conns[connID]
				connsMu.RUnlock()

				if localConn != nil {
					if _, err := localConn.Write(data); err != nil {
						logger.Info("Local TCP write error", "connID", connID, "error", err)
					}
				}
				continue
			}

			// Text frame: control messages
			msgStr := string(msg)

			if msgStr == "CLOSE_CONNECTION_FROM_PEER" {
				logger.Info("Received CLOSE_CONNECTION_FROM_PEER")
				return
			}

			// Skip pings etc.
			if !strings.HasPrefix(msgStr, "PFM:") {
				continue
			}

			// PFM:O:<connID> — open a new sub-connection
			if strings.HasPrefix(msgStr, pfmOpenPrefix) {
				connID := strings.TrimPrefix(msgStr, pfmOpenPrefix)
				logger.Info("Opening sub-connection", "connID", connID)

				localConn, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
				if err != nil {
					logger.Error("Failed to dial local port-forward", "connID", connID, "error", err)
					wsSendText(pfmClosePrefix + connID)
					continue
				}

				connsMu.Lock()
				conns[connID] = localConn
				connsMu.Unlock()

				go readLocalAndSend(connID, localConn)
				continue
			}

			// PFM:C:<connID> — CLI closed a sub-connection
			if strings.HasPrefix(msgStr, pfmClosePrefix) {
				connID := strings.TrimPrefix(msgStr, pfmClosePrefix)
				logger.Info("Remote closed sub-connection", "connID", connID)
				connsMu.Lock()
				if c, ok := conns[connID]; ok {
					c.Close()
					delete(conns, connID)
				}
				connsMu.Unlock()
				continue
			}
		}
	}()

	<-done

	// Cleanup: close all sub-connections
	connsMu.Lock()
	for id, c := range conns {
		c.Close()
		delete(conns, id)
	}
	connsMu.Unlock()

	close(stopChan)
	logger.Info("Port-forward tunnel closed", "pod", podName, "port", request.RemotePort)
}

func resolvePortForwardTarget(clientset k8s.Interface, req PortForwardConnectionRequest) (string, error) {
	kind := strings.ToLower(req.Kind)

	if kind == "pod" {
		return req.WorkloadName, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var labelSelector string

	switch kind {
	case "service":
		svc, err := clientset.CoreV1().Services(req.Namespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("service %q not found: %w", req.WorkloadName, err)
		}
		labelSelector = labels.Set(svc.Spec.Selector).String()

	case "deployment":
		deploy, err := clientset.AppsV1().Deployments(req.Namespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("deployment %q not found: %w", req.WorkloadName, err)
		}
		labelSelector = metav1.FormatLabelSelector(deploy.Spec.Selector)

	case "statefulset":
		sts, err := clientset.AppsV1().StatefulSets(req.Namespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("statefulset %q not found: %w", req.WorkloadName, err)
		}
		labelSelector = metav1.FormatLabelSelector(sts.Spec.Selector)

	case "daemonset":
		ds, err := clientset.AppsV1().DaemonSets(req.Namespace).Get(ctx, req.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("daemonset %q not found: %w", req.WorkloadName, err)
		}
		labelSelector = metav1.FormatLabelSelector(ds.Spec.Selector)

	default:
		return "", fmt.Errorf("unsupported kind %q", req.Kind)
	}

	pods, err := clientset.CoreV1().Pods(req.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return "", fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no running pods found for selector %q", labelSelector)
	}

	return pods.Items[0].Name, nil
}