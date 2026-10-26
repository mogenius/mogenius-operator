package tlsgateway

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v5/acme"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/go-acme/lego/v5/certificate"
	"github.com/go-acme/lego/v5/challenge/tlsalpn01"
	"github.com/go-acme/lego/v5/lego"
	"github.com/go-acme/lego/v5/providers/dns/cloudflare"
	"github.com/go-acme/lego/v5/registration"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

// The states of a domain's certificate, as the platform shows them.
const (
	CertificateIssuing = "issuing"
	CertificateReady   = "ready"
	CertificateError   = "error"
)

const (
	// acmeAccountSecret holds the ACME account key and registration of this cluster.
	acmeAccountSecret = "mogenius-tunnel-acme-account"
	// acmeDNSSecret, created by the cluster's owner, switches issuance to DNS-01; it never leaves the cluster.
	acmeDNSSecret = "mogenius-tunnel-dns"
	// acmeCertSecretPrefix starts the name of a domain's certificate Secret.
	acmeCertSecretPrefix = "mogenius-tunnel-tls-"
	// acmeHostnameAnnotation names the domain of a certificate Secret.
	acmeHostnameAnnotation = "mogenius.com/tunnel-hostname"
	// acmeManagedLabel marks the certificate Secrets this package owns.
	acmeManagedLabel = "mogenius.com/tunnel-certificate"
	// acmeRenewBefore is how long before its end a certificate is renewed.
	acmeRenewBefore = 30 * 24 * time.Hour
	// acmeRenewInterval is how often the certificates are checked for renewal.
	acmeRenewInterval = 12 * time.Hour
	// acmeRetryAfter is how long a failed issuance waits before the next status read retries it.
	acmeRetryAfter = 5 * time.Minute
	// acmeIssueTimeout bounds one issuance.
	acmeIssueTimeout = 5 * time.Minute
	// LetsEncryptProduction is the default ACME server.
	LetsEncryptProduction = "https://acme-v02.api.letsencrypt.org/directory"
)

// ACMEConfig is where and as whom certificates are requested.
type ACMEConfig struct {
	// Server is the ACME directory URL.
	Server string
	// Email is the contact of the ACME account; optional.
	Email string
	// CAFile is a PEM file the ACME server's TLS certificate is verified against, for test servers like Pebble.
	CAFile string
}

// CertificateStatus is what the platform learns about a domain's certificate: never the certificate or its key.
type CertificateStatus struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
	// CAARecord is the CAA record value that lets only this cluster's ACME account issue for the domain.
	CAARecord string `json:"caaRecord,omitempty"`
}

// ACME issues the certificates of end-to-end tunnel domains inside the cluster: by TLS-ALPN-01 through the
// platform's edge (the validation reaches the TLS gateway like any browser), or by DNS-01 when the cluster's owner
// created the acmeDNSSecret. Keys and certificates are Secrets in the operator's namespace and never leave it.
type ACME struct {
	logger    *slog.Logger
	clientset k8s.Interface
	namespace string
	config    ACMEConfig

	mu         sync.Mutex
	certs      map[string]*tls.Certificate
	states     map[string]acmeState
	challenges map[string]*tls.Certificate
	client     *lego.Client
	dnsKind    string
	accountURI string
	// issuances are the running issue() calls per domain. Forget cancels a
	// domain's issuance, and an issuance only keeps its certificate while it
	// is still the domain's current one: one finishing after its tunnel was
	// deleted used to recreate the Secret and the cached certificate.
	issuances   map[string]acmeIssuance
	issuanceSeq uint64
}

type acmeIssuance struct {
	id     uint64
	cancel context.CancelFunc
}

// acmeState is a domain without a usable certificate: being issued, or failed at a time.
type acmeState struct {
	state    string
	message  string
	failedAt time.Time
}

func NewACME(logger *slog.Logger, clientset k8s.Interface, namespace string, config ACMEConfig) *ACME {
	if config.Server == "" {
		config.Server = LetsEncryptProduction
	}
	return &ACME{
		logger:     logger,
		clientset:  clientset,
		namespace:  namespace,
		config:     config,
		certs:      map[string]*tls.Certificate{},
		states:     map[string]acmeState{},
		challenges: map[string]*tls.Certificate{},
		issuances:  map[string]acmeIssuance{},
	}
}

// Certificate serves a domain's certificate; without one it starts the issuance and the handshake fails meanwhile.
func (a *ACME) Certificate(hostname string) (*tls.Certificate, error) {
	if cert := a.cached(hostname); cert != nil {
		return cert, nil
	}
	if cert := a.loadSecret(context.Background(), hostname); cert != nil {
		return cert, nil
	}
	a.Status(hostname)
	return nil, fmt.Errorf("the certificate of %s is being issued", hostname)
}

// Challenge serves the TLS-ALPN-01 challenge certificate while a validation of hostname runs.
func (a *ACME) Challenge(hostname string) (*tls.Certificate, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	cert, ok := a.challenges[strings.ToLower(hostname)]
	return cert, ok
}

// Status reports a domain's certificate and starts its issuance when there is none yet, or retries a failed one
// after a while. The platform asks once the domain routes to this cluster.
func (a *ACME) Status(hostname string) CertificateStatus {
	hostname = strings.ToLower(hostname)
	if a.cached(hostname) != nil || a.loadSecret(context.Background(), hostname) != nil {
		return CertificateStatus{State: CertificateReady, CAARecord: a.caaRecord()}
	}
	a.mu.Lock()
	state, known := a.states[hostname]
	retry := known && state.state == CertificateError && time.Since(state.failedAt) > acmeRetryAfter
	_, issuing := a.issuances[hostname]
	if (!known || retry) && !issuing {
		a.states[hostname] = acmeState{state: CertificateIssuing}
		state = a.states[hostname]
		go a.issue(hostname)
	}
	a.mu.Unlock()
	return CertificateStatus{State: state.state, Message: state.message, CAARecord: a.caaRecord()}
}

// Forget drops a domain's certificate, in memory and as Secret, once its tunnel is gone.
func (a *ACME) Forget(ctx context.Context, hostname string) error {
	hostname = strings.ToLower(hostname)
	a.mu.Lock()
	delete(a.certs, hostname)
	delete(a.states, hostname)
	if issuance, ok := a.issuances[hostname]; ok {
		issuance.cancel()
		delete(a.issuances, hostname)
	}
	a.mu.Unlock()
	err := a.clientset.CoreV1().Secrets(a.namespace).Delete(ctx, certSecretName(hostname), metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// Run renews the certificates that end soon until ctx ends. A renewal needs the domain to route to this cluster,
// so a tunnel that is down over the date renews when it is up again (Status at its next use).
func (a *ACME) Run(ctx context.Context) {
	ticker := time.NewTicker(acmeRenewInterval)
	defer ticker.Stop()
	for {
		a.renewDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *ACME) renewDue(ctx context.Context) {
	secrets, err := a.clientset.CoreV1().Secrets(a.namespace).List(ctx, metav1.ListOptions{LabelSelector: acmeManagedLabel + "=true"})
	if err != nil {
		a.logger.Warn("list tunnel certificates", "error", err)
		return
	}
	for _, secret := range secrets.Items {
		hostname := secret.Annotations[acmeHostnameAnnotation]
		cert, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
		if hostname == "" || err != nil || cert.Leaf == nil {
			continue
		}
		if time.Until(cert.Leaf.NotAfter) > acmeRenewBefore {
			continue
		}
		a.mu.Lock()
		_, running := a.states[hostname]
		if !running {
			a.states[hostname] = acmeState{state: CertificateIssuing}
		}
		a.mu.Unlock()
		if !running {
			a.logger.Info("renewing tunnel certificate", "hostname", hostname, "notAfter", cert.Leaf.NotAfter)
			go a.issue(hostname)
		}
	}
}

// issue requests a certificate for hostname and keeps it; the outcome becomes the domain's state.
func (a *ACME) issue(hostname string) {
	ctx, cancel := context.WithTimeout(context.Background(), acmeIssueTimeout)
	defer cancel()
	a.mu.Lock()
	a.issuanceSeq++
	id := a.issuanceSeq
	a.issuances[hostname] = acmeIssuance{id: id, cancel: cancel}
	a.mu.Unlock()

	err := a.obtain(ctx, hostname, id)
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.isCurrentIssuanceLocked(hostname, id) {
		// forgotten meanwhile: leave no state behind
		return
	}
	delete(a.issuances, hostname)
	if err != nil {
		a.logger.Warn("tunnel certificate not issued", "hostname", hostname, "error", err)
		a.states[hostname] = acmeState{state: CertificateError, message: certificateErrorMessage(err), failedAt: time.Now()}
		return
	}
	delete(a.states, hostname)
	a.logger.Info("tunnel certificate issued", "hostname", hostname)
}

// isCurrentIssuanceLocked reports whether issuance id is still hostname's
// current one; a.mu must be held.
func (a *ACME) isCurrentIssuanceLocked(hostname string, id uint64) bool {
	issuance, ok := a.issuances[hostname]
	return ok && issuance.id == id
}

func (a *ACME) obtain(ctx context.Context, hostname string, id uint64) error {
	client, err := a.legoClient(ctx)
	if err != nil {
		return err
	}
	resource, err := client.Certificate.Obtain(ctx, certificate.ObtainRequest{Domains: []string{hostname}, KeyType: certcrypto.EC256, Bundle: true})
	if err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(resource.Certificate, resource.PrivateKey)
	if err != nil {
		return fmt.Errorf("the issued certificate does not parse: %w", err)
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        certSecretName(hostname),
			Namespace:   a.namespace,
			Labels:      map[string]string{acmeManagedLabel: "true"},
			Annotations: map[string]string{acmeHostnameAnnotation: hostname},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{corev1.TLSCertKey: resource.Certificate, corev1.TLSPrivateKeyKey: resource.PrivateKey},
	}
	if err := a.saveSecret(ctx, secret); err != nil {
		return fmt.Errorf("store the certificate: %w", err)
	}
	a.mu.Lock()
	current := a.isCurrentIssuanceLocked(hostname, id)
	if current {
		a.certs[hostname] = &cert
	}
	a.mu.Unlock()
	if !current {
		// Forget ran while this was issuing, possibly before the Secret was
		// written: remove it again instead of resurrecting the domain.
		err := a.clientset.CoreV1().Secrets(a.namespace).Delete(context.Background(), certSecretName(hostname), metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			a.logger.Warn("remove certificate of a forgotten tunnel domain", "hostname", hostname, "error", err)
		}
		return fmt.Errorf("the domain was removed while its certificate was issued")
	}
	return nil
}

// legoClient is the ACME client of this cluster's account, created and registered on first use. With DNS
// credentials in the cluster it validates by DNS-01, otherwise by TLS-ALPN-01 through the edge.
func (a *ACME) legoClient(ctx context.Context) (*lego.Client, error) {
	dnsToken, dnsKind := a.dnsCredentials(ctx)
	a.mu.Lock()
	if a.client != nil && a.dnsKind == dnsKind {
		client := a.client
		a.mu.Unlock()
		return client, nil
	}
	a.mu.Unlock()

	user, err := a.account(ctx)
	if err != nil {
		return nil, fmt.Errorf("the ACME account: %w", err)
	}
	config := lego.NewConfig(user)
	config.CADirURL = a.config.Server
	if a.config.CAFile != "" {
		httpClient, err := httpClientTrusting(a.config.CAFile)
		if err != nil {
			return nil, err
		}
		config.HTTPClient = httpClient
	}
	client, err := lego.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("the ACME server %s: %w", a.config.Server, err)
	}
	if dnsKind == "cloudflare" {
		cfConfig := cloudflare.NewDefaultConfig()
		cfConfig.AuthToken = dnsToken
		provider, err := cloudflare.NewDNSProviderConfig(cfConfig)
		if err != nil {
			return nil, fmt.Errorf("the DNS credentials in %s: %w", acmeDNSSecret, err)
		}
		if err := client.Challenge.SetDNS01Provider(provider); err != nil {
			return nil, err
		}
	} else if err := client.Challenge.SetTLSALPN01Provider(alpnProvider{a}); err != nil {
		return nil, err
	}

	if user.registration == nil {
		registered, err := client.Registration.Register(ctx, registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, fmt.Errorf("register the ACME account: %w", err)
		}
		user.registration = registered
		if err := a.saveAccount(ctx, user); err != nil {
			return nil, err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.client = client
	a.dnsKind = dnsKind
	a.accountURI = user.registration.Location
	return client, nil
}

// dnsCredentials reads the cluster owner's DNS token; empty kind means TLS-ALPN-01.
func (a *ACME) dnsCredentials(ctx context.Context) (token string, kind string) {
	secret, err := a.clientset.CoreV1().Secrets(a.namespace).Get(ctx, acmeDNSSecret, metav1.GetOptions{})
	if err != nil {
		return "", ""
	}
	if token := strings.TrimSpace(string(secret.Data["CLOUDFLARE_DNS_API_TOKEN"])); token != "" {
		return token, "cloudflare"
	}
	return "", ""
}

// caaRecord is the CAA value that limits issuance for a domain to this cluster's account, once it is known.
func (a *ACME) caaRecord() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.accountURI == "" || a.config.Server != LetsEncryptProduction {
		return ""
	}
	method := "tls-alpn-01"
	if a.dnsKind != "" {
		method = "dns-01"
	}
	return fmt.Sprintf(`0 issue "letsencrypt.org; accounturi=%s; validationmethods=%s"`, a.accountURI, method)
}

func (a *ACME) cached(hostname string) *tls.Certificate {
	hostname = strings.ToLower(hostname)
	a.mu.Lock()
	defer a.mu.Unlock()
	cert := a.certs[hostname]
	if cert == nil || cert.Leaf == nil || time.Now().After(cert.Leaf.NotAfter) {
		return nil
	}
	return cert
}

// loadSecret takes a domain's certificate from its Secret, e.g. after an operator restart.
func (a *ACME) loadSecret(ctx context.Context, hostname string) *tls.Certificate {
	hostname = strings.ToLower(hostname)
	secret, err := a.clientset.CoreV1().Secrets(a.namespace).Get(ctx, certSecretName(hostname), metav1.GetOptions{})
	if err != nil {
		return nil
	}
	cert, err := tls.X509KeyPair(secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey])
	if err != nil || cert.Leaf == nil || time.Now().After(cert.Leaf.NotAfter) {
		return nil
	}
	a.mu.Lock()
	a.certs[hostname] = &cert
	a.mu.Unlock()
	return &cert
}

func (a *ACME) saveSecret(ctx context.Context, secret *corev1.Secret) error {
	secrets := a.clientset.CoreV1().Secrets(a.namespace)
	_, err := secrets.Create(ctx, secret, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		_, err = secrets.Update(ctx, secret, metav1.UpdateOptions{})
	}
	return err
}

// account loads this cluster's ACME account, or makes a key for a new one.
func (a *ACME) account(ctx context.Context) (*acmeUser, error) {
	secret, err := a.clientset.CoreV1().Secrets(a.namespace).Get(ctx, acmeAccountSecret, metav1.GetOptions{})
	if err == nil && secret.Data["server"] != nil && string(secret.Data["server"]) == a.config.Server {
		block, _ := pem.Decode(secret.Data["key"])
		if block == nil {
			return nil, errors.New("the account key does not parse")
		}
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		user := &acmeUser{email: a.config.Email, key: key}
		if raw := secret.Data["registration"]; len(raw) > 0 {
			user.registration = decodeRegistration(raw)
		}
		return user, nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &acmeUser{email: a.config.Email, key: key}, nil
}

func (a *ACME) saveAccount(ctx context.Context, user *acmeUser) error {
	der, err := x509.MarshalECPrivateKey(user.key)
	if err != nil {
		return err
	}
	registered, err := json.Marshal(user.registration)
	if err != nil {
		return err
	}
	return a.saveSecret(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: acmeAccountSecret, Namespace: a.namespace},
		Data: map[string][]byte{
			"key":          pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}),
			"registration": registered,
			"server":       []byte(a.config.Server),
		},
	})
}

// acmeUser is the ACME account as lego wants it.
type acmeUser struct {
	email        string
	key          *ecdsa.PrivateKey
	registration *acme.ExtendedAccount
}

func (u *acmeUser) GetEmail() string                       { return u.email }
func (u *acmeUser) GetRegistration() *acme.ExtendedAccount { return u.registration }
func (u *acmeUser) GetPrivateKey() crypto.Signer           { return u.key }

// decodeRegistration reads the stored registration, also in the format of lego v4 ({"body": …, "uri": …}). Without
// an account URL it is nil, and the account registers again: the ACME server answers with the account of the key.
func decodeRegistration(raw []byte) *acme.ExtendedAccount {
	var account acme.ExtendedAccount
	if err := json.Unmarshal(raw, &account); err != nil {
		return nil
	}
	if account.Location == "" {
		var legacy struct {
			Body acme.Account `json:"body"`
			URI  string       `json:"uri"`
		}
		if err := json.Unmarshal(raw, &legacy); err != nil || legacy.URI == "" {
			return nil
		}
		account = acme.ExtendedAccount{Account: legacy.Body, Location: legacy.URI}
	}
	return &account
}

// alpnProvider answers TLS-ALPN-01 validations through the TLS gateway instead of a listener of its own.
type alpnProvider struct{ a *ACME }

func (p alpnProvider) Present(_ context.Context, domain, _, keyAuth string) error {
	cert, err := tlsalpn01.ChallengeCert(domain, keyAuth)
	if err != nil {
		return err
	}
	p.a.mu.Lock()
	defer p.a.mu.Unlock()
	p.a.challenges[strings.ToLower(domain)] = cert
	return nil
}

func (p alpnProvider) CleanUp(_ context.Context, domain, _, _ string) error {
	p.a.mu.Lock()
	defer p.a.mu.Unlock()
	delete(p.a.challenges, strings.ToLower(domain))
	return nil
}

// certSecretName is the Secret of a domain's certificate: a hash, since a domain may be longer than a name.
func certSecretName(hostname string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(hostname)))
	return acmeCertSecretPrefix + hex.EncodeToString(sum[:])[:16]
}

// certificateErrorMessage turns an ACME error into a line for the platform, without the account URL.
func certificateErrorMessage(err error) string {
	msg := err.Error()
	if len(msg) > 500 {
		msg = msg[:500] + "…"
	}
	return msg
}

// httpClientTrusting is an HTTP client that also trusts the CA in caFile.
func httpClientTrusting(caFile string) (*http.Client, error) {
	pemBytes, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read the ACME CA file: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("no certificate in %s", caFile)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	return &http.Client{Transport: transport, Timeout: 30 * time.Second}, nil
}
