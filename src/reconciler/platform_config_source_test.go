package reconciler

import (
	"testing"

	"mogenius-operator/src/crds/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func TestDetectConfigSource(t *testing.T) {
	tests := []struct {
		name            string
		labels          map[string]string
		annotations     map[string]string
		engineNamespace string
		expected        v1alpha1.PlatformConfigSource
	}{
		{
			name:            "no markers is cluster owned",
			engineNamespace: "flux-system",
			expected:        v1alpha1.PlatformConfigSource{Source: configSourceCluster},
		},
		{
			// The trap this guards: a repository that is configured but not
			// syncing must stay editable, or the UI locks the user out of the
			// configuration they are trying to fix.
			name:            "mogenius labels alone are not evidence of a sync",
			labels:          map[string]string{"app.kubernetes.io/managed-by": "mogenius-operator"},
			engineNamespace: "flux-system",
			expected:        v1alpha1.PlatformConfigSource{Source: configSourceCluster},
		},
		{
			name: "flux kustomization labels",
			labels: map[string]string{
				fluxKustomizationNameLabel:      "platform",
				fluxKustomizationNamespaceLabel: "flux-system",
			},
			engineNamespace: "flux-system",
			expected:        v1alpha1.PlatformConfigSource{Source: configSourceGit, SyncedBy: "flux-system/platform"},
		},
		{
			name:            "flux without the namespace label falls back to the engine namespace",
			labels:          map[string]string{fluxKustomizationNameLabel: "platform"},
			engineNamespace: "flux-system",
			expected:        v1alpha1.PlatformConfigSource{Source: configSourceGit, SyncedBy: "flux-system/platform"},
		},
		{
			name:            "argo tracking id",
			annotations:     map[string]string{argoTrackingIDAnnotation: "platform:mogenius.com/PlatformConfig:/platform"},
			engineNamespace: "argocd",
			expected:        v1alpha1.PlatformConfigSource{Source: configSourceGit, SyncedBy: "argocd/platform"},
		},
		{
			name:            "argo instance label when tracking by label",
			labels:          map[string]string{argoInstanceLabel: "platform"},
			engineNamespace: "argocd",
			expected:        v1alpha1.PlatformConfigSource{Source: configSourceGit, SyncedBy: "argocd/platform"},
		},
		{
			name: "flux wins over an argo instance label",
			labels: map[string]string{
				fluxKustomizationNameLabel:      "platform",
				fluxKustomizationNamespaceLabel: "flux-system",
				argoInstanceLabel:               "something-else",
			},
			engineNamespace: "flux-system",
			expected:        v1alpha1.PlatformConfigSource{Source: configSourceGit, SyncedBy: "flux-system/platform"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			source := detectConfigSource(tc.labels, tc.annotations, tc.engineNamespace)

			assert.NotNil(t, source)
			assert.Equal(t, tc.expected, *source)
		})
	}
}

func TestArgoApplicationName(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		expected    string
	}{
		{
			name:        "full tracking id",
			annotations: map[string]string{argoTrackingIDAnnotation: "platform:apps/Deployment:mogenius/operator"},
			expected:    "platform",
		},
		{
			name:        "tracking id without separators is the name itself",
			annotations: map[string]string{argoTrackingIDAnnotation: "platform"},
			expected:    "platform",
		},
		{
			name:     "instance label",
			labels:   map[string]string{argoInstanceLabel: "platform"},
			expected: "platform",
		},
		{
			name:     "nothing",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, argoApplicationName(tc.labels, tc.annotations))
		})
	}
}

func TestConfigSourceEqual(t *testing.T) {
	source := &v1alpha1.PlatformConfigSource{Source: configSourceGit, Revision: "main@sha1:abc", SyncedBy: "flux-system/platform"}

	assert.True(t, configSourceEqual(nil, nil))
	assert.False(t, configSourceEqual(nil, source))
	assert.False(t, configSourceEqual(source, nil))
	assert.True(t, configSourceEqual(source, &v1alpha1.PlatformConfigSource{
		Source: configSourceGit, Revision: "main@sha1:abc", SyncedBy: "flux-system/platform",
	}))
	assert.False(t, configSourceEqual(source, &v1alpha1.PlatformConfigSource{
		Source: configSourceGit, Revision: "main@sha1:def", SyncedBy: "flux-system/platform",
	}))
	assert.False(t, configSourceEqual(source, &v1alpha1.PlatformConfigSource{Source: configSourceCluster}))
}
