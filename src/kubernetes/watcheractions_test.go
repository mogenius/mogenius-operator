package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBackgroundDeleteOptions(t *testing.T) {
	opts := backgroundDeleteOptions()
	if assert.NotNil(t, opts.PropagationPolicy) {
		assert.Equal(t, metav1.DeletePropagationBackground, *opts.PropagationPolicy)
	}
}

func TestBlockingFinalizers(t *testing.T) {
	tests := []struct {
		name       string
		finalizers []string
		expected   []string
	}{
		{
			name:       "no finalizers",
			finalizers: []string{},
			expected:   []string{},
		},
		{
			name:       "transient orphan finalizer is ignored",
			finalizers: []string{metav1.FinalizerOrphanDependents},
			expected:   []string{},
		},
		{
			name:       "transient foregroundDeletion finalizer is ignored",
			finalizers: []string{metav1.FinalizerDeleteDependents},
			expected:   []string{},
		},
		{
			name:       "custom finalizer blocks",
			finalizers: []string{"kubernetes.io/pvc-protection"},
			expected:   []string{"kubernetes.io/pvc-protection"},
		},
		{
			name:       "mixed finalizers keep only blocking ones",
			finalizers: []string{metav1.FinalizerOrphanDependents, "example.com/my-finalizer", metav1.FinalizerDeleteDependents},
			expected:   []string{"example.com/my-finalizer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, BlockingFinalizers(tt.finalizers))
		})
	}
}
