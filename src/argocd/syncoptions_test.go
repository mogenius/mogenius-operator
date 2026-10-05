package argocd

import (
	"slices"
	"testing"
)

func TestWithServerSideApply(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"adds it", []string{"Prune=true"}, []string{"Prune=true", "ServerSideApply=true"}},
		{"overrides false", []string{"ServerSideApply=false", "CreateNamespace=true"}, []string{"CreateNamespace=true", "ServerSideApply=true"}},
		{"no duplicate", []string{"ServerSideApply=true"}, []string{"ServerSideApply=true"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withServerSideApply(tt.input); !slices.Equal(got, tt.expected) {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
		})
	}
}
