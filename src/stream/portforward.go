package stream

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mogenius-operator/src/sshgateway"
	"mogenius-operator/src/tlsgateway"
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

// pfModeTLS serves an end-to-end tunnel domain: each sub-connection carries
// the browser's TLS, which the in-process TLS gateway ends with a certificate
// kept in this cluster before it proxies HTTP to the target. The platform's
// edge only passed the encrypted bytes on.
const pfModeTLS = "tls"

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
	if errors.Is(err, errNoRunningPod) {
		return errNoRunningPod.Error()
	}
	return "dial failed"
}

var pfRestConfig *rest.Config
var pfClientset k8s.Interface

// pfAllowExternalHosts gates the pfKindHost path. When false, host tunnels are
// rejected — they turn the operator into a proxy into the node's LAN (SSRF).
var pfAllowExternalHosts bool

// pfTunnelCertificates are the certificates of end-to-end tunnel domains;
// nil rejects mode=tls requests (see SetupTunnelTLS).
var pfTunnelCertificates tlsgateway.CertificateManager

// SetupTunnelTLS enables mode=tls tunnels with the given certificate source.
func SetupTunnelTLS(certificates tlsgateway.CertificateManager) {
	pfTunnelCertificates = certificates
}

// TunnelCertificateRequest asks about the certificate of an end-to-end tunnel
// domain, or (Forget) drops it once the domain is removed.
type TunnelCertificateRequest struct {
	Hostname string `json:"hostname" validate:"required"`
	Forget   bool   `json:"forget"`
}

// TunnelCertificate answers the platform: the state of a domain's
// certificate, never the certificate itself.
func TunnelCertificate(ctx context.Context, request TunnelCertificateRequest) (tlsgateway.CertificateStatus, error) {
	if pfTunnelCertificates == nil {
		return tlsgateway.CertificateStatus{}, fmt.Errorf("tunnel TLS is not set up on this operator")
	}
	hostname := strings.ToLower(strings.TrimSpace(request.Hostname))
	if request.Forget {
		return tlsgateway.CertificateStatus{}, pfTunnelCertificates.Forget(ctx, hostname)
	}
	return pfTunnelCertificates.Status(hostname), nil
}

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
	// Hostname is the end-to-end tunnel domain of a mode=tls request: the only
	// server name its TLS gateway answers for.
	Hostname string `json:"hostname"`
	// Auth is how a mode=tls tunnel admits visitors: the platform cannot check
	// a password or a sign-in on traffic it never sees, so the gateway does.
	Auth tlsgateway.AuthConfig `json:"auth"`

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

// isTLS reports whether the request serves an end-to-end tunnel domain.
func (r PortForwardConnectionRequest) isTLS() bool {
	return strings.ToLower(r.Mode) == pfModeTLS
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
	if r.isTLS() && strings.TrimSpace(r.Hostname) == "" {
		return fmt.Errorf("hostname is required for mode %q", r.Mode)
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
	logger := streamLogger.With("scope", "PortForwardStreamConnection")

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
	isTLS := request.isTLS()
	if isTLS && pfTunnelCertificates == nil {
		logger.Error("Rejected end-to-end tunnel request: tunnel TLS is not set up on this operator",
			"hostname", request.Hostname)
		return
	}
	var tlsAuth *tlsgateway.Authenticator
	if isTLS {
		var err error
		if tlsAuth, err = tlsgateway.NewAuthenticator(request.Auth); err != nil {
			logger.Error("Rejected end-to-end tunnel request: its access cannot be checked", "hostname", request.Hostname, "error", err)
			return
		}
	}

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

	// An open port-forward or SSH tunnel keeps an agent sandbox from its
	// auto-stop. The hold follows the tunnel when it moves to a new pod.
	hold := newPodHold(request.Namespace)
	defer hold.release()
	if isSSH {
		hold.set(podName)
	}

	// Step 2: Connect to the stream gateway
	wsReq := request.WsConnection
	wsURL := url.URL{
		Scheme: wsReq.WebsocketScheme,
		Host:   wsReq.WebsocketHost,
		Path:   GatewayPath,
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
		HandshakeTimeout:  wsHandshakeTimeout,
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

	// Step 3: Establish how sub-connections reach the target.
	//   - host targets: dial the external host/IP directly.
	//   - k8s targets:  a kube-apiserver port-forward to the target's current
	//     pod, rebuilt on the next pod when that one goes away.
	var dialAddr string
	var forwarder *podForwarder // non-nil only for the k8s port-forward path

	if isSSH {
		// Nothing to dial: sub-connections are served by the in-process SSH
		// server via net.Pipe() (see the PFM:O handler below).
	} else if isHost {
		dialAddr = net.JoinHostPort(request.WorkloadName, strconv.Itoa(request.RemotePort))
	} else {
		forwarder = newPodForwarder(
			logger,
			func() (string, error) { return resolvePortForwardTarget(pfClientset, request) },
			func(pod string) (*podForward, error) {
				return startKubePodForward(request.Namespace, pod, request.RemotePort)
			},
			func(pod string) (bool, error) { return podAlive(pfClientset, request.Namespace, pod) },
			hold,
		)
		if err := forwarder.startOn(podName); err != nil {
			logger.Error("K8s port-forward failed", "error", err)
			wsMu.Lock()
			_ = wsConn.WriteMessage(websocket.TextMessage, []byte("ERROR: "+err.Error()))
			wsMu.Unlock()
			return
		}
		logger.Info("K8s port-forward established", "pod", podName, "remotePort", request.RemotePort)
	}

	logger.Info("Tunnel ready, waiting for client data", "target", request.WorkloadName, "dialAddr", dialAddr)

	done := make(chan struct{})
	var once sync.Once
	closeDone := func() { once.Do(func() { close(done) }) }

	if forwarder != nil {
		go forwarder.run(done)
	}

	// dialTarget opens one connection to the target, as a PFM:O does.
	dialTarget := func() (net.Conn, error) {
		if forwarder != nil {
			return forwarder.dial(done, 5*time.Second)
		}
		return net.DialTimeout("tcp", dialAddr, 5*time.Second)
	}

	// An end-to-end tunnel proxies every request of every sub-connection
	// through one reverse proxy, so upstream connections are reused.
	var tlsProxy http.Handler
	if isTLS {
		tlsProxy = tlsAuth.Wrap(tlsgateway.NewProxy(
			func(context.Context) (net.Conn, error) { return dialTarget() },
			needsTLS(request.RemotePort, request.TargetProtocol),
			logger,
		))
	}

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
		receivedAny := false
		defer func() {
			connsMu.Lock()
			delete(conns, connID)
			connsMu.Unlock()
			_ = localConn.Close()
			_ = wsSendText(pfmClosePrefix + connID)
			logger.Info("Sub-connection closed", "connID", connID)
			// Closed before the pod sent a byte: the forward may point at a
			// pod that is gone while its local port still accepts.
			if !receivedAny && forwarder != nil {
				forwarder.requestCheck()
			}
		}()

		buf := make([]byte, 32*1024)
		for {
			n, err := localConn.Read(buf)
			if n > 0 {
				receivedAny = true
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

				// End-to-end tunnels: the edge appends the browser's address
				// ("PFM:O:<connID>:<ip>"); the sub-connection carries its TLS,
				// which the in-process gateway ends.
				if isTLS {
					var clientIP string
					connID, clientIP = splitOpenFrame(after)
					tunnelEnd, gatewayEnd := net.Pipe()
					connsMu.Lock()
					conns[connID] = tunnelEnd
					connsMu.Unlock()
					go tlsgateway.Serve(gatewayEnd, tlsgateway.Options{
						Hostname:     request.Hostname,
						ClientIP:     clientIP,
						Certificates: pfTunnelCertificates,
						Proxy:        tlsProxy,
						Logger:       logger,
					})
					go readLocalAndSend(connID, tunnelEnd)
					continue
				}

				// SSH targets are served in-process: one pipe end joins the
				// tunnel like any TCP sub-connection, the other end speaks
				// SSH with the embedded gateway server.
				if isSSH {
					// Resolved per session rather than once per tunnel: after
					// a rollout the pod resolved at open time is gone.
					target, err := resolveSSHTarget(sshSession.Clientset(), request)
					if err != nil {
						logger.Error("Failed to resolve SSH target to pod", "connID", connID, "error", err)
						_ = wsSendText(pfmErrorPrefix + connID + ":no running pod for the target")
						_ = wsSendText(pfmClosePrefix + connID)
						continue
					}
					hold.set(target)

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
					go sshSession.Handle(sshEnd, request.Namespace, target, reject)
					go readLocalAndSend(connID, tunnelEnd)
					continue
				}

				localConn, err := dialTarget()
				if err != nil {
					logger.Error("Failed to dial target", "connID", connID, "target", request.WorkloadName, "error", err)
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

	if forwarder != nil {
		forwarder.close()
	}
	logger.Info("Port-forward tunnel closed", "target", request.WorkloadName, "port", request.RemotePort)
}

// splitOpenFrame splits the payload of an end-to-end tunnel's PFM:O into the
// sub-connection's ID and the browser's address. Only the edge sends the
// address, and only on mode=tls tunnels, whose connection IDs carry no colon.
func splitOpenFrame(payload string) (connID, clientIP string) {
	connID, clientIP, _ = strings.Cut(payload, ":")
	if net.ParseIP(clientIP) == nil {
		clientIP = ""
	}
	return connID, clientIP
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

	podName, ok := pickRunningPod(pods.Items)
	if !ok {
		return "", fmt.Errorf("no running pods found for selector %q", labelSelector)
	}
	return podName, nil
}
