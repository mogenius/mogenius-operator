package tlsgateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPrefix = "/webhook/github"

// Shared with the platform: mo-platform-api-service PortForwardTunnelUtils spec ("path prefix (shared table with the
// operator)") carries the same cases. Both proxy paths must decide every case alike.
var pathPrefixCases = []struct {
	raw        string
	normalized string // "" when the path cannot be read unambiguously (400)
	matches    bool
}{
	{"/webhook/github", "/webhook/github", true},
	{"/webhook/github/", "/webhook/github/", true},
	{"/webhook/github/push", "/webhook/github/push", true},
	{"/webhook/github/./push", "/webhook/github/push", true},
	{"//webhook/github/push", "/webhook/github/push", true},
	{"/webhook//github/push", "/webhook/github/push", true},
	{"/webhook/github/a%20b", "/webhook/github/a b", true},
	{"/webhook/githubx", "/webhook/githubx", false},
	{"/webhook", "/webhook", false},
	{"/", "/", false},
	{"/WEBHOOK/github", "/WEBHOOK/github", false},
	{"/webhook/github/../admin", "/webhook/admin", false},
	{"/webhook/github/../../admin", "/admin", false},
	{"/../webhook/github", "", false},
	{"/webhook/github/%2e%2e/admin", "", false},
	{"/webhook/github/%2E%2E/admin", "", false},
	{"/webhook/github%2fpush", "", false},
	{"/webhook/github/%5c..", "", false},
	{`/webhook\github`, "", false},
	{"/webhook/github/%zz", "", false},
	{"/webhook/github/%252e%252e", "", false},
	{"/webhook/github/%00", "", false},
	{"/webhook/github/%ff", "", false},
	{"webhook/github", "", false},
}

func TestNormalizeRequestPathSharedTable(t *testing.T) {
	for _, c := range pathPrefixCases {
		t.Run(c.raw, func(t *testing.T) {
			path, ok := normalizeRequestPath(c.raw)
			assert.Equal(t, c.normalized != "", ok)
			assert.Equal(t, c.normalized, path)
			assert.Equal(t, c.matches, ok && pathMatchesPrefix(path, testPrefix))
		})
	}
}

func TestValidPathPrefix(t *testing.T) {
	assert.True(t, validPathPrefix("/webhook/github"))
	for _, invalid := range []string{"webhook", "/Webhook", "/web hook", "/a//b", "/a/../b", "/a/./b", "/a%20b", "/a/"} {
		assert.False(t, validPathPrefix(invalid), invalid)
	}
	_, err := NewAuthenticator(AuthConfig{Mode: AuthNone, PathPrefix: "/a/../b"})
	assert.Error(t, err, "a tunnel with a prefix it cannot enforce must not open")
}

// pathSeen records the path that reached the application.
func pathSeen(seen *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = r.URL.EscapedPath() + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte("app"))
	})
}

func TestPathPrefixHoldsForAnOpenTunnel(t *testing.T) {
	auth, err := NewAuthenticator(AuthConfig{Mode: AuthNone, PathPrefix: testPrefix})
	require.NoError(t, err)
	var seen string
	h := auth.Wrap(pathSeen(&seen))

	under := httptest.NewRecorder()
	h.ServeHTTP(under, request(http.MethodPost, "/webhook//github/a%20b?x=1", ""))
	assert.Equal(t, http.StatusOK, under.Code)
	assert.Equal(t, "/webhook/github/a%20b?x=1", seen, "the application gets the checked path")

	for _, target := range []string{"/", "/admin", "/webhook/githubx", "/webhook/github/../admin", "/__pf_login"} {
		seen = ""
		outside := httptest.NewRecorder()
		h.ServeHTTP(outside, request(http.MethodGet, target, ""))
		assert.Equal(t, http.StatusNotFound, outside.Code, target)
		assert.Empty(t, seen, target)
	}

	ambiguous := httptest.NewRecorder()
	h.ServeHTTP(ambiguous, request(http.MethodGet, "/webhook/github/%2e%2e/admin", ""))
	assert.Equal(t, http.StatusBadRequest, ambiguous.Code)
	assert.Empty(t, seen)
}

func TestPathPrefixKeepsTheSignInReachableAndComesFirst(t *testing.T) {
	config := passwordConfig(t)
	config.PathPrefix = testPrefix
	var cookies string
	h := guard(t, config, &cookies)

	// Outside the prefix: 404, not the password form.
	outside := httptest.NewRecorder()
	h.ServeHTTP(outside, request(http.MethodGet, "/admin", ""))
	assert.Equal(t, http.StatusNotFound, outside.Code)

	// The password form works although it is outside the prefix.
	login := httptest.NewRecorder()
	h.ServeHTTP(login, request(http.MethodPost, loginPath, "password=secret-password"))
	assert.NotEqual(t, http.StatusNotFound, login.Code)
	require.NotEmpty(t, login.Result().Cookies())

	// Under the prefix without a cookie: the challenge, not the application.
	locked := httptest.NewRecorder()
	h.ServeHTTP(locked, request(http.MethodGet, "/webhook/github/push", ""))
	assert.NotEqual(t, "app", locked.Body.String())

	// Under the prefix with the cookie: the application.
	open := httptest.NewRecorder()
	h.ServeHTTP(open, request(http.MethodGet, "/webhook/github/push", "", login.Result().Cookies()[0]))
	assert.Equal(t, "app", open.Body.String())
}
