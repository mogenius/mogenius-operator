package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceKeyMatches(t *testing.T) {
	allowed := map[string]struct{}{
		"v1:Pod":                            {},
		"apps/v1:Deployment":                {},
		"rbac.authorization.k8s.io/v1:Role": {},
	}

	tests := []struct {
		name      string
		key       string
		namespace string
		expected  bool
	}{
		{
			name:      "simple pod in namespace",
			key:       "resources:v1:Pod:myns:web-abc123",
			namespace: "myns",
			expected:  true,
		},
		{
			name:      "apiVersion with slash",
			key:       "resources:apps/v1:Deployment:myns:web",
			namespace: "myns",
			expected:  true,
		},
		{
			name: "RBAC name containing colons must not be dropped",
			key:  "resources:rbac.authorization.k8s.io/v1:Role:kube-system:system:controller:bootstrap-signer",

			namespace: "kube-system",
			expected:  true,
		},
		{
			name:      "wrong namespace",
			key:       "resources:v1:Pod:other:web-abc123",
			namespace: "myns",
			expected:  false,
		},
		{
			name:      "kind not in allowed set",
			key:       "resources:v1:Secret:myns:token",
			namespace: "myns",
			expected:  false,
		},
		{
			name:      "cluster-scoped resource (empty namespace segment)",
			key:       "resources:v1:Namespace::myns",
			namespace: "myns",
			expected:  false,
		},
		{
			name:      "glob crossing segments: name contains ':myns:' but namespace differs",
			key:       "resources:rbac.authorization.k8s.io/v1:Role:kube-system:system:myns:something",
			namespace: "myns",
			expected:  false,
		},
		{
			name:      "empty namespace matches any namespace",
			key:       "resources:v1:Pod:whatever:web",
			namespace: "",
			expected:  true,
		},
		{
			name:      "too few segments",
			key:       "resources:v1:Pod:myns",
			namespace: "myns",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, resourceKeyMatches(tt.key, tt.namespace, allowed))
		})
	}
}
