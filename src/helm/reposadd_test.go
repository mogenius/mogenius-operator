package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeRepoURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "https://charts.example.com", "https://charts.example.com"},
		{"trailing slash", "https://charts.example.com/", "https://charts.example.com"},
		{"multiple trailing slashes", "https://charts.example.com///", "https://charts.example.com"},
		{"surrounding whitespace", "  https://charts.example.com/  ", "https://charts.example.com"},
		{"path preserved", "https://prometheus-community.github.io/helm-charts/", "https://prometheus-community.github.io/helm-charts"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeRepoURL(tc.in))
		})
	}
}

func TestNormalizeRepoURL_EqualityForDifferentTrailingSlash(t *testing.T) {
	// The same repo registered with and without a trailing slash must compare
	// equal so HelmRepoAdd detects the "same URL, different name" collision.
	a := normalizeRepoURL("https://prometheus-community.github.io/helm-charts")
	b := normalizeRepoURL("https://prometheus-community.github.io/helm-charts/")
	assert.Equal(t, a, b)
}
