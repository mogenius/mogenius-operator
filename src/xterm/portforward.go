package xterm

import (
	"context"
	"encoding/base64"
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

const pfDataPrefix = "PF:"

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
// Uses pfRestConfig and pfClientset set via SetupPortForward().
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

	// Step 2: Connect to the stream gateway directly (not via GenerateWsConnection,
	// because its internal read-loop conflicts with our own reads).
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
	dialer := &websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL.String(), headers)
	if err != nil {
		logger.Error("Failed to connect to stream gateway", "error", err)
		return
	}
	connWriteLock := &sync.Mutex{}
	defer func() {
		connWriteLock.Lock()
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		connWriteLock.Unlock()
		conn.Close()
	}()

	// Wait for ack-ready from stream gateway
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, ackMsg, err := conn.ReadMessage()
	if err != nil {
		logger.Error("Failed to receive ack from stream gateway", "error", err)
		return
	}
	logger.Info("Stream gateway ack received", "msg", string(ackMsg))
	conn.SetReadDeadline(time.Time{}) // Clear deadline

	// Step 3: Start k8s port-forward on a random local port
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logger.Error("Failed to create local listener", "error", err)
		return
	}
	localPort := localListener.Addr().(*net.TCPAddr).Port
	localListener.Close() // Release the port for portforward to use

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

		dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, pfURL)
		ports := []string{fmt.Sprintf("%d:%d", localPort, request.RemotePort)}

		fw, err := portforward.New(dialer, ports, stopChan, readyChan, io.Discard, io.Discard)
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
		if conn != nil {
			connWriteLock.Lock()
			conn.WriteMessage(websocket.TextMessage, []byte("ERROR: "+err.Error()))
			connWriteLock.Unlock()
		}
		return
	case <-time.After(30 * time.Second):
		logger.Error("K8s port-forward timeout")
		close(stopChan)
		return
	}

	// Step 4: Connect to the local port-forward and tunnel data
	logger.Info("Connecting to local port-forward", "addr", fmt.Sprintf("127.0.0.1:%d", localPort))
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 5*time.Second)
	if err != nil {
		logger.Error("Failed to connect to local port-forward", "error", err)
		close(stopChan)
		return
	}
	logger.Info("Connected to local port-forward, tunnel is fully active")
	defer localConn.Close()

	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }

	// WebSocket → Local TCP (CLI sends "PF:<base64>" text → k8s pod)
	go func() {
		defer closeDone()
		msgCount := 0
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					logger.Debug("WebSocket read ended", "error", err)
				}
				return
			}

			msgStr := string(msg)
			if msgStr == "CLOSE_CONNECTION_FROM_PEER" {
				return
			}

			if !strings.HasPrefix(msgStr, pfDataPrefix) {
				continue
			}

			data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(msgStr, pfDataPrefix))
			if err != nil {
				logger.Debug("Base64 decode error", "error", err)
				continue
			}

			msgCount++
			logger.Info("WS→TCP", "msg#", msgCount, "bytes", len(data))
			if _, err := localConn.Write(data); err != nil {
				logger.Debug("Local TCP write error", "error", err)
				return
			}
		}
	}()

	// Local TCP → WebSocket (k8s pod sends data → CLI as "PF:<base64>" text)
	go func() {
		defer closeDone()
		buf := make([]byte, 32*1024)
		respCount := 0
		for {
			n, err := localConn.Read(buf)
			if err != nil {
				if err != io.EOF {
					logger.Debug("Local TCP read ended", "error", err)
				}
				return
			}

			respCount++
			encoded := pfDataPrefix + base64.StdEncoding.EncodeToString(buf[:n])
			logger.Info("TCP→WS", "resp#", respCount, "bytes", n, "encoded_len", len(encoded))

			connWriteLock.Lock()
			err = conn.WriteMessage(websocket.TextMessage, []byte(encoded))
			connWriteLock.Unlock()

			if err != nil {
				logger.Debug("WebSocket write error", "error", err)
				return
			}
		}
	}()

	<-done

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
