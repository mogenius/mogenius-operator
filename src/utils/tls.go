package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// BuildPlatformTLSConfig returns a *tls.Config for platform WebSocket connections.
// Returns nil when no custom settings are needed (gorilla uses Go's default TLS).
func BuildPlatformTLSConfig(skipVerify bool, caCertFile string) (*tls.Config, error) {
	if !skipVerify && caCertFile == "" {
		return nil, nil
	}
	cfg := &tls.Config{InsecureSkipVerify: skipVerify} //nolint:gosec
	if caCertFile != "" {
		pem, err := os.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("read CA cert file %q: %w", caCertFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no valid PEM certificates found in %q", caCertFile)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}
