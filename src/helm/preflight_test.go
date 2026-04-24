package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
