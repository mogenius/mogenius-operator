package openbaomanager

import (
	"testing"

	bao "github.com/openbao/openbao/api/v2"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOwnerReferences(t *testing.T) {
	t.Parallel()

	// No UID yet: no owner references (avoids an invalid dangling ref).
	assert.Nil(t, ownerReferences(metav1.OwnerReference{Kind: "PlatformConfig"}))

	owner := metav1.OwnerReference{
		APIVersion: "mogenius.com/v1alpha1",
		Kind:       "PlatformConfig",
		Name:       "platform",
		UID:        "abc-123",
	}
	refs := ownerReferences(owner)
	assert.Len(t, refs, 1)
	assert.Equal(t, owner, refs[0])
}

func TestTrimSlash(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "secret", trimSlash("secret/"))
	assert.Equal(t, "secret", trimSlash("secret"))
	assert.Equal(t, "", trimSlash(""))
	assert.Equal(t, "a/b", trimSlash("a/b/"))
}

func TestFilterKVMounts(t *testing.T) {
	t.Parallel()

	mounts := map[string]*bao.MountOutput{
		"secret/":    {Type: "kv", Options: map[string]string{"version": "2"}},
		"legacy/":    {Type: "kv", Options: map[string]string{"version": "1"}},
		"kv-noopt/":  {Type: "kv", Options: nil},
		"cubbyhole/": {Type: "cubbyhole"},
		"sys/":       {Type: "system"},
		"nil-mount/": nil,
	}

	got := filterKVMounts(mounts)

	// Only the three kv mounts survive.
	assert.Len(t, got, 3)

	byPath := map[string]KVMount{}
	for _, m := range got {
		byPath[m.Path] = m
	}
	assert.Equal(t, "2", byPath["secret"].Version)
	assert.Equal(t, "1", byPath["legacy"].Version)
	// Missing version defaults to "1".
	assert.Equal(t, "1", byPath["kv-noopt"].Version)

	_, hasCubby := byPath["cubbyhole"]
	assert.False(t, hasCubby)
}
