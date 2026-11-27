package tlsgateway

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testHostname = "app.example.com"

// challengeCertificates serves a challenge certificate while challenge is set.
type challengeCertificates struct {
	*SelfSigned
	challenge *tls.Certificate
}

func (c challengeCertificates) Challenge(string) (*tls.Certificate, bool) {
	return c.challenge, c.challenge != nil
}

// serveOverPipe runs the gateway on one end of a pipe and returns the other.
func serveOverPipe(t *testing.T, target *httptest.Server, certs Certificates, clientIP string) net.Conn {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dial := func(ctx context.Context) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", target.Listener.Addr().String())
	}
	clientEnd, gatewayEnd := net.Pipe()
	go Serve(gatewayEnd, Options{
		Hostname:     testHostname,
		ClientIP:     clientIP,
		Certificates: certs,
		Proxy:        NewProxy(dial, false, logger),
		Logger:       logger,
	})
	t.Cleanup(func() { _ = clientEnd.Close() })
	return clientEnd
}

func TestServeProxiesWithTheTunnelsHostAndTheBrowsersAddress(t *testing.T) {
	var gotHost, gotForwardedFor, gotProto string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotForwardedFor, gotProto = r.Host, r.Header.Get("X-Forwarded-For"), r.Header.Get("X-Forwarded-Proto")
		_, _ = io.WriteString(w, "hello from the pod")
	}))
	defer target.Close()

	conn := serveOverPipe(t, target, NewSelfSigned(), "203.0.113.7")
	client := tls.Client(conn, &tls.Config{ServerName: testHostname, InsecureSkipVerify: true}) //nolint:gosec
	require.NoError(t, client.Handshake())
	assert.Equal(t, testHostname, client.ConnectionState().PeerCertificates[0].DNSNames[0])

	req, _ := http.NewRequest(http.MethodGet, "https://"+testHostname+"/path", nil)
	require.NoError(t, req.Write(client))
	res, err := http.ReadResponse(bufio.NewReader(client), req)
	require.NoError(t, err)
	body, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "hello from the pod", string(body))
	assert.Equal(t, testHostname, gotHost)
	assert.Equal(t, "203.0.113.7", gotForwardedFor)
	assert.Equal(t, "https", gotProto)
}

func TestServeReplacesTheAddressHeadersABrowserSends(t *testing.T) {
	tests := []struct {
		name      string
		clientIP  string
		wantFor   string
		wantReal  string
		wantFound bool
	}{
		{name: "known address", clientIP: "2001:db8::7", wantFor: "2001:db8::7", wantReal: "2001:db8::7", wantFound: true},
		{name: "unknown address", clientIP: "", wantFound: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make(chan http.Header, 1)
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got <- r.Header.Clone()
			}))
			defer target.Close()

			conn := serveOverPipe(t, target, NewSelfSigned(), tt.clientIP)
			client := tls.Client(conn, &tls.Config{ServerName: testHostname, InsecureSkipVerify: true}) //nolint:gosec
			require.NoError(t, client.Handshake())
			req, _ := http.NewRequest(http.MethodGet, "https://"+testHostname+"/", nil)
			req.Header.Set("X-Forwarded-For", "1.2.3.4")
			req.Header.Set("X-Real-IP", "1.2.3.4")
			require.NoError(t, req.Write(client))
			res, err := http.ReadResponse(bufio.NewReader(client), req)
			require.NoError(t, err)
			_ = res.Body.Close()

			header := <-got
			_, hasFor := header["X-Forwarded-For"]
			_, hasReal := header["X-Real-Ip"]
			assert.Equal(t, tt.wantFound, hasFor)
			assert.Equal(t, tt.wantFound, hasReal)
			assert.Equal(t, tt.wantFor, header.Get("X-Forwarded-For"))
			assert.Equal(t, tt.wantReal, header.Get("X-Real-IP"))
		})
	}
}

func TestServeRefusesAnotherServerName(t *testing.T) {
	target := httptest.NewServer(http.NotFoundHandler())
	defer target.Close()

	conn := serveOverPipe(t, target, NewSelfSigned(), "")
	client := tls.Client(conn, &tls.Config{ServerName: "other.example.com", InsecureSkipVerify: true}) //nolint:gosec
	assert.Error(t, client.Handshake())
}

func TestServeAnswersATLSALPNValidationOnlyWhileOneRuns(t *testing.T) {
	target := httptest.NewServer(http.NotFoundHandler())
	defer target.Close()
	challenge, err := selfSignedCertificate(testHostname, time.Now())
	require.NoError(t, err)

	running := serveOverPipe(t, target, challengeCertificates{NewSelfSigned(), challenge}, "")
	client := tls.Client(running, &tls.Config{ //nolint:gosec
		ServerName: testHostname, NextProtos: []string{ACMETLSALPNProtocol}, InsecureSkipVerify: true,
	})
	require.NoError(t, client.Handshake())
	assert.Equal(t, ACMETLSALPNProtocol, client.ConnectionState().NegotiatedProtocol)
	assert.Equal(t, challenge.Leaf.SerialNumber, client.ConnectionState().PeerCertificates[0].SerialNumber)
	// The connection ends after the handshake: no HTTP on a validation.
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err = client.Read(make([]byte, 1))
	assert.Error(t, err)

	idle := serveOverPipe(t, target, challengeCertificates{NewSelfSigned(), nil}, "")
	client = tls.Client(idle, &tls.Config{ //nolint:gosec
		ServerName: testHostname, NextProtos: []string{ACMETLSALPNProtocol}, InsecureSkipVerify: true,
	})
	err = client.Handshake()
	require.Error(t, err)
	assert.False(t, strings.Contains(err.Error(), "timeout"))
}

func TestServeAnswersAnUnreachableTargetWithBadGateway(t *testing.T) {
	target := httptest.NewServer(http.NotFoundHandler())
	target.Close()

	conn := serveOverPipe(t, target, NewSelfSigned(), "")
	client := tls.Client(conn, &tls.Config{ServerName: testHostname, InsecureSkipVerify: true}) //nolint:gosec
	require.NoError(t, client.Handshake())
	req, _ := http.NewRequest(http.MethodGet, "https://"+testHostname+"/", nil)
	require.NoError(t, req.Write(client))
	res, err := http.ReadResponse(bufio.NewReader(client), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, res.StatusCode)
}
