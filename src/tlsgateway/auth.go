package tlsgateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// The access modes of an end-to-end tunnel, as the platform names them.
const (
	AuthNone     = "none"
	AuthPassword = "password"
	AuthMogenius = "mogenius"
)

const (
	// authCookiePrefix starts the name of the cookie that unlocks a tunnel; the session ID follows. The platform
	// uses the same name on its own tunnel hosts.
	authCookiePrefix = "mo-pf-auth-"
	// loginPath takes the password form.
	loginPath = "/__pf_login"
	// callbackPath takes the platform's sign-in handoff.
	callbackPath = "/__mogenius/callback"
	// passwordCookieTTL is how long a correct password unlocks the tunnel.
	passwordCookieTTL = 24 * time.Hour
	// mogeniusCookieTTL is how long a mogenius sign-in unlocks the tunnel; a revoked role takes effect after it.
	mogeniusCookieTTL = 8 * time.Hour
	// loginAttempts wrong passwords from one address within loginWindow lock it out for the rest of the window;
	// loginTunnelAttempts from all addresses together lock the whole tunnel, which caps attempts spread over many
	// addresses. The same limits as the platform's own tunnel hosts.
	loginAttempts       = 10
	loginTunnelAttempts = 100
	loginWindow         = 5 * time.Minute
	// maxLoginBody bounds the password form's body.
	maxLoginBody = 8 << 10
)

// AuthConfig is how an end-to-end tunnel admits visitors. The platform sends it with the stream request over the
// operator's authenticated control connection.
type AuthConfig struct {
	// Mode is AuthNone, AuthPassword or AuthMogenius.
	Mode string `json:"mode"`
	// SessionID names the tunnel's cookie and is part of every signature.
	SessionID string `json:"sessionId"`
	// PasswordHash is the bcrypt hash of the tunnel's password (AuthPassword); the password itself never comes here.
	PasswordHash string `json:"passwordHash,omitempty"`
	// Key signs the cookies and verifies the platform's sign-in handoff tokens (hex).
	Key string `json:"key,omitempty"`
	// HandoffURL is where a visitor without a cookie signs in (AuthMogenius): the platform's own tunnel host, where
	// the visitor's platform session lives.
	HandoffURL string `json:"handoffUrl,omitempty"`
}

// handoffToken is what the platform signs after it checked a visitor's sign-in and permissions.
type handoffToken struct {
	SessionID string `json:"sid"`
	Host      string `json:"host"`
	UserID    string `json:"uid"`
	Expires   int64  `json:"exp"`
}

// Authenticator guards one tunnel: the password form, the platform's sign-in handoff and the cookie they set.
type Authenticator struct {
	config AuthConfig
	key    []byte
	limit  *loginLimiter
}

// NewAuthenticator checks that the config can work; a tunnel that cannot check its visitors must not open.
func NewAuthenticator(config AuthConfig) (*Authenticator, error) {
	switch config.Mode {
	case "", AuthNone:
		return &Authenticator{config: AuthConfig{Mode: AuthNone}}, nil
	case AuthPassword, AuthMogenius:
	default:
		return nil, fmt.Errorf("unknown access mode %q", config.Mode)
	}
	key, err := hex.DecodeString(config.Key)
	if err != nil || len(key) < 16 || config.SessionID == "" {
		return nil, fmt.Errorf("access mode %q needs a session and a key", config.Mode)
	}
	if config.Mode == AuthPassword && config.PasswordHash == "" {
		return nil, fmt.Errorf("access mode %q needs the password hash", config.Mode)
	}
	if config.Mode == AuthMogenius {
		parsed, err := url.Parse(config.HandoffURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, fmt.Errorf("access mode %q needs an https handoff URL", config.Mode)
		}
	}
	return &Authenticator{config: config, key: key, limit: passwordFailures}, nil
}

// Wrap guards next: a request passes with a valid cookie, which is removed before it reaches the application.
func (a *Authenticator) Wrap(next http.Handler) http.Handler {
	if a.config.Mode == AuthNone {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case a.config.Mode == AuthPassword && r.URL.Path == loginPath && r.Method == http.MethodPost:
			a.passwordLogin(w, r)
			return
		case a.config.Mode == AuthMogenius && r.URL.Path == callbackPath:
			a.callback(w, r)
			return
		}
		if !a.validCookie(r) {
			a.challenge(w, r)
			return
		}
		stripAuthCookies(r)
		next.ServeHTTP(w, r)
	})
}

// challenge sends a visitor without a cookie to the password form or to the platform's sign-in.
func (a *Authenticator) challenge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if a.config.Mode == AuthPassword {
		renderLoginForm(w, http.StatusUnauthorized, a.config.SessionID, "")
		return
	}
	// Only a page load can follow a redirect to another site; a script's request gets a plain answer.
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Sign in to open this tunnel.", http.StatusUnauthorized)
		return
	}
	target, _ := url.Parse(a.config.HandoffURL)
	query := target.Query()
	// With its port, so the way back also works on a non-standard port; the token binds the name only.
	query.Set("host", strings.ToLower(r.Host))
	query.Set("return", r.URL.RequestURI())
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (a *Authenticator) passwordLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	client := clientAddress(r)
	if !a.limit.allowed(a.config.SessionID, client) {
		renderLoginForm(w, http.StatusTooManyRequests, a.config.SessionID, "Too many attempts. Try again in a few minutes.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBody)
	if err := r.ParseForm(); err != nil {
		renderLoginForm(w, http.StatusBadRequest, a.config.SessionID, "Invalid request.")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(a.config.PasswordHash), []byte(r.PostForm.Get("password"))) != nil {
		a.limit.failed(a.config.SessionID, client)
		renderLoginForm(w, http.StatusUnauthorized, a.config.SessionID, "Wrong password.")
		return
	}
	a.setCookie(w, passwordCookieTTL)
	http.Redirect(w, r, "/", http.StatusFound)
}

// callback takes the platform's handoff: a token it signed for this tunnel, this host and a short while.
func (a *Authenticator) callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := a.verifyHandoff(r.URL.Query().Get("token"))
	if !ok || token.SessionID != a.config.SessionID || !strings.EqualFold(token.Host, hostOnly(r.Host)) {
		http.Error(w, "The sign-in link is invalid or expired. Open the address again.", http.StatusForbidden)
		return
	}
	a.setCookie(w, mogeniusCookieTTL)
	http.Redirect(w, r, safeReturnPath(r.URL.Query().Get("return")), http.StatusFound)
}

func (a *Authenticator) verifyHandoff(raw string) (handoffToken, bool) {
	payload, signature, ok := strings.Cut(raw, ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(a.sign(payload))) {
		return handoffToken{}, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return handoffToken{}, false
	}
	var token handoffToken
	if json.Unmarshal(decoded, &token) != nil || time.Now().UnixMilli() > token.Expires {
		return handoffToken{}, false
	}
	return token, true
}

// Cookie value: "<expiry ms>.<hex hmac(key, sessionId.expiry)>", the format the platform uses.
func (a *Authenticator) setCookie(w http.ResponseWriter, ttl time.Duration) {
	expires := time.Now().Add(ttl).UnixMilli()
	http.SetCookie(w, &http.Cookie{
		Name:     authCookiePrefix + a.config.SessionID,
		Value:    strconv.FormatInt(expires, 10) + "." + a.sign(a.config.SessionID+"."+strconv.FormatInt(expires, 10)),
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *Authenticator) validCookie(r *http.Request) bool {
	cookie, err := r.Cookie(authCookiePrefix + a.config.SessionID)
	if err != nil {
		return false
	}
	expiry, signature, ok := strings.Cut(cookie.Value, ".")
	if !ok {
		return false
	}
	expires, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || time.Now().UnixMilli() > expires {
		return false
	}
	return hmac.Equal([]byte(signature), []byte(a.sign(a.config.SessionID+"."+expiry)))
}

func (a *Authenticator) sign(message string) string {
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// stripAuthCookies keeps the tunnel's own cookies from the application.
func stripAuthCookies(r *http.Request) {
	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, cookie := range cookies {
		if !strings.HasPrefix(cookie.Name, authCookiePrefix) {
			r.AddCookie(cookie)
		}
	}
}

// safeReturnPath only follows a path on the same host: never another site.
func safeReturnPath(raw string) string {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") {
		return "/"
	}
	return raw
}

func hostOnly(hostport string) string {
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return strings.ToLower(host)
	}
	return strings.ToLower(hostport)
}

func clientAddress(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// passwordFailures is shared by every authenticator of this operator. A tunnel gets a new authenticator with each
// stream the platform opens (per API pod, again after a minute without connections), so a limiter of its own would
// forget the failures and let the same address try again.
var passwordFailures = newLoginLimiter()

// loginLimiter counts wrong passwords per tunnel and address, and per tunnel.
type loginLimiter struct {
	mu        sync.Mutex
	failures  map[string][]time.Time
	lastSweep time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{failures: map[string][]time.Time{}}
}

func (l *loginLimiter) allowed(sessionID, address string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(sessionID+"|"+address)) < loginAttempts && len(l.recent(sessionID)) < loginTunnelAttempts
}

func (l *loginLimiter) failed(sessionID, address string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for _, key := range []string{sessionID + "|" + address, sessionID} {
		l.failures[key] = append(l.recent(key), now)
	}
	// Entries are only trimmed when their key comes again; drop the stale ones now and then so addresses that
	// never return do not pile up.
	if now.Sub(l.lastSweep) > loginWindow {
		l.lastSweep = now
		for key := range l.failures {
			l.recent(key)
		}
	}
}

func (l *loginLimiter) recent(key string) []time.Time {
	kept := l.failures[key][:0]
	for _, at := range l.failures[key] {
		if time.Since(at) < loginWindow {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(l.failures, key)
		return nil
	}
	l.failures[key] = kept
	return kept
}

// loginForm looks like the platform's own password page.
var loginForm = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Tunnel locked</title>
  <style>
    :root { color-scheme: dark; }
    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif; background: #0e0f12; color: #e6e7eb; }
    .card { width: 100%; max-width: 360px; background: #16181d; border: 1px solid #25282f; border-radius: 12px;
      padding: 28px 24px; box-shadow: 0 12px 40px rgba(0, 0, 0, 0.4); }
    .lock { width: 44px; height: 44px; border-radius: 50%; background: rgba(0, 155, 197, 0.12); color: #009bc5;
      display: flex; align-items: center; justify-content: center; margin-bottom: 16px; font-size: 22px; }
    h1 { font-size: 18px; margin: 0 0 6px; font-weight: 600; }
    p.hint { font-size: 13px; color: #9aa0aa; margin: 0 0 20px; }
    label { display: block; font-size: 12px; color: #9aa0aa; margin-bottom: 6px; text-transform: uppercase; letter-spacing: 0.04em; }
    input[type="password"] { width: 100%; padding: 10px 12px; background: #0e0f12; border: 1px solid #2a2e36;
      border-radius: 6px; color: #fff; font-size: 14px; outline: none; }
    input[type="password"]:focus { border-color: #009bc5; }
    button { width: 100%; margin-top: 14px; padding: 10px 12px; background: #009bc5; color: #fff; border: none;
      border-radius: 6px; font-size: 14px; font-weight: 600; cursor: pointer; }
    button:hover { background: #00aedb; }
    .err { background: rgba(220, 53, 69, 0.12); color: #ff6b76; border: 1px solid rgba(220, 53, 69, 0.3);
      padding: 8px 10px; border-radius: 6px; font-size: 13px; margin-bottom: 14px; }
    .footer { font-size: 11px; color: #5d626c; margin-top: 18px; text-align: center; }
  </style>
</head>
<body>
  <form class="card" method="post" action="` + loginPath + `" autocomplete="off">
    <div class="lock">&#128274;</div>
    <h1>Tunnel locked</h1>
    <p class="hint">This port-forward tunnel is protected by a password.</p>
    {{if .Error}}<div class="err">{{.Error}}</div>{{end}}
    <label for="pf-password">Password</label>
    <input id="pf-password" type="password" name="password" autofocus required />
    <button type="submit">Unlock</button>
    <div class="footer">mogenius port-forward · {{.SessionID}}</div>
  </form>
</body>
</html>`))

func renderLoginForm(w http.ResponseWriter, status int, sessionID, errorMessage string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = loginForm.Execute(w, struct{ SessionID, Error string }{sessionID, errorMessage})
}
