// Package sshgateway terminates the SSH protocol inside the operator and
// translates sessions into Kubernetes exec API calls — no sshd, no open port
// 22 and no image changes in the target container (Teleport-style gateway).
//
// It has no listener of its own: the port-forward tunnel (see src/xterm)
// hands each kind=ssh sub-connection over as an in-process net.Pipe end.
// Authentication happens before a connection ever reaches this package —
// the platform authenticates and authorizes the tunnel — so the embedded
// server accepts SSH auth "none". The SSH username selects the target
// container in multi-container pods.
//
// Authorization inside the cluster comes from impersonation: a tunnel binds
// once to the RBAC subject on the platform user's User CRD, and every
// Kubernetes call it makes — target lookup included — runs as that subject,
// so the kube-apiserver applies the user's own permissions. A user the
// cluster does not know is refused outright.
//
// Sessions the platform marks as organization- or cluster-admin instead run
// under the operator's identity. That marking cannot be verified here: it
// arrives in the same frame as the request, over the operator's authenticated
// control connection, so it is as trustworthy as that connection and no more.
// MO_SSH_GATEWAY_ALLOW_ADMIN_BYPASS turns the exception off for clusters that
// want every session impersonated, and MO_SSH_GATEWAY_ENABLED switches the
// gateway off entirely.
package sshgateway

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-operator/src/debugcontainer"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/k8sexec"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	gwLogger          *slog.Logger
	gwProvider        k8sclient.K8sClientProvider
	gwOwnNamespace    string
	gwHostSigner      ssh.Signer
	gwAllowAdminBypas bool
	// gwDebugImage is the image of the ephemeral debug container a user gets
	// with `ssh mogenius-debug@…` — for workloads whose image ships no shell.
	gwDebugImage string
)

// Setup stores dependencies and loads the cluster's persistent host key
// (created in a Secret in ownNamespace on first use, so known_hosts entries
// survive operator restarts). ownNamespace is also where the platform's
// User CRDs live.
// allowAdminBypass controls whether a session the platform marks as admin may
// run under the operator's own identity; see MO_SSH_GATEWAY_ALLOW_ADMIN_BYPASS.
func Setup(logger *slog.Logger, provider k8sclient.K8sClientProvider, ownNamespace string, allowAdminBypass bool, debugImage string) error {
	signer, err := loadOrCreateHostSigner(logger, provider.K8sClientSet(), ownNamespace)
	if err != nil {
		return fmt.Errorf("ssh host key: %w", err)
	}

	gwLogger = logger
	gwProvider = provider
	gwOwnNamespace = ownNamespace
	gwHostSigner = signer
	gwAllowAdminBypas = allowAdminBypass
	gwDebugImage = debugImage

	logger.Info("SSH gateway initialized",
		"hostKeyFingerprint", ssh.FingerprintSHA256(signer.PublicKey()),
		"allowAdminBypass", allowAdminBypass)
	return nil
}

// Ready reports whether Setup completed.
// Every dependency Setup assigns is checked here: a partially initialized
// gateway must never look ready, because Setup's failure is deliberately
// non-fatal for the operator as a whole.
func Ready() bool {
	return gwHostSigner != nil && gwProvider != nil && gwLogger != nil && gwOwnNamespace != ""
}

// ConnectionUser identifies the platform user behind a tunnel.
//
// Both fields are set by the platform API and reach the operator over its
// authenticated control connection. Note that they arrive in the same JSON
// frame as the request payload, so they carry the platform's trust, not a
// separate one — treat this as "the API decided", not as an independent
// check. Authorization inside the cluster comes from the impersonated
// identity below, which is what actually constrains a session.
type ConnectionUser struct {
	Email   string
	IsAdmin bool
}

// execClients are the Kubernetes clients a single SSH connection uses for
// every API call it triggers — impersonated for regular users so the
// kube-apiserver enforces the user's RBAC grants.
type execClients struct {
	restConfig *rest.Config
	clientset  k8s.Interface
}

// Session is one tunnel's Kubernetes access. Resolving it once per tunnel
// rather than per sub-connection matters twice over: every SSH channel of a
// tunnel then acts under the same identity, and the operator stops building
// a fresh clientset — each with its own client-side rate limiter — for every
// connection the client opens.
type Session struct {
	clients *execClients
	user    ConnectionUser
}

// NewSession binds a tunnel to the Kubernetes identity its session will act
// under. Callers must use its clients for everything they do on the tunnel's
// behalf, target resolution included — otherwise part of the work runs with
// the operator's own privileges.
func NewSession(user ConnectionUser) (*Session, error) {
	clients, err := resolveExecClients(user)
	if err != nil {
		return nil, err
	}
	return &Session{clients: clients, user: user}, nil
}

// Clientset is the Kubernetes client the tunnel's identity may use.
func (s *Session) Clientset() k8s.Interface { return s.clients.clientset }

// resolveExecClients maps the tunnel user to Kubernetes clients. The rules
// — admin bypass, impersonation via the User CRD, fail closed — live in
// k8sexec.ResolveClients so a one-off exec-request and a shell into the same
// pod are judged alike.
func resolveExecClients(user ConnectionUser) (*execClients, error) {
	clients, err := k8sexec.ResolveClients(gwLogger, gwProvider, gwOwnNamespace, gwAllowAdminBypas, k8sexec.Identity{
		Email:   user.Email,
		IsAdmin: user.IsAdmin,
	})
	if err != nil {
		return nil, err
	}
	return &execClients{restConfig: clients.RestConfig, clientset: clients.Clientset}, nil
}

// Handle runs an embedded SSH server on conn (one end of an in-process pipe
// from the tunnel) and serves channels against the given pod, all under the
// session's identity. It returns when the SSH connection ends; closing conn
// tears everything down.
// reject is called with a short, client-safe reason when the session cannot
// be served, so the tunnel can report it instead of just dropping the pipe.
func (s *Session) Handle(conn net.Conn, namespace string, podName string, reject func(reason string)) {
	user := s.user
	if reject == nil {
		reject = func(string) {}
	}
	// Ready() first: gwLogger is one of the dependencies it checks, so using
	// the logger before the guard would nil-panic in this bare goroutine and
	// take the operator process down — exactly the state the guard exists to
	// handle, since Setup's failure is non-fatal by design.
	if !Ready() {
		slog.Error("SSH gateway not initialized. Call Setup() first.", "namespace", namespace, "pod", podName)
		reject("the ssh gateway is not initialized on this cluster")
		_ = conn.Close()
		return
	}

	logger := gwLogger.With("scope", "HandleConnection", "namespace", namespace, "pod", podName, "email", user.Email)

	config := &ssh.ServerConfig{
		// The platform tunnel authenticated and authorized this connection
		// before it reached the gateway; SSH-level auth adds nothing here.
		NoClientAuth: true,
	}
	config.AddHostKey(gwHostSigner)

	sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		logger.Info("SSH handshake failed", "error", err)
		reject("ssh handshake failed")
		_ = conn.Close()
		return
	}
	defer func() { _ = sconn.Close() }()

	// Global requests (keepalives etc.) need no handling beyond a reply.
	go ssh.DiscardRequests(reqs)

	clients := s.clients

	container, available, err := resolveContainer(clients, namespace, podName, sconn.User())
	if err != nil {
		logger.Error("failed to resolve container", "user", sconn.User(), "error", err)
		// The pod may be gone, or the impersonated identity may not be
		// allowed to read it — either way the user needs to know that, not
		// just see the connection drop.
		reject(fmt.Sprintf("cannot access pod %q in namespace %q", podName, namespace))
		go rejectRemainingChannels(chans)
		return
	}
	// `ssh mogenius-debug@…` asks for the ephemeral debug container: the
	// workload container (first one, or `mogenius-debug` never matches a real
	// container) becomes the target whose PID namespace the debug shell joins.
	// Runs under the session's identity, so RBAC decides who may mutate the
	// pod spec; a rejection reaches the client as the reason, not as a dropped
	// pipe.
	var containerNote string
	if debugcontainer.IsDebugContainer(sconn.User()) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		debugName, err := debugcontainer.Ensure(ctx, clients.clientset, namespace, podName, debugcontainer.Options{
			Image:           gwDebugImage,
			TargetContainer: container,
		})
		cancel()
		if err != nil {
			logger.Error("debug container unavailable", "target", container, "error", err)
			reject(err.Error())
			go rejectRemainingChannels(chans)
			return
		}
		containerNote = fmt.Sprintf("mogenius: debug container %q attached to %q — its processes are in `ps`, its filesystem under /proc/1/root", debugName, container)
		container = debugName
	}
	logger.Info("SSH connection established", "user", sconn.User(), "container", container, "isAdmin", user.IsAdmin)

	// Multi-container pods get a one-line hint in interactive shells: which
	// container was picked, what else exists, and how to select one.
	if containerNote == "" && len(available) > 1 {
		if container == sconn.User() {
			containerNote = fmt.Sprintf("mogenius: container %q (available: %s)", container, strings.Join(available, ", "))
		} else {
			containerNote = fmt.Sprintf(
				"mogenius: container %q — username %q matched no container (available: %s; select via ssh <container>@host)",
				container, sconn.User(), strings.Join(available, ", "))
		}
	}

	for newChannel := range chans {
		switch newChannel.ChannelType() {
		case "session":
			channel, requests, err := newChannel.Accept()
			if err != nil {
				logger.Error("failed to accept session channel", "error", err)
				continue
			}
			go handleSession(logger, clients, channel, requests, namespace, podName, container, containerNote)
		case "direct-tcpip":
			// ssh -L to a pod-local port, and VS Code Remote-SSH's traffic to
			// the server it installed in the container.
			go handleDirectTCPIP(logger, clients, newChannel, namespace, podName)
		default:
			_ = newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("channel type %q not supported", newChannel.ChannelType()))
		}
	}
}

// rejectRemainingChannels drains chans, rejecting everything still queued.
//
// Abandoning the channel without draining it parks the connection's mux
// goroutine: x/crypto/ssh feeds new channels through a fixed-size buffer with
// an unguarded blocking send, and closing the connection cannot wake a
// goroutine already blocked on that send. The loop ends when the closing
// connection closes chans.
func rejectRemainingChannels(chans <-chan ssh.NewChannel) {
	for newChannel := range chans {
		_ = newChannel.Reject(ssh.ConnectionFailed, "session unavailable")
	}
}
