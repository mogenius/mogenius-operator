package sshgateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	mocrds "mogenius-operator/src/crds"
	"mogenius-operator/src/k8sclient"
)

// startTestGateway runs handleSession behind a real SSH server and returns a
// connected client. The transport is a loopback socket rather than the
// net.Pipe the tunnel uses: both handshake sides write before they read, and
// an unbuffered pipe deadlocks on that in-process. The Kubernetes clients are
// fakes — every exec fails, which is enough to exercise the request loop
// without a cluster.
func startTestGateway(t *testing.T) *ssh.Client {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	// A real clientset pointed at a dead port: every exec fails fast with
	// "connection refused". The generated fake is unusable here because its
	// RESTClient() returns a nil *rest.RESTClient.
	deadConfig := &rest.Config{Host: "https://127.0.0.1:1"}
	clientset, err := kubernetes.NewForConfig(deadConfig)
	if err != nil {
		t.Fatalf("clientset: %v", err)
	}
	clients := &execClients{restConfig: deadConfig, clientset: clientset}

	go func() {
		serverEnd, err := listener.Accept()
		if err != nil {
			return
		}
		config := &ssh.ServerConfig{NoClientAuth: true}
		config.AddHostKey(signer)
		sconn, chans, reqs, err := ssh.NewServerConn(serverEnd, config)
		if err != nil {
			return
		}
		defer func() { _ = sconn.Close() }()
		go ssh.DiscardRequests(reqs)

		for newChannel := range chans {
			if newChannel.ChannelType() != "session" {
				_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported")
				continue
			}
			channel, requests, err := newChannel.Accept()
			if err != nil {
				return
			}
			go handleSession(slog.Default(), clients, channel, requests, "ns", "pod", "app", "")
		}
	}()

	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		User:            "app",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		t.Fatalf("client handshake: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// A session whose payload is running must keep servicing the SSH request
// queue. x/crypto/ssh delivers requests from the connection's single mux
// goroutine with an unguarded blocking send into a fixed-size buffer, so a
// handler that stops reading deadlocks the whole connection — not just that
// session — once the buffer fills. Terminal resize is the everyday trigger:
// one window-change per drag step.
func TestSessionKeepsServicingRequestsWhilePayloadRuns(t *testing.T) {
	client := startTestGateway(t)

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	if err := session.RequestPty("xterm", 24, 80, ssh.TerminalModes{}); err != nil {
		t.Fatalf("RequestPty: %v", err)
	}
	// Starts the sftp subsystem, which blocks serving until the channel ends
	// — a payload that is genuinely in flight.
	if err := session.RequestSubsystem("sftp"); err != nil {
		t.Fatalf("RequestSubsystem: %v", err)
	}

	// Well past the 16-slot buffer that the previous implementation filled.
	resized := make(chan error, 1)
	go func() {
		for i := 0; i < 64; i++ {
			if err := session.WindowChange(24+i%10, 80); err != nil {
				resized <- err
				return
			}
		}
		resized <- nil
	}()

	select {
	case err := <-resized:
		if err != nil {
			t.Fatalf("WindowChange failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("window-change stalled: the session stopped draining its request queue and deadlocked the connection")
	}

	// The connection as a whole must still work afterwards.
	probe, err := client.NewSession()
	if err != nil {
		t.Fatalf("connection unusable after resizes: %v", err)
	}
	_ = probe.Close()
}

// Filecmd must not dereference the parsed attributes: pkg/sftp discards the
// unmarshal error and hands back nil for a truncated attribute blob, so this
// used to panic and take the operator process down.
func TestFilecmdSetstatWithMalformedAttributes(t *testing.T) {
	handlers := &sftpHandlers{logger: slog.Default(), namespace: "ns", pod: "pod", container: "app"}

	req := sftp.NewRequest("Setstat", "/tmp/x")
	req.Flags = 0x00000004 // SSH_FILEXFER_ATTR_PERMISSIONS
	req.Attrs = []byte{}   // truncated

	if !req.AttrFlags().Permissions || req.Attributes() != nil {
		t.Skip("pkg/sftp no longer yields nil attributes for this input")
	}

	// nil clients on purpose: a correct implementation reports the malformed
	// packet before it ever reaches the Kubernetes call.
	if err := handlers.Filecmd(req); err == nil {
		t.Fatal("Filecmd accepted a malformed Setstat instead of reporting it")
	}
}

// WithImpersonate exits the process on an unknown subject kind, and the
// subject comes from a User CRD whose schema does not constrain it.
func TestValidateImpersonationSubject(t *testing.T) {
	valid := []rbacv1.Subject{
		{Kind: "User", Name: "jane", APIGroup: rbacv1.GroupName},
		{Kind: "Group", Name: "ops", APIGroup: rbacv1.GroupName},
		{Kind: "ServiceAccount", Name: "runner", Namespace: "mogenius"},
	}
	for _, subject := range valid {
		if err := validateImpersonationSubject(subject); err != nil {
			t.Errorf("valid %s subject rejected: %v", subject.Kind, err)
		}
	}

	invalid := map[string]rbacv1.Subject{
		"empty kind":               {Name: "jane", APIGroup: rbacv1.GroupName},
		"lowercase kind":           {Kind: "user", Name: "jane", APIGroup: rbacv1.GroupName},
		"unknown kind":             {Kind: "Robot", Name: "jane"},
		"user without name":        {Kind: "User", APIGroup: rbacv1.GroupName},
		"user with wrong apigroup": {Kind: "User", Name: "jane"},
		"user with namespace":      {Kind: "User", Name: "jane", APIGroup: rbacv1.GroupName, Namespace: "default"},
		"sa without namespace":     {Kind: "ServiceAccount", Name: "runner"},
		"sa with apigroup":         {Kind: "ServiceAccount", Name: "runner", Namespace: "mogenius", APIGroup: rbacv1.GroupName},
	}
	for name, subject := range invalid {
		if err := validateImpersonationSubject(subject); err == nil {
			t.Errorf("%s was accepted; WithImpersonate would exit the process", name)
		}
	}
}

// stubProvider stands in for the operator's Kubernetes client provider so the
// bypass path can be exercised without a cluster.
type stubProvider struct {
	config    *rest.Config
	clientset *kubernetes.Clientset
}

func (p *stubProvider) K8sClientSet() *kubernetes.Clientset          { return p.clientset }
func (p *stubProvider) DynamicClient() *dynamic.DynamicClient        { return nil }
func (p *stubProvider) MogeniusClientSet() *mocrds.MogeniusClientSet { return nil }
func (p *stubProvider) RunsInCluster() bool                          { return true }
func (p *stubProvider) ClientConfig() *rest.Config                   { return p.config }
func (p *stubProvider) WithImpersonate(rbacv1.Subject) (k8sclient.K8sClientProvider, error) {
	return p, nil
}

// With the admin bypass switched off, a session the platform marked as admin
// must fall through to impersonation instead of receiving the operator's own
// clients. An admin without an identity the cluster knows is then refused,
// where previously the flag alone was enough.
func TestAdminBypassCanBeDisabled(t *testing.T) {
	previousLogger, previousProvider, previousBypass := gwLogger, gwProvider, gwAllowAdminBypas
	t.Cleanup(func() {
		gwLogger, gwProvider, gwAllowAdminBypas = previousLogger, previousProvider, previousBypass
	})

	deadConfig := &rest.Config{Host: "https://127.0.0.1:1"}
	clientset, err := kubernetes.NewForConfig(deadConfig)
	if err != nil {
		t.Fatalf("clientset: %v", err)
	}
	gwLogger = slog.Default()
	gwProvider = &stubProvider{config: deadConfig, clientset: clientset}

	admin := ConnectionUser{IsAdmin: true}

	gwAllowAdminBypas = true
	clients, err := resolveExecClients(admin)
	if err != nil {
		t.Fatalf("with the bypass enabled an admin session was refused: %v", err)
	}
	if clients.restConfig != deadConfig {
		t.Error("the bypass did not hand back the operator's own client config")
	}

	// Same input, bypass off: the flag alone is no longer enough, and an
	// admin the cluster cannot identify is refused.
	gwAllowAdminBypas = false
	if _, err := resolveExecClients(admin); err == nil {
		t.Fatal("with the bypass disabled an admin without a known identity was accepted")
	}
}
