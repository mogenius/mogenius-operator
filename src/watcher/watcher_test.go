package watcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestListOptionsTweak_FiltersHelmReleaseSecrets(t *testing.T) {
	secretsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	tweak := listOptionsTweak(secretsGVR)
	require.NotNil(t, tweak, "core/v1 Secrets must exclude Helm release secrets")

	opts := metav1.ListOptions{}
	tweak(&opts)
	assert.Equal(t, "type!=helm.sh/release.v1", opts.FieldSelector)
}

func TestListOptionsTweak_LeavesOtherResourcesUnfiltered(t *testing.T) {
	cases := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		// A secrets resource in a non-core group must NOT be treated as the
		// core Secret kind - it would not expose the `type` field selector.
		{Group: "example.com", Version: "v1", Resource: "secrets"},
	}
	for _, gvr := range cases {
		assert.Nil(t, listOptionsTweak(gvr), "%v must not get a field selector", gvr)
	}
}

func TestSecretWatchFieldSelectorExcludesHelmReleases(t *testing.T) {
	assert.Equal(t, "type!=helm.sh/release.v1", secretWatchFieldSelector)
}
