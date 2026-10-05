package reconciler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfigURL(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		version  string
		expected string
	}{
		{"empty version uses main branch", "", "", "https://raw.githubusercontent.com/mogenius/platform-defaults/refs/heads/main/argocd.yaml"},
		{"branch name uses heads", "", "develop", "https://raw.githubusercontent.com/mogenius/platform-defaults/refs/heads/develop/argocd.yaml"},
		{"release version uses tags", "", "v1.0.0", "https://raw.githubusercontent.com/mogenius/platform-defaults/refs/tags/v1.0.0/argocd.yaml"},
		{"release version without v uses tags", "", "1.2.3", "https://raw.githubusercontent.com/mogenius/platform-defaults/refs/tags/1.2.3/argocd.yaml"},
		{"pre-release version uses tags", "", "v1.0.0-rc.1", "https://raw.githubusercontent.com/mogenius/platform-defaults/refs/tags/v1.0.0-rc.1/argocd.yaml"},
		{"custom source is used as is", "https://example.com/defaults", "v1.0.0", "https://example.com/defaults/v1.0.0/argocd.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, defaultConfigURL(tt.source, tt.version, "argocd"))
		})
	}
}
