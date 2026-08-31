package xterm

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mogenius-operator/src/sshgateway"
	"mogenius-operator/src/utils"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	// pfmErrorPrefix reports a failed sub-connection with a reason:
	// "PFM:E:<connID>:<reason>". Sent before the compat pfmClosePrefix frame so
	// API versions that don't know PFM:E still tear the sub-connection down.
	pfmErrorPrefix = "PFM:E:"
)

// pfKindHost is the target kind for a non-Kubernetes target: the operator dials
// the given host/IP:port directly on its own network instead of resolving a pod
// and tunneling through the kube-apiserver.
const pfKindHost = "host"

// pfModeSSH selects the embedded SSH gateway: instead of dialing anything,
// each sub-connection is handed to an in-process SSH server that translates
// the session into Kubernetes exec API calls (no sshd in the container).
// remotePort is unused on this path, and Kind keeps its usual meaning — the
// resource kind of the target.
const pfModeSSH = "ssh"

// pfKindSSH is the pre-mode spelling of the same request, where "ssh" was put
// in Kind for want of another field. Accepted so clients released before the
// mode existed keep working; those send the target kind inside WorkloadName
// as "<kind>/<name>".
//
// Deprecated: set Mode instead and leave Kind to the resource kind.
const pfKindSSH = "ssh"

// pfDialErrorReason classifies a dial error into a short, address-free reason
// for the PFM:E frame. The raw error string contains the internal target
// address, which must not surface on the public tunnel error page.
func pfDialErrorReason(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "dial timeout: target host not responding"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection refused: target port not open"
	}
	if errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH) {
		return "no route to target host"
	}
	return "dial failed"
}

var pfRestConfig *rest.Config
var pfClientset k8s.Interface

// pfAllowExternalHosts gates the pfKindHost path. When false, host tunnels are
// rejected — they turn the operator into a proxy into the node's LAN (SSRF).
var pfAllowExternalHosts bool

// pfSSHGatewayEnabled gates the pfKindSSH path. An SSH session grants an
// interactive shell, read/write access to the container filesystem and port
// forwarding inside the pod, so a cluster must be able to switch it off.
var pfSSHGatewayEnabled bool

// SetupPortForward stores K8s client dependencies needed for port-forward tunneling.
// allowExternalHosts enables dialing arbitrary hosts/IPs (kind=host);
// sshGatewayEnabled enables SSH sessions (kind=ssh).
func SetupPortForward(restConfig *rest.Config, clientset k8s.Interface, allowExternalHosts bool, sshGatewayEnabled bool) {
	pfRestConfig = restConfig
	pfClientset = clientset
	pfAllowExternalHosts = allowExternalHosts
	pfSSHGatewayEnabled = sshGatewayEnabled
}

type PortForwardConnectionRequest struct {
	// What a request must carry depends on where the tunnel ends up, and a
	// struct tag cannot express that — so Namespace, RemotePort and Kind are
	// all checked in validate() instead. A blanket `required` on any of them
	// rejects a legitimate ssh request before the handler ever sees it, with
	// nothing the user can act on.
	Namespace  string `json:"namespace"`
	RemotePort int    `json:"remotePort"`
	// Mode is what the operator does once the target is reached; empty means
	// plain TCP forwarding. Kind stays the Kubernetes resource kind, and is
	// optional for mode=ssh — the hostname grammar lets callers omit it, and
	// the target is then resolved by name across kinds.
	Mode           string              `json:"mode"`
	Kind           string              `json:"kind"`
	WorkloadName   string              `json:"workloadName" validate:"required"`
	WsConnection   WsConnectionRequest `json:"wsConnectionRequest" validate:"required"`
	TargetProtocol string              `json:"targetProtocol"` // "http", "https", or empty for auto-detect

	// IsAdmin is the platform's judgement that the requester is an
	// organization admin, and IsClusterAdmin that an API key carries a
	// CLUSTER_ADMIN scope for this cluster. Both let a session skip
	// impersonation and act as the operator.
	//
	// They are not independently verifiable here. The whole datagram —
	// payload and user alike — arrives over the operator's authenticated
	// control connection, so these flags are exactly as trustworthy as that
	// connection and nothing more; anyone able to speak on it can already
	// drive the operator directly. MO_SSH_GATEWAY_ALLOW_ADMIN_BYPASS exists
	// for clusters that would rather not extend that trust to shell access.
	IsAdmin        bool `json:"isAdmin"`
	IsClusterAdmin bool `json:"isClusterAdmin"`
	// UserEmail identifies the platform user to impersonate on kind=ssh. It
	// is filled at the dispatch site from the datagram's user field rather
	// than decoded from the payload, so the two cannot drift apart within a
	// handler — but see above: both come from the same frame, so this is
	// about consistency, not about a trust boundary.
	UserEmail string `json:"-"`
}

// isSSH reports whether the request opens an SSH session, accepting both the
// mode field and the legacy kind spelling.
func (r PortForwardConnectionRequest) isSSH() bool {
	return strings.ToLower(r.Mode) == pfModeSSH || strings.ToLower(r.Kind) == pfKindSSH
}

// validate applies the per-request requirements the struct tags cannot
// express: what is required depends on where the tunnel ends up.
func (r PortForwardConnectionRequest) validate() error {
	if r.isSSH() {
		// An ssh request dials nothing and may omit the kind; only the target
		// itself is indispensable.
		if strings.TrimSpace(r.Namespace) == "" {
			return fmt.Errorf("namespace is required for mode %q", r.Mode)
		}
		return nil
	}

	if strings.TrimSpace(r.Kind) == "" {
		return fmt.Errorf("kind is required")
	}
	if strings.ToLower(r.Kind) != pfKindHost && strings.TrimSpace(r.Namespace) == "" {
		return fmt.Errorf("namespace is required for kind %q", r.Kind)
	}
	if r.RemotePort <= 0 {
		return fmt.Errorf("remotePort is required for kind %q", r.Kind)
	}
	return nil
}

// needsTLS determines if the connection to the target port should use TLS.
// Auto-detects based on port if targetProtocol is empty.
func needsTLS(remotePort int, targetProtocol string) bool {
	switch strings.ToLower(targetProtocol) {
	case "https":
		return true
	case "http":
		return false
	default:
		// Auto-detect based on common HTTPS ports
		return remotePort == 443 || remotePort == 8443 || remotePort == 6443
	}
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

	if err := request.validate(); err != nil {
		logger.Error("Rejected port-forward request", "error", err)
		return
	}

	isHost := strings.ToLower(request.Kind) == pfKindHost
	isSSH := request.isSSH()

	// Step 1: Resolve target. Kubernetes kinds resolve to a pod; host targets
	// are dialed directly and skip pod resolution + the kube-apiserver forward.
	var podName string
	// sshSession carries the Kubernetes identity for the whole tunnel; it is
	// resolved before anything is looked up so the lookups run as the user
	// too, not just the session that follows them.
	var sshSession *sshgateway.Session
	if isSSH {
		if !pfSSHGatewayEnabled {
			logger.Error("Rejected SSH request: the SSH gateway is disabled. Set SSH_GATEWAY_ENABLED=true on the operator to enable it.",
				"namespace", request.Namespace, "target", request.WorkloadName)
			return
		}
		if !sshgateway.Ready() {
			logger.Error("SSH gateway not initialized. Call sshgateway.Setup() first.")
			return
		}

		var err error
		sshSession, err = sshgateway.NewSession(sshgateway.ConnectionUser{
			Email: request.UserEmail,
			// Both flags are the platform's decision, delivered over the
			// operator's authenticated control connection; the gateway can
			// be configured to ignore them and impersonate regardless.
			IsAdmin: request.IsAdmin || request.IsClusterAdmin,
		})
		if err != nil {
			logger.Error("Rejected SSH request: cannot bind the tunnel to a Kubernetes identity", "error", err)
			return
		}

		// Deliberately the session's client, not the operator's: resolving a
		// name is itself a read, and doing it with operator privileges would
		// answer "does this exist?" for namespaces the user cannot see.
		podName, err = resolveSSHTarget(sshSession.Clientset(), request)
		if err != nil {
			logger.Error("Failed to resolve SSH target to pod", "error", err)
			return
		}
		logger.Info("Resolved SSH target", "pod", podName, "namespace", request.Namespace)
	} else if isHost {
		if !pfAllowExternalHosts {
			logger.Error("Rejected host port-forward request: external hosts are disabled. Set PORT_FORWARD_ALLOW_EXTERNAL_HOSTS=true on the operator to enable.",
				"host", request.WorkloadName, "port", request.RemotePort)
			return
		}
		if strings.TrimSpace(request.WorkloadName) == "" {
			logger.Error("Rejected host port-forward request: empty host")
			return
		}
		logger.Info("Host target: dialing external host directly", "host", request.WorkloadName, "port", request.RemotePort)
	} else {
		if pfRestConfig == nil || pfClientset == nil {
			logger.Error("Port-forward not initialized. Call SetupPortForward() first.")
			return
		}

		var err error
		podName, err = resolvePortForwardTarget(pfClientset, request)
		if err != nil {
			logger.Error("Failed to resolve target to pod", "error", err)
			return
		}
		logger.Info("Resolved target", "pod", podName, "namespace", request.Namespace, "port", request.RemotePort)
	}

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
		_ = wsConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		wsMu.Unlock()
		_ = wsConn.Close()
	}()

	// Wait for ack-ready from stream gateway
	_ = wsConn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, ackMsg, err := wsConn.ReadMessage()
	if err != nil {
		logger.Error("Failed to receive ack from stream gateway", "error", err)
		return
	}
	logger.Info("Stream gateway ack received", "msg", string(ackMsg))
	_ = wsConn.SetReadDeadline(time.Time{})

	// Step 3: Establish the target address to dial per sub-connection.
	//   - host targets: dial the external host/IP directly.
	//   - k8s targets:  start a kube-apiserver port-forward on a random local
	//     port and dial 127.0.0.1:<localPort>.
	var dialAddr string
	var stopChan chan struct{} // non-nil only for the k8s port-forward path

	if isSSH {
		// Nothing to dial: sub-connections are served by the in-process SSH
		// server via net.Pipe() (see the PFM:O handler below).
	} else if isHost {
		dialAddr = net.JoinHostPort(request.WorkloadName, strconv.Itoa(request.RemotePort))
	} else {
		localListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			logger.Error("Failed to create local listener", "error", err)
			return
		}
		localPort := localListener.Addr().(*net.TCPAddr).Port
		_ = localListener.Close()

		stopChan = make(chan struct{})
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
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte("ERROR: "+err.Error()))
			wsMu.Unlock()
			return
		case <-time.After(30 * time.Second):
			logger.Error("K8s port-forward timeout")
			close(stopChan)
			return
		}

		dialAddr = fmt.Sprintf("127.0.0.1:%d", localPort)
	}

	logger.Info("Tunnel ready, waiting for client data", "dialAddr", dialAddr)

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
			_ = localConn.Close()
			_ = wsSendText(pfmClosePrefix + connID)
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
			if after, ok := strings.CutPrefix(msgStr, pfmOpenPrefix); ok {
				connID := after
				logger.Info("Opening sub-connection", "connID", connID)

				// SSH targets are served in-process: one pipe end joins the
				// tunnel like any TCP sub-connection, the other end speaks
				// SSH with the embedded gateway server.
				if isSSH {
					tunnelEnd, sshEnd := net.Pipe()
					connsMu.Lock()
					conns[connID] = tunnelEnd
					connsMu.Unlock()

					// Report a refused SSH session the same way the TCP
					// branch reports a failed dial, so the client sees a
					// reason instead of a bare closed pipe.
					reject := func(reason string) {
						_ = wsSendText(pfmErrorPrefix + connID + ":" + reason)
					}
					go sshSession.Handle(sshEnd, request.Namespace, podName, reject)
					go readLocalAndSend(connID, tunnelEnd)
					continue
				}

				localConn, err := net.DialTimeout("tcp", dialAddr, 5*time.Second)
				if err != nil {
					logger.Error("Failed to dial target", "connID", connID, "dialAddr", dialAddr, "error", err)
					_ = wsSendText(pfmErrorPrefix + connID + ":" + pfDialErrorReason(err))
					_ = wsSendText(pfmClosePrefix + connID)
					continue
				}

				// Wrap with TLS if target expects HTTPS
				finalConn := localConn
				if needsTLS(request.RemotePort, request.TargetProtocol) {
					logger.Info("Wrapping connection with TLS", "connID", connID, "port", request.RemotePort)
					// Cluster-internal self-signed certs are expected, so we skip
					// verification. TODO: for kind=host (external) targets, consider
					// verifying the certificate / making this configurable.
					tlsConn := tls.Client(localConn, &tls.Config{
						InsecureSkipVerify: true,
					})
					if err := tlsConn.Handshake(); err != nil {
						logger.Error("TLS handshake failed", "connID", connID, "error", err)
						_ = localConn.Close()
						_ = wsSendText(pfmErrorPrefix + connID + ":tls handshake failed: " + err.Error())
						_ = wsSendText(pfmClosePrefix + connID)
						continue
					}
					finalConn = tlsConn
					logger.Info("TLS handshake successful", "connID", connID)
				}

				connsMu.Lock()
				conns[connID] = finalConn
				connsMu.Unlock()

				go readLocalAndSend(connID, finalConn)
				continue
			}

			// PFM:C:<connID> — CLI closed a sub-connection
			if after, ok := strings.CutPrefix(msgStr, pfmClosePrefix); ok {
				connID := after
				logger.Info("Remote closed sub-connection", "connID", connID)
				connsMu.Lock()
				if c, ok := conns[connID]; ok {
					_ = c.Close()
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
		_ = c.Close()
		delete(conns, id)
	}
	connsMu.Unlock()

	if stopChan != nil {
		close(stopChan)
	}
	logger.Info("Port-forward tunnel closed", "target", request.WorkloadName, "port", request.RemotePort)
}

// resolveSSHTarget resolves an SSH request's target to a running pod.
//
// With a kind it resolves that kind directly. Without one — the hostname
// grammar makes it optional — it tries the kinds in a fixed order: an exact
// pod match first, then the selector-based ones.
func resolveSSHTarget(clientset k8s.Interface, req PortForwardConnectionRequest) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	kind, name := strings.ToLower(req.Kind), req.WorkloadName
	// Pre-mode clients put "ssh" in Kind and the real kind in WorkloadName as
	// "<kind>/<name>"; Kubernetes names cannot contain slashes, so this is
	// unambiguous. Deprecated together with pfKindSSH.
	if legacyKind, legacyName, ok := strings.Cut(req.WorkloadName, "/"); ok {
		kind, name = strings.ToLower(legacyKind), legacyName
	}

	if kind != "" && kind != pfKindSSH {
		if kind == "pod" {
			if _, err := clientset.CoreV1().Pods(req.Namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
				return "", fmt.Errorf("pod %q not found in namespace %q: %w", name, req.Namespace, err)
			}
			return name, nil
		}
		kindReq := req
		kindReq.Kind = kind
		kindReq.WorkloadName = name
		return resolvePortForwardTarget(clientset, kindReq)
	}

	req.WorkloadName = name
	if _, err := clientset.CoreV1().Pods(req.Namespace).Get(ctx, req.WorkloadName, metav1.GetOptions{}); err == nil {
		return req.WorkloadName, nil
	}

	for _, kind := range []string{"deployment", "statefulset", "daemonset", "service"} {
		kindReq := req
		kindReq.Kind = kind
		if podName, err := resolvePortForwardTarget(clientset, kindReq); err == nil {
			return podName, nil
		}
	}

	return "", fmt.Errorf("no pod, deployment, statefulset, daemonset or service named %q with running pods in namespace %q", req.WorkloadName, req.Namespace)
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
