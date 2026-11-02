// Package tlsgateway ends TLS for end-to-end tunnel domains inside the
// operator, so the certificate, its key and the plain traffic never leave the
// cluster. The platform's edge only reads the server name from the
// ClientHello and passes the encrypted bytes through the port-forward tunnel.
//
// Like sshgateway it has no listener of its own: the port-forward tunnel (see
// src/stream) hands each mode=tls sub-connection over as an in-process
// net.Pipe end. After the handshake every request goes through a reverse
// proxy to the tunnel's target.
//
// A ClientHello that offers only the ALPN protocol acme-tls/1 is a
// TLS-ALPN-01 validation (RFC 8737): it gets the challenge certificate of a
// running issuance and the connection ends after the handshake.
package tlsgateway

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// ACMETLSALPNProtocol is the ALPN protocol of a TLS-ALPN-01 validation.
const ACMETLSALPNProtocol = "acme-tls/1"

// handshakeTimeout bounds the TLS handshake of one sub-connection.
const handshakeTimeout = 15 * time.Second

// readHeaderTimeout bounds reading a request's header after the handshake.
const readHeaderTimeout = 30 * time.Second

// idleTimeout closes a kept-alive connection without a new request after this long, so idle connections do not
// hold the tunnel's connection slots. No ReadTimeout or WriteTimeout: they would cut long uploads, downloads and
// streams.
const idleTimeout = 2 * time.Minute

// Certificates is where the gateway takes the certificates it serves.
type Certificates interface {
	// Certificate is the certificate for hostname, or an error when there is
	// none (yet): the handshake then fails, there is no fallback certificate.
	Certificate(hostname string) (*tls.Certificate, error)
	// Challenge is the TLS-ALPN-01 challenge certificate for hostname while an
	// issuance for it runs.
	Challenge(hostname string) (*tls.Certificate, bool)
}

// CertificateManager is a Certificates source the platform can ask about a domain and tell to drop it.
type CertificateManager interface {
	Certificates
	// Status reports a domain's certificate and starts its issuance when there is none.
	Status(hostname string) CertificateStatus
	// Forget drops a domain's certificate once its tunnel is gone.
	Forget(ctx context.Context, hostname string) error
}

// Options are what Serve needs for one sub-connection.
type Options struct {
	// Hostname is the tunnel's domain; a ClientHello for any other name fails.
	Hostname string
	// ClientIP is the browser's address as the platform's edge saw it, empty
	// when unknown. It becomes X-Forwarded-For.
	ClientIP     string
	Certificates Certificates
	// Proxy forwards the requests to the target; NewProxy builds it once per tunnel.
	Proxy  http.Handler
	Logger *slog.Logger
}

// Serve ends TLS on conn and serves its requests through opts.Proxy until the
// client or the tunnel closes it.
func Serve(conn net.Conn, opts Options) {
	defer func() { _ = conn.Close() }()

	tlsConn := tls.Server(conn, &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1", ACMETLSALPNProtocol},
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return certificateFor(hello, opts)
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	err := tlsConn.HandshakeContext(ctx)
	cancel()
	if err != nil {
		opts.Logger.Info("TLS handshake failed", "hostname", opts.Hostname, "error", err)
		return
	}
	if tlsConn.ConnectionState().NegotiatedProtocol == ACMETLSALPNProtocol {
		opts.Logger.Info("answered a TLS-ALPN-01 validation", "hostname", opts.Hostname)
		return
	}

	handler := opts.Proxy
	if opts.ClientIP != "" {
		handler = withClientIP(handler, opts.ClientIP)
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(opts.Logger.Handler(), slog.LevelDebug),
	}
	_ = server.Serve(newSingleConnListener(tlsConn))
}

// certificateFor picks the certificate of a handshake: only for the tunnel's
// own hostname, the challenge certificate for a validation.
func certificateFor(hello *tls.ClientHelloInfo, opts Options) (*tls.Certificate, error) {
	if !strings.EqualFold(hello.ServerName, opts.Hostname) {
		return nil, fmt.Errorf("server name %q is not this tunnel's domain", hello.ServerName)
	}
	if slices.Contains(hello.SupportedProtos, ACMETLSALPNProtocol) {
		if cert, ok := opts.Certificates.Challenge(opts.Hostname); ok {
			return cert, nil
		}
		return nil, errors.New("no TLS-ALPN-01 validation runs for this domain")
	}
	return opts.Certificates.Certificate(opts.Hostname)
}

// withClientIP lets the proxy see the browser's address, which the pipe hides.
func withClientIP(next http.Handler, clientIP string) http.Handler {
	remoteAddr := net.JoinHostPort(clientIP, "0")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = remoteAddr
		next.ServeHTTP(w, r)
	})
}

// NewProxy builds the reverse proxy of a tunnel: every upstream connection is
// one dial into the tunnel's target, over TLS when the target speaks HTTPS.
// The Host header stays the tunnel's domain, so the application sees the
// name it is served under.
func NewProxy(dial func(ctx context.Context) (net.Conn, error), targetTLS bool, logger *slog.Logger) http.Handler {
	scheme := "http"
	if targetTLS {
		scheme = "https"
	}
	target := &url.URL{Scheme: scheme, Host: "tunnel-target"}
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = r.In.Host
			r.SetXForwarded()
			// Every request here came over TLS; the server cannot tell, since
			// the connection is wrapped to notice its close.
			r.Out.Header.Set("X-Forwarded-Proto", "https")
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dial(ctx)
			},
			// Cluster-internal targets serve self-signed certificates, as on
			// the plain port-forward path.
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			MaxIdleConnsPerHost: 16,
			IdleConnTimeout:     90 * time.Second,
		},
		// Server-sent events and other streamed answers go out as they come.
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Info("tunnel target unreachable", "host", r.Host, "error", err)
			http.Error(w, "The application behind this tunnel is not reachable right now.", http.StatusBadGateway)
		},
	}
}

// singleConnListener hands out one connection, then blocks until it is
// closed, so http.Server.Serve returns once the connection is done.
type singleConnListener struct {
	conn   net.Conn
	once   sync.Once
	closed chan struct{}
	served bool
	mu     sync.Mutex
}

func newSingleConnListener(conn net.Conn) *singleConnListener {
	l := &singleConnListener{closed: make(chan struct{})}
	l.conn = &notifyCloseConn{Conn: conn, onClose: func() { l.once.Do(func() { close(l.closed) }) }}
	return l
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if !l.served {
		l.served = true
		l.mu.Unlock()
		return l.conn, nil
	}
	l.mu.Unlock()
	<-l.closed
	return nil, net.ErrClosed
}

func (l *singleConnListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

// notifyCloseConn reports its first Close.
type notifyCloseConn struct {
	net.Conn
	onClose func()
}

func (c *notifyCloseConn) Close() error {
	err := c.Conn.Close()
	c.onClose()
	return err
}
