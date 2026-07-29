package kubernetes

import (
	"io"
	"log/slog"
	"testing"

	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/watcher"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

type fakeWatcherModule struct {
	watcher.WatcherModule
	watched   []utils.ResourceDescriptor
	unwatched []utils.ResourceDescriptor
}

func (f *fakeWatcherModule) ListWatchedResources() []utils.ResourceDescriptor {
	return f.watched
}

func (f *fakeWatcherModule) Unwatch(resource utils.ResourceDescriptor) error {
	f.unwatched = append(f.unwatched, resource)
	return nil
}

type fakeValkeyClient struct {
	valkeyclient.ValkeyClient
	deletedPatterns [][]string
}

func (f *fakeValkeyClient) DeleteMultiple(patterns ...string) error {
	f.deletedPatterns = append(f.deletedPatterns, patterns)
	return nil
}

func newCRDObject(group, plural, kind string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": plural + "." + group},
		"spec": map[string]any{
			"group": group,
			"scope": "Namespaced",
			"names": map[string]any{"plural": plural, "kind": kind},
		},
	}}
}

// Regression test: handleCRDDeletion used to build the descriptor to unwatch
// from the event's watcher (CustomResourceDefinition itself) instead of the
// deleted CRD's spec. The first CRD deletion therefore killed the CRD watcher
// and every subsequent CRD delete event was lost — deleted CRDs stayed in the
// paginated resource store until TTL expiry.
func TestHandleCRDDeletion(t *testing.T) {
	prevLogger, prevValkey := k8sLogger, valkeyClient
	defer func() { k8sLogger, valkeyClient = prevLogger, prevValkey }()
	k8sLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

	crdDescriptor := utils.ResourceDescriptor{
		Plural:     "customresourcedefinitions",
		ApiVersion: "apiextensions.k8s.io/v1",
		Kind:       "CustomResourceDefinition",
		Namespaced: false,
	}
	widgetDescriptor := utils.ResourceDescriptor{
		Plural:     "widgets",
		ApiVersion: "example.com/v1alpha1",
		Kind:       "Widget",
		Namespaced: true,
	}
	// same kind/plural in a different group must not be unwatched
	foreignWidgetDescriptor := utils.ResourceDescriptor{
		Plural:     "widgets",
		ApiVersion: "other.io/v1",
		Kind:       "Widget",
		Namespaced: true,
	}

	t.Run("unwatches the deleted CRD's kind, never the CRD watcher itself", func(t *testing.T) {
		wm := &fakeWatcherModule{watched: []utils.ResourceDescriptor{crdDescriptor, widgetDescriptor, foreignWidgetDescriptor}}
		valkey := &fakeValkeyClient{}
		valkeyClient = valkey

		handleCRDDeletion(wm, crdDescriptor, newCRDObject("example.com", "widgets", "Widget"))

		assert.Equal(t, []utils.ResourceDescriptor{widgetDescriptor}, wm.unwatched)
		if assert.Len(t, valkey.deletedPatterns, 1) {
			assert.Contains(t, valkey.deletedPatterns[0], "resources:example.com/v1alpha1:Widget:*")
			assert.Contains(t, valkey.deletedPatterns[0], "resources-idx:example.com/v1alpha1:Widget:*")
		}
	})

	t.Run("ignores delete events from non-CRD watchers", func(t *testing.T) {
		wm := &fakeWatcherModule{watched: []utils.ResourceDescriptor{crdDescriptor, widgetDescriptor}}
		valkey := &fakeValkeyClient{}
		valkeyClient = valkey

		handleCRDDeletion(wm, widgetDescriptor, newCRDObject("example.com", "widgets", "Widget"))

		assert.Empty(t, wm.unwatched)
		assert.Empty(t, valkey.deletedPatterns)
	})

	t.Run("does not unwatch anything for a malformed CRD object", func(t *testing.T) {
		wm := &fakeWatcherModule{watched: []utils.ResourceDescriptor{crdDescriptor, widgetDescriptor}}
		valkey := &fakeValkeyClient{}
		valkeyClient = valkey

		handleCRDDeletion(wm, crdDescriptor, &unstructured.Unstructured{Object: map[string]any{}})

		assert.Empty(t, wm.unwatched)
		assert.Empty(t, valkey.deletedPatterns)
	})
}
