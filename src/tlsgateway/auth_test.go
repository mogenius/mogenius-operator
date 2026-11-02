package tlsgateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const (
	testSessionID = "0b9414885f2339ba3de8"
	testKey       = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	testHandoff   = "https://0b9414885f2339ba3de8.tunnel.dev.mogenius.com/__mogenius/handoff"
)

// appSeen records what reached the application behind the guard.
func appSeen(t *testing.T, cookies *string) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*cookies = r.Header.Get("Cookie")
		_, _ = w.Write([]byte("app"))
	})
}

func guard(t *testing.T, config AuthConfig, cookies *string) http.Handler {
	t.Helper()
	// The limiter is shared by the whole operator; every test starts with no failures.
	passwordFailures = newLoginLimiter()
	auth, err := NewAuthenticator(config)
	require.NoError(t, err)
	return auth.Wrap(appSeen(t, cookies))
}

func passwordConfig(t *testing.T) AuthConfig {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("secret-password"), bcrypt.MinCost)
	require.NoError(t, err)
	return AuthConfig{Mode: AuthPassword, SessionID: testSessionID, PasswordHash: string(hash), Key: testKey}
}

// handoff signs a token as the platform does.
func handoff(t *testing.T, token handoffToken) string {
	t.Helper()
	raw, err := json.Marshal(token)
	require.NoError(t, err)
	payload := base64.RawURLEncoding.EncodeToString(raw)
	key, _ := hex.DecodeString(testKey)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func request(method, target string, body string, cookies ...*http.Cookie) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Host = "app.example.com"
	r.RemoteAddr = "203.0.113.7:0"
	if body != "" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	return r
}

func TestPasswordTunnelUnlocksWithTheRightPasswordOnly(t *testing.T) {
	var cookies string
	h := guard(t, passwordConfig(t), &cookies)

	locked := httptest.NewRecorder()
	h.ServeHTTP(locked, request(http.MethodGet, "/", ""))
	assert.Equal(t, http.StatusUnauthorized, locked.Code)
	assert.Contains(t, locked.Body.String(), "Tunnel locked")

	wrong := httptest.NewRecorder()
	h.ServeHTTP(wrong, request(http.MethodPost, loginPath, "password=nope"))
	assert.Equal(t, http.StatusUnauthorized, wrong.Code)
	assert.Contains(t, wrong.Body.String(), "Wrong password.")
	assert.Empty(t, wrong.Result().Cookies())

	right := httptest.NewRecorder()
	h.ServeHTTP(right, request(http.MethodPost, loginPath, "password=secret-password"))
	require.Equal(t, http.StatusFound, right.Code)
	unlock := right.Result().Cookies()[0]
	assert.Equal(t, authCookiePrefix+testSessionID, unlock.Name)
	assert.True(t, unlock.HttpOnly && unlock.Secure)

	open := httptest.NewRecorder()
	h.ServeHTTP(open, request(http.MethodGet, "/", "", unlock, &http.Cookie{Name: "app-session", Value: "x"}))
	assert.Equal(t, "app", open.Body.String())
	assert.Equal(t, "app-session=x", cookies, "the tunnel's own cookie never reaches the application")
}

func TestPasswordTunnelLocksOutAfterTooManyWrongPasswords(t *testing.T) {
	var cookies string
	h := guard(t, passwordConfig(t), &cookies)
	for range loginAttempts {
		h.ServeHTTP(httptest.NewRecorder(), request(http.MethodPost, loginPath, "password=nope"))
	}
	locked := httptest.NewRecorder()
	h.ServeHTTP(locked, request(http.MethodPost, loginPath, "password=secret-password"))
	assert.Equal(t, http.StatusTooManyRequests, locked.Code)
}

func TestPasswordLockoutSurvivesANewStreamOfTheSameTunnel(t *testing.T) {
	var cookies string
	h := guard(t, passwordConfig(t), &cookies)
	for range loginAttempts {
		h.ServeHTTP(httptest.NewRecorder(), request(http.MethodPost, loginPath, "password=nope"))
	}
	// The platform opens a new stream (another API pod, or after a quiet minute): a new authenticator.
	again, err := NewAuthenticator(passwordConfig(t))
	require.NoError(t, err)
	locked := httptest.NewRecorder()
	again.Wrap(appSeen(t, &cookies)).ServeHTTP(locked, request(http.MethodPost, loginPath, "password=secret-password"))
	assert.Equal(t, http.StatusTooManyRequests, locked.Code)

	// Another tunnel and another address are not affected.
	otherConfig := passwordConfig(t)
	otherConfig.SessionID = "1c0dbad0c0ffee0ddba1"
	other, err := NewAuthenticator(otherConfig)
	require.NoError(t, err)
	open := httptest.NewRecorder()
	other.Wrap(appSeen(t, &cookies)).ServeHTTP(open, request(http.MethodPost, loginPath, "password=secret-password"))
	assert.Equal(t, http.StatusFound, open.Code)

	fromElsewhere := request(http.MethodPost, loginPath, "password=secret-password")
	fromElsewhere.RemoteAddr = "198.51.100.9:0"
	elsewhere := httptest.NewRecorder()
	again.Wrap(appSeen(t, &cookies)).ServeHTTP(elsewhere, fromElsewhere)
	assert.Equal(t, http.StatusFound, elsewhere.Code)
}

func TestPasswordTunnelLocksAfterTooManyWrongPasswordsFromManyAddresses(t *testing.T) {
	var cookies string
	h := guard(t, passwordConfig(t), &cookies)
	for i := range loginTunnelAttempts {
		r := request(http.MethodPost, loginPath, "password=nope")
		r.RemoteAddr = net.JoinHostPort("2001:db8::"+strconv.FormatInt(int64(i), 16), "0")
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	locked := httptest.NewRecorder()
	fresh := request(http.MethodPost, loginPath, "password=secret-password")
	fresh.RemoteAddr = "192.0.2.1:0"
	h.ServeHTTP(locked, fresh)
	assert.Equal(t, http.StatusTooManyRequests, locked.Code)
}

func TestCookieOfAnotherTunnelOrForgedDoesNotUnlock(t *testing.T) {
	var cookies string
	h := guard(t, passwordConfig(t), &cookies)
	expiry := time.Now().Add(time.Hour).UnixMilli()
	for _, value := range []string{"garbage", "1.abc", strings.Repeat("0", 13) + ".deadbeef"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, request(http.MethodGet, "/", "", &http.Cookie{Name: authCookiePrefix + testSessionID, Value: value}))
		assert.Equal(t, http.StatusUnauthorized, rec.Code, value)
	}
	// A valid cookie of another tunnel, renamed to this one, still does not open it.
	other, err := NewAuthenticator(AuthConfig{Mode: AuthPassword, SessionID: "other", PasswordHash: "x", Key: testKey})
	require.NoError(t, err)
	expires := strconv.FormatInt(expiry, 10)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request(http.MethodGet, "/", "", &http.Cookie{Name: authCookiePrefix + testSessionID, Value: expires + "." + other.sign("other."+expires)}))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMogeniusTunnelSendsPageLoadsToThePlatformSignIn(t *testing.T) {
	var cookies string
	h := guard(t, AuthConfig{Mode: AuthMogenius, SessionID: testSessionID, Key: testKey, HandoffURL: testHandoff}, &cookies)

	page := httptest.NewRecorder()
	h.ServeHTTP(page, request(http.MethodGet, "/dashboard?tab=2", ""))
	require.Equal(t, http.StatusFound, page.Code)
	location, err := url.Parse(page.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "0b9414885f2339ba3de8.tunnel.dev.mogenius.com", location.Host)
	assert.Equal(t, "app.example.com", location.Query().Get("host"))
	assert.Equal(t, "/dashboard?tab=2", location.Query().Get("return"))

	script := httptest.NewRecorder()
	h.ServeHTTP(script, request(http.MethodPost, "/api/save", "a=b"))
	assert.Equal(t, http.StatusUnauthorized, script.Code)
}

func TestMogeniusCallbackTakesOnlyAFreshTokenForThisTunnelAndHost(t *testing.T) {
	var cookies string
	h := guard(t, AuthConfig{Mode: AuthMogenius, SessionID: testSessionID, Key: testKey, HandoffURL: testHandoff}, &cookies)
	valid := handoffToken{SessionID: testSessionID, Host: "app.example.com", UserID: "u1", Expires: time.Now().Add(time.Minute).UnixMilli()}

	for name, token := range map[string]string{
		"expired":      handoff(t, handoffToken{SessionID: testSessionID, Host: "app.example.com", Expires: time.Now().Add(-time.Second).UnixMilli()}),
		"other tunnel": handoff(t, handoffToken{SessionID: "other", Host: "app.example.com", Expires: valid.Expires}),
		"other host":   handoff(t, handoffToken{SessionID: testSessionID, Host: "evil.example.com", Expires: valid.Expires}),
		"forged":       strings.SplitN(handoff(t, valid), ".", 2)[0] + ".00",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, request(http.MethodGet, callbackPath+"?token="+url.QueryEscape(token), ""))
		assert.Equal(t, http.StatusForbidden, rec.Code, name)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request(http.MethodGet, callbackPath+"?token="+url.QueryEscape(handoff(t, valid))+"&return="+url.QueryEscape("/dashboard"), ""))
	require.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
	unlock := rec.Result().Cookies()[0]

	open := httptest.NewRecorder()
	h.ServeHTTP(open, request(http.MethodGet, "/dashboard", "", unlock))
	assert.Equal(t, "app", open.Body.String())
}

func TestReturnPathNeverLeavesTheHost(t *testing.T) {
	for raw, want := range map[string]string{
		"/a?b=c":               "/a?b=c",
		"https://evil.example": "/",
		"//evil.example":       "/",
		"/\\evil.example":      "/",
		"":                     "/",
	} {
		assert.Equal(t, want, safeReturnPath(raw), raw)
	}
}

func TestAuthenticatorRefusesWhatItCannotCheck(t *testing.T) {
	for name, config := range map[string]AuthConfig{
		"unknown mode":        {Mode: "magic"},
		"password, no hash":   {Mode: AuthPassword, SessionID: testSessionID, Key: testKey},
		"no key":              {Mode: AuthPassword, SessionID: testSessionID, PasswordHash: "x"},
		"handoff over http":   {Mode: AuthMogenius, SessionID: testSessionID, Key: testKey, HandoffURL: "http://x/"},
		"mogenius, no target": {Mode: AuthMogenius, SessionID: testSessionID, Key: testKey},
	} {
		_, err := NewAuthenticator(config)
		assert.Error(t, err, name)
	}
	public, err := NewAuthenticator(AuthConfig{})
	require.NoError(t, err)
	var cookies string
	rec := httptest.NewRecorder()
	public.Wrap(appSeen(t, &cookies)).ServeHTTP(rec, request(http.MethodGet, "/", ""))
	assert.Equal(t, "app", rec.Body.String())
}
