package sshgateway

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	hostKeySecretName = "mogenius-ssh-gateway-host-key"
	hostKeySecretKey  = "hostKey"

	hostKeyManagedByLabelKey   = "app.kubernetes.io/managed-by"
	hostKeyManagedByLabelValue = "mogenius"
)

// loadOrCreateHostSigner returns the cluster's persistent SSH host key,
// creating and storing a fresh ed25519 key in a Secret on first use. A stable
// host key is what keeps clients' known_hosts entries valid across operator
// restarts. Failures fall back to an ephemeral key (with a warning) so SSH
// keeps working even if the Secret is unreachable.
func loadOrCreateHostSigner(logger *slog.Logger, clientset k8s.Interface, namespace string) (ssh.Signer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	secrets := clientset.CoreV1().Secrets(namespace)

	secret, err := secrets.Get(ctx, hostKeySecretName, metav1.GetOptions{})
	if err == nil {
		if pemBytes, ok := secret.Data[hostKeySecretKey]; ok && len(pemBytes) > 0 {
			signer, parseErr := ssh.ParsePrivateKey(pemBytes)
			if parseErr == nil {
				return signer, nil
			}
			logger.Warn("stored SSH host key is unparsable — regenerating", "error", parseErr)
		}
	} else if !apierrors.IsNotFound(err) {
		return ephemeralHostSigner(logger, fmt.Errorf("get secret %q: %w", hostKeySecretName, err))
	}

	// Generate a fresh key and persist it (upsert: a concurrent replica may
	// have won the create race — then its key is authoritative).
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ssh host key: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "mogenius-ssh-gateway")
	if err != nil {
		return nil, fmt.Errorf("marshal ssh host key: %w", err)
	}
	pemBytes := pem.EncodeToMemory(block)

	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostKeySecretName,
			Namespace: namespace,
			Labels:    map[string]string{hostKeyManagedByLabelKey: hostKeyManagedByLabelValue},
		},
		Data: map[string][]byte{hostKeySecretKey: pemBytes},
	}
	_, err = secrets.Create(ctx, newSecret, metav1.CreateOptions{})
	if err == nil {
		return ssh.NewSignerFromKey(priv)
	}
	if !apierrors.IsAlreadyExists(err) {
		return ephemeralHostSigner(logger, fmt.Errorf("create secret %q: %w", hostKeySecretName, err))
	}

	// Secret exists (create race or unparsable data from a previous run):
	// prefer the stored key if it parses now, otherwise overwrite with ours.
	var signer ssh.Signer
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, getErr := secrets.Get(ctx, hostKeySecretName, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}
		if pemStored, ok := current.Data[hostKeySecretKey]; ok && len(pemStored) > 0 {
			if parsed, parseErr := ssh.ParsePrivateKey(pemStored); parseErr == nil {
				signer = parsed
				return nil
			}
		}
		if current.Data == nil {
			current.Data = map[string][]byte{}
		}
		current.Data[hostKeySecretKey] = pemBytes
		if _, updateErr := secrets.Update(ctx, current, metav1.UpdateOptions{}); updateErr != nil {
			return updateErr
		}
		signer, err = ssh.NewSignerFromKey(priv)
		return err
	})
	if err != nil {
		return ephemeralHostSigner(logger, fmt.Errorf("update secret %q: %w", hostKeySecretName, err))
	}
	return signer, nil
}

// ephemeralHostSigner is the degraded path: SSH stays available, but clients
// will see a changed host key after the next operator restart.
func ephemeralHostSigner(logger *slog.Logger, cause error) (ssh.Signer, error) {
	logger.Warn("falling back to ephemeral SSH host key (not persisted!)", "error", cause)
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral ssh host key: %w", err)
	}
	return ssh.NewSignerFromKey(priv)
}
