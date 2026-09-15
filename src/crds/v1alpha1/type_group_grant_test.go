package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroupGrantSpec_ApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    GroupGrantSpec
		expected string
	}{
		{
			name:     "claim value without role defaults to viewer",
			input:    GroupGrantSpec{ClaimValue: "642e3fdd-407f-4ba5-bb70-4412dd39e4a8", TargetType: "workspace", TargetName: "test"},
			expected: GroupGrantDefaultRole,
		},
		{
			name:     "explicit role is kept",
			input:    GroupGrantSpec{ClaimValue: "team-a", TargetType: "workspace", TargetName: "test", Role: "admin"},
			expected: "admin",
		},
		{
			name:     "no claim value leaves the role alone",
			input:    GroupGrantSpec{TargetType: "workspace", TargetName: "test"},
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := tt.input
			spec.ApplyDefaults()
			assert.Equal(t, tt.expected, spec.Role)
		})
	}
}
