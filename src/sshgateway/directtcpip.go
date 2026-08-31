package sshgateway

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// pfReadyTimeout bounds the kube-apiserver port-forward setup per channel.
const pfReadyTimeout = 30 * time.Second

// channelOpenDirectMsg is the direct-tcpip channel open payload (RFC 4254
// §7.2). DestAddr names the host the client wants to reach — inside a pod
// that is its loopback, so only DestPort matters here.
type channelOpenDirectMsg struct {
	DestAddr string
	DestPort uint32
	OrigAddr string
	OrigPort uint32
}

// forwardableDestinations are the destination hosts a channel may name. The
// forward always lands on the target pod, so anything else would silently
// connect the client somewhere other than it asked for.
var forwardableDestinations = map[string]bool{
	"": true, "localhost": true, "127.0.0.1": true, "::1": true, "0.0.0.0": true,
}

// forwardableDestination reports whether addr names the target pod itself.
func forwardableDestination(addr string) bool {
	return forwardableDestinations[strings.ToLower(addr)]
}

// handleDirectTCPIP serves one direct-tcpip channel (ssh -L, VS Code
// Remote-SSH server traffic) by bridging it to the pod port via a dedicated
// kube-apiserver port-forward. One forward per channel keeps lifecycles
// trivial; clients like VS Code hold few long-lived connections.
//
// Only pod-local destinations are served. A forward to a third host would
// have to be dialed from inside the pod, which this path cannot do — so
// `ssh -L 5432:some.service:5432` is refused rather than quietly answered by
// the pod's own port 5432, and `ssh -D` (SOCKS, a different destination per
// connection) is refused outright instead of misrouting every request.
func handleDirectTCPIP(logger *slog.Logger, clients *execClients, newChannel ssh.NewChannel, namespace string, podName string) {
	var msg channelOpenDirectMsg
	if err := ssh.Unmarshal(newChannel.ExtraData(), &msg); err != nil {
		_ = newChannel.Reject(ssh.ConnectionFailed, "invalid direct-tcpip payload")
		return
	}
	logger = logger.With("scope", "direct-tcpip", "destPort", msg.DestPort, "destAddr", msg.DestAddr)

	if !forwardableDestination(msg.DestAddr) {
		logger.Info("rejecting forward to a non-pod destination")
		_ = newChannel.Reject(ssh.Prohibited, fmt.Sprintf(
			"the mogenius ssh gateway forwards to the target pod only; %q is not reachable through it", msg.DestAddr))
		return
	}

	pfURL := clients.clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(clients.restConfig)
	if err != nil {
		logger.Error("SPDY transport failed", "error", err)
		_ = newChannel.Reject(ssh.ConnectionFailed, "port-forward transport failed")
		return
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, pfURL)

	stopChan := make(chan struct{})
	readyChan := make(chan struct{})
	// "0:<port>" lets the forwarder pick a free local port.
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("0:%d", msg.DestPort)}, stopChan, readyChan, io.Discard, io.Discard)
	if err != nil {
		logger.Error("port-forward create failed", "error", err)
		_ = newChannel.Reject(ssh.ConnectionFailed, "port-forward create failed")
		return
	}

	errChan := make(chan error, 1)
	go func() { errChan <- fw.ForwardPorts() }()

	select {
	case <-readyChan:
	case err := <-errChan:
		logger.Info("port-forward failed", "error", err)
		_ = newChannel.Reject(ssh.ConnectionFailed, fmt.Sprintf("port-forward to pod port %d failed", msg.DestPort))
		return
	case <-time.After(pfReadyTimeout):
		close(stopChan)
		_ = newChannel.Reject(ssh.ConnectionFailed, "port-forward timeout")
		return
	}
	defer close(stopChan)

	ports, err := fw.GetPorts()
	if err != nil || len(ports) == 0 {
		logger.Error("port-forward local port unknown", "error", err)
		_ = newChannel.Reject(ssh.ConnectionFailed, "port-forward setup failed")
		return
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", ports[0].Local), 5*time.Second)
	if err != nil {
		logger.Error("dial forwarded port failed", "error", err)
		_ = newChannel.Reject(ssh.ConnectionFailed, "dial forwarded port failed")
		return
	}
	defer func() { _ = conn.Close() }()

	channel, requests, err := newChannel.Accept()
	if err != nil {
		logger.Error("failed to accept direct-tcpip channel", "error", err)
		return
	}
	defer func() { _ = channel.Close() }()
	go ssh.DiscardRequests(requests)

	// Bridge both directions; propagate EOF as a half-close so protocols
	// that shut down one side first (e.g. HTTP) still drain cleanly.
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(channel, conn)
		_ = channel.CloseWrite()
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(conn, channel)
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}()

	// Return once either direction ends, and let the deferred Close calls
	// unblock the other one. Waiting for both would pin this channel's
	// port-forward, its listeners and roughly ten goroutines for the
	// operator's lifetime whenever one side half-closes — the normal case
	// when the ssh client goes away while the pod keeps its socket open.
	<-done
}
