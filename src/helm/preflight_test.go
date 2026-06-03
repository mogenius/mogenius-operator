package helm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestIsCRDNotInstalledError(t *testing.T) {
	noMatch := &meta.NoKindMatchError{
		GroupKind:        schema.GroupKind{Group: "monitoring.coreos.com", Kind: "Prometheus"},
		SearchedVersions: []string{"v1"},
	}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"typed NoKindMatchError", noMatch, true},
		{"wrapped NoKindMatchError", fmt.Errorf("preflight: %w", noMatch), true},
		{"no matches for kind message", fmt.Errorf(`no matches for kind "PrometheusRule" in version "monitoring.coreos.com/v1"`), true},
		{"resource mapping not found message", fmt.Errorf("resource mapping not found for name: x"), true},
		{"ensure CRDs hint", fmt.Errorf("ensure CRDs are installed first"), true},
		{"unrelated error", fmt.Errorf("connection refused"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isCRDNotInstalledError(tc.err))
		})
	}
}

func TestPreflightResult_HasConflicts(t *testing.T) {
	cases := []struct {
		name   string
		result PreflightResult
		want   bool
	}{
		{
			name:   "empty result has no conflicts",
			result: PreflightResult{},
			want:   false,
		},
		{
			name: "adoptable only has no conflicts",
			result: PreflightResult{
				Adoptable: []AdoptionCandidate{{Kind: "ClusterRole", Name: "x"}},
			},
			want: false,
		},
		{
			name: "conflicts present reports true",
			result: PreflightResult{
				Conflicts: []OwnershipConflict{{Kind: "ClusterRole", Name: "x", OwnerRelease: "other"}},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.result.HasConflicts())
		})
	}
}

func TestPreflightResult_ConflictError_NilWhenNoConflicts(t *testing.T) {
	r := PreflightResult{
		Adoptable: []AdoptionCandidate{{Kind: "ClusterRole", Name: "x"}},
	}
	assert.NoError(t, r.ConflictError("rel", "ns"))
}

func TestPreflightResult_ConflictError_EnumeratesConflicts(t *testing.T) {
	r := PreflightResult{
		Conflicts: []OwnershipConflict{
			{
				Kind:         "ClusterRole",
				Namespace:    "",
				Name:         "kube-prometheus-stack-kube-state-metrics",
				OwnerRelease: "other-release",
				OwnerNS:      "other-ns",
			},
			{
				Kind:         "ClusterRoleBinding",
				Name:         "kube-prometheus-stack-operator",
				OwnerRelease: "other-release",
				OwnerNS:      "other-ns",
			},
		},
	}

	err := r.ConflictError("kube-prometheus-stack", "monitoring")

	assert.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "kube-prometheus-stack")
	assert.Contains(t, msg, "monitoring")
	assert.Contains(t, msg, "kube-prometheus-stack-kube-state-metrics")
	assert.Contains(t, msg, "kube-prometheus-stack-operator")
	assert.Contains(t, msg, "other-release")
	assert.Equal(t, 2, strings.Count(msg, "owned by release"))
}
