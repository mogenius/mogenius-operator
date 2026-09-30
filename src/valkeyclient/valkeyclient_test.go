package valkeyclient

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	valkeyclient "github.com/valkey-io/valkey-go"
)

// newTestClient wires a valkeyClient against an in-memory server. Nodes() on a
// standalone client returns the single connection, which is the fan-out path
// scanKeys takes for a cluster.
func newTestClient(t *testing.T) (*valkeyClient, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	client, err := valkeyclient.NewClient(valkeyclient.ClientOption{
		InitAddress:  []string{mr.Addr()},
		DisableCache: true,
	})
	assert.NoError(t, err)
	t.Cleanup(client.Close)

	return &valkeyClient{
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		ctx:          context.Background(),
		valkeyClient: client,
	}, mr
}

func set(t *testing.T, mr *miniredis.Miniredis, key string) {
	t.Helper()
	assert.NoError(t, mr.Set(key, "{}"))
}

func TestKeysReturnsEveryMatchingKey(t *testing.T) {
	self, mr := newTestClient(t)

	// More keys than a single SCAN batch returns, so the cursor has to be
	// carried across round trips.
	for i := range 250 {
		set(t, mr, "resources:v1:Node::node-"+strconv.Itoa(i))
	}
	set(t, mr, "resources:v1:Pod:ns:some-pod")

	keys, err := self.Keys("resources:v1:Node:*")
	assert.NoError(t, err)
	assert.Len(t, keys, 250)

	all, err := self.Keys("*")
	assert.NoError(t, err)
	assert.Len(t, all, 251)
}

func TestKeysDeduplicatesAcrossNodes(t *testing.T) {
	self, mr := newTestClient(t)

	set(t, mr, "a:1")
	set(t, mr, "a:2")

	keys, err := self.scanKeys("a:*", 1)
	assert.NoError(t, err)

	sort.Strings(keys)
	assert.Equal(t, []string{"a:1", "a:2"}, keys)
}

func TestDeleteMultipleOnlyDeletesMatchingPatterns(t *testing.T) {
	self, mr := newTestClient(t)

	set(t, mr, "live-stats:cpu:node-a")
	set(t, mr, "live-stats:memory:node-a")
	set(t, mr, "resources:v1:Node::node-a")

	assert.NoError(t, self.DeleteMultiple("live-stats:*"))

	keys, err := self.Keys("*")
	assert.NoError(t, err)
	assert.Equal(t, []string{"resources:v1:Node::node-a"}, keys)
}

// writeSelfSignedCert generates a self-signed certificate/key pair and writes
// both as PEM files under the test's temp dir, returning their paths.
func writeSelfSignedCert(t *testing.T, dir, name string) (certFile, keyFile string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	assert.NoError(t, err)

	certFile = filepath.Join(dir, name+".crt")
	keyFile = filepath.Join(dir, name+".key")

	certOut, err := os.Create(certFile)
	assert.NoError(t, err)
	assert.NoError(t, pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}))
	assert.NoError(t, certOut.Close())

	keyDER, err := x509.MarshalECPrivateKey(key)
	assert.NoError(t, err)
	keyOut, err := os.Create(keyFile)
	assert.NoError(t, err)
	assert.NoError(t, pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	assert.NoError(t, keyOut.Close())

	return certFile, keyFile
}

func TestBuildTLSConfigFromFilesPlain(t *testing.T) {
	cfg, err := buildTLSConfigFromFiles(tlsFileConfig{serverName: "valkey.example.com"})
	assert.NoError(t, err)
	assert.Equal(t, "valkey.example.com", cfg.ServerName)
	assert.False(t, cfg.InsecureSkipVerify)
	assert.Nil(t, cfg.RootCAs)
	assert.Empty(t, cfg.Certificates)
}

func TestBuildTLSConfigFromFilesInsecureSkipVerify(t *testing.T) {
	cfg, err := buildTLSConfigFromFiles(tlsFileConfig{insecureSkipVerify: true})
	assert.NoError(t, err)
	assert.True(t, cfg.InsecureSkipVerify)
}

func TestBuildTLSConfigFromFilesLoadsCACert(t *testing.T) {
	caFile, _ := writeSelfSignedCert(t, t.TempDir(), "ca")

	cfg, err := buildTLSConfigFromFiles(tlsFileConfig{caCertFile: caFile})
	assert.NoError(t, err)
	assert.NotNil(t, cfg.RootCAs)
}

func TestBuildTLSConfigFromFilesRejectsInvalidCACert(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "ca.crt")
	assert.NoError(t, os.WriteFile(invalid, []byte("not a certificate"), 0o600))

	_, err := buildTLSConfigFromFiles(tlsFileConfig{caCertFile: invalid})
	assert.ErrorContains(t, err, "no valid certificates found")
}

func TestBuildTLSConfigFromFilesMissingCACert(t *testing.T) {
	_, err := buildTLSConfigFromFiles(tlsFileConfig{caCertFile: "/nonexistent/ca.crt"})
	assert.ErrorContains(t, err, "read CA cert")
}

func TestBuildTLSConfigFromFilesLoadsClientCert(t *testing.T) {
	certFile, keyFile := writeSelfSignedCert(t, t.TempDir(), "client")

	cfg, err := buildTLSConfigFromFiles(tlsFileConfig{clientCertFile: certFile, clientKeyFile: keyFile})
	assert.NoError(t, err)
	assert.Len(t, cfg.Certificates, 1)
}

func TestBuildTLSConfigFromFilesRequiresCertAndKeyTogether(t *testing.T) {
	certFile, _ := writeSelfSignedCert(t, t.TempDir(), "client")

	_, err := buildTLSConfigFromFiles(tlsFileConfig{clientCertFile: certFile})
	assert.ErrorContains(t, err, "must be set together")

	_, err = buildTLSConfigFromFiles(tlsFileConfig{clientKeyFile: "some.key"})
	assert.ErrorContains(t, err, "must be set together")
}

func TestBuildTLSConfigFromFilesRejectsMismatchedClientCertAndKey(t *testing.T) {
	dir := t.TempDir()
	certFile, _ := writeSelfSignedCert(t, dir, "client")
	_, keyFile := writeSelfSignedCert(t, dir, "other")

	_, err := buildTLSConfigFromFiles(tlsFileConfig{clientCertFile: certFile, clientKeyFile: keyFile})
	assert.ErrorContains(t, err, "load client certificate")
}
