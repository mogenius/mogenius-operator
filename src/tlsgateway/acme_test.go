package tlsgateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "mogenius"

func newTestACME(t *testing.T, objects ...corev1.Secret) (*ACME, *fake.Clientset) {
	t.Helper()
	clientset := fake.NewClientset()
	for i := range objects {
		_, err := clientset.CoreV1().Secrets(testNamespace).Create(context.Background(), &objects[i], metav1.CreateOptions{})
		require.NoError(t, err)
	}
	// An unreachable ACME server: an issuance that starts fails, nothing leaves the test.
	acme := NewACME(slog.New(slog.NewTextHandler(io.Discard, nil)), clientset, testNamespace, ACMEConfig{Server: "http://127.0.0.1:1/directory"})
	return acme, clientset
}

func tlsSecret(t *testing.T, hostname string, now time.Time) corev1.Secret {
	t.Helper()
	cert, err := selfSignedCertificate(hostname, now)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	require.NoError(t, err)
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        certSecretName(hostname),
			Namespace:   testNamespace,
			Labels:      map[string]string{acmeManagedLabel: "true"},
			Annotations: map[string]string{acmeHostnameAnnotation: hostname},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}),
			corev1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		},
	}
}

func TestACMEServesTheCertificateOfAnEarlierIssuance(t *testing.T) {
	acme, _ := newTestACME(t, tlsSecret(t, "app.example.com", time.Now()))

	assert.Equal(t, CertificateReady, acme.Status("App.Example.com").State)
	cert, err := acme.Certificate("app.example.com")
	require.NoError(t, err)
	assert.Equal(t, "app.example.com", cert.Leaf.DNSNames[0])
}

func TestACMEStartsAnIssuanceAndReportsItsFailure(t *testing.T) {
	acme, _ := newTestACME(t)

	assert.Equal(t, CertificateIssuing, acme.Status("app.example.com").State)
	_, err := acme.Certificate("app.example.com")
	assert.Error(t, err, "no certificate while it is being issued: the handshake fails, nothing falls back")

	require.Eventually(t, func() bool { return acme.Status("app.example.com").State == CertificateError }, 10*time.Second, 50*time.Millisecond)
	status := acme.Status("app.example.com")
	assert.NotEmpty(t, status.Message)
	// A failed issuance is retried only after a while, not on every read.
	assert.Equal(t, CertificateError, acme.Status("app.example.com").State)
}

func TestACMEForgetDropsTheCertificateSecret(t *testing.T) {
	acme, clientset := newTestACME(t, tlsSecret(t, "app.example.com", time.Now()))
	require.Equal(t, CertificateReady, acme.Status("app.example.com").State)

	require.NoError(t, acme.Forget(context.Background(), "app.example.com"))

	_, err := clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), certSecretName("app.example.com"), metav1.GetOptions{})
	assert.Error(t, err)
	assert.NoError(t, acme.Forget(context.Background(), "app.example.com"), "forgetting twice is fine")
}

func TestACMEAnswersAValidationThroughTheGateway(t *testing.T) {
	acme, _ := newTestACME(t)
	provider := alpnProvider{acme}
	require.NoError(t, provider.Present(context.Background(), "app.example.com", "token", "key-authorization"))

	clientEnd, gatewayEnd := net.Pipe()
	defer func() { _ = clientEnd.Close() }()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go Serve(gatewayEnd, Options{Hostname: "app.example.com", Certificates: acme, Proxy: http.NotFoundHandler(), Logger: logger})
	client := tls.Client(clientEnd, &tls.Config{ //nolint:gosec
		ServerName: "app.example.com", NextProtos: []string{ACMETLSALPNProtocol}, InsecureSkipVerify: true,
	})
	require.NoError(t, client.Handshake())
	assert.Equal(t, ACMETLSALPNProtocol, client.ConnectionState().NegotiatedProtocol)

	require.NoError(t, provider.CleanUp(context.Background(), "app.example.com", "token", "key-authorization"))
	_, ok := acme.Challenge("app.example.com")
	assert.False(t, ok)
}

func TestCertificateSecretNameIsStableAndShort(t *testing.T) {
	name := certSecretName("App.Example.com")
	assert.Equal(t, name, certSecretName("app.example.com"))
	assert.LessOrEqual(t, len(name), 63)
	assert.NotEqual(t, name, certSecretName("other.example.com"))
}

func TestCAARecordNamesTheAccountOnlyForLetsEncrypt(t *testing.T) {
	acme, _ := newTestACME(t)
	acme.accountURI = "https://acme-v02.api.letsencrypt.org/acme/acct/123"
	assert.Empty(t, acme.caaRecord(), "a test server's account means nothing to a CAA record")

	acme.config.Server = LetsEncryptProduction
	assert.Equal(t, `0 issue "letsencrypt.org; accounturi=https://acme-v02.api.letsencrypt.org/acme/acct/123; validationmethods=tls-alpn-01"`, acme.caaRecord())
	acme.dnsKind = "cloudflare"
	assert.Contains(t, acme.caaRecord(), "validationmethods=dns-01")
}

func TestDecodeRegistrationReadsBothLegoFormats(t *testing.T) {
	const accountURL = "https://acme-v02.api.letsencrypt.org/acme/acct/123"

	current := decodeRegistration([]byte(`{"status":"valid","accountURL":"` + accountURL + `"}`))
	require.NotNil(t, current)
	assert.Equal(t, accountURL, current.Location)

	legacy := decodeRegistration([]byte(`{"body":{"status":"valid"},"uri":"` + accountURL + `"}`))
	require.NotNil(t, legacy, "a registration stored by lego v4 keeps its account")
	assert.Equal(t, accountURL, legacy.Location)
	assert.Equal(t, "valid", legacy.Status)

	assert.Nil(t, decodeRegistration([]byte(`{"status":"valid"}`)), "without an account URL the account registers again")
	assert.Nil(t, decodeRegistration([]byte(`not json`)))
}
