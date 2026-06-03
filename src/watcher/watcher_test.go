package watcher

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

// newRoutingTestWatcher builds a watcher with two distinct factories so we can
// assert that factoryForGVR routes Secrets to the dedicated (field-selector)
// factory and everything else to the shared one.
func newRoutingTestWatcher() (*watcher, dynamicinformer.DynamicSharedInformerFactory, dynamicinformer.DynamicSharedInformerFactory) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	shared := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, time.Minute, metav1.NamespaceAll, nil)
	secret := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, time.Minute, metav1.NamespaceAll, func(opts *metav1.ListOptions) {
		opts.FieldSelector = secretWatchFieldSelector
	})
	return &watcher{factory: shared, secretFactory: secret}, shared, secret
}

func TestFactoryForGVR_RoutesSecretsToSecretFactory(t *testing.T) {
	w, _, secret := newRoutingTestWatcher()

	secretsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	assert.Same(t, secret, w.factoryForGVR(secretsGVR),
		"core/v1 Secret must route to the dedicated secretFactory")
}

func TestFactoryForGVR_RoutesOtherResourcesToSharedFactory(t *testing.T) {
	w, shared, _ := newRoutingTestWatcher()

	cases := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		// A secrets resource in a non-core group must NOT be treated as the
		// core Secret kind - it would not expose the `type` field selector.
		{Group: "example.com", Version: "v1", Resource: "secrets"},
	}
	for _, gvr := range cases {
		assert.Same(t, shared, w.factoryForGVR(gvr),
			"non core/v1-Secret GVR %v must route to the shared factory", gvr)
	}
}

func TestSecretWatchFieldSelectorExcludesHelmReleases(t *testing.T) {
	assert.Equal(t, "type!=helm.sh/release.v1", secretWatchFieldSelector)
}
