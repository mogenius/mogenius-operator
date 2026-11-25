package watcher

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mogenius-operator/src/utils"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

var (
	configMapsGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	configMapRes  = utils.ResourceDescriptor{Kind: "ConfigMap", Plural: "configmaps", ApiVersion: "v1", Namespaced: true}
)

func newFakeDynamicClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		configMapsGVR: "ConfigMapList",
	}, objects...)
}

func newTestWatcher(client *dynamicfake.FakeDynamicClient, limit int) *watcher {
	w := newWatcher(slog.New(slog.NewTextHandler(io.Discard, nil)), client, limit)
	w.overflowRecheckInterval = 50 * time.Millisecond
	return w
}

func newConfigMap(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": name, "namespace": "default"},
	}}
}

func configMaps(n int) []runtime.Object {
	objects := make([]runtime.Object, 0, n)
	for i := range n {
		objects = append(objects, newConfigMap(fmt.Sprintf("cm-%d", i)))
	}
	return objects
}

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func (self *watcher) runningInformer(gvr schema.GroupVersionResource) *informerEntry {
	self.handlerMapLock.RLock()
	defer self.handlerMapLock.RUnlock()
	return self.informers[gvr]
}

func stateOf(t *testing.T, w *watcher, resource utils.ResourceDescriptor) WatcherResourceState {
	t.Helper()
	state, err := w.State(resource)
	require.NoError(t, err)
	return state
}

// A kind with more objects than the limit gets no informer, is reported as
// Overflow and still settles OnSynced so the store can go ready.
func TestWatch_KindAboveObjectLimitAtStartOverflowsAndSettles(t *testing.T) {
	w := newTestWatcher(newFakeDynamicClient(configMaps(3)...), 2)
	w.overflowRecheckInterval = time.Hour // stay in overflow for this test

	synced := make(chan struct{})
	w.OnSynced(configMapRes, func() { close(synced) })
	_, err := w.Watch(configMapRes, WatchWeightStore, nil, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Unwatch(configMapRes) })

	waitClosed(t, synced, "overflow to settle OnSynced")
	assert.Equal(t, WatchingOverflow, stateOf(t, w, configMapRes))
	assert.Nil(t, w.runningInformer(configMapsGVR), "no informer may be started for an overflowing kind")

	// OnSynced registered after the fact fires right away
	late := make(chan struct{})
	w.OnSynced(configMapRes, func() { close(late) })
	waitClosed(t, late, "late OnSynced on overflowing kind")
}

// A kind that grows past the limit while watched has its informer stopped
// and released, and is watched again once it shrinks below the limit.
func TestWatch_KindGrowingPastLimitIsStoppedAndRecovers(t *testing.T) {
	client := newFakeDynamicClient(configMaps(2)...)
	w := newTestWatcher(client, 3)

	var mu sync.Mutex
	added := map[string]int{}
	onAdd := func(_ utils.ResourceDescriptor, obj *unstructured.Unstructured) {
		mu.Lock()
		added[obj.GetName()]++
		mu.Unlock()
	}

	synced := make(chan struct{})
	w.OnSynced(configMapRes, func() { close(synced) })
	_, err := w.Watch(configMapRes, WatchWeightStore, onAdd, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Unwatch(configMapRes) })

	waitClosed(t, synced, "initial sync")
	assert.Equal(t, Watching, stateOf(t, w, configMapRes))
	first := w.runningInformer(configMapsGVR)
	require.NotNil(t, first)

	// grow to 5 objects: the 4th crosses the limit of 3
	cms := client.Resource(configMapsGVR).Namespace("default")
	ctx := context.Background()
	for i := 2; i < 5; i++ {
		_, err := cms.Create(ctx, newConfigMap(fmt.Sprintf("cm-%d", i)), metav1.CreateOptions{})
		require.NoError(t, err)
	}

	assert.Eventually(t, func() bool { return stateOf(t, w, configMapRes) == WatchingOverflow },
		5*time.Second, 10*time.Millisecond, "kind must go to Overflow once it exceeds the limit")
	waitClosed(t, first.stopCh, "overflowing informer to be stopped")
	assert.NotSame(t, first, w.runningInformer(configMapsGVR), "the stopped informer must be forgotten")
	w.handlerMapLock.RLock()
	assert.Nil(t, w.activeHandlers[configMapRes].informer, "no reference to the stopped informer may remain")
	w.handlerMapLock.RUnlock()

	// shrink below the limit: the re-check starts a fresh informer
	require.NoError(t, cms.Delete(ctx, "cm-3", metav1.DeleteOptions{}))
	require.NoError(t, cms.Delete(ctx, "cm-4", metav1.DeleteOptions{}))

	assert.Eventually(t, func() bool { return stateOf(t, w, configMapRes) == Watching },
		5*time.Second, 10*time.Millisecond, "kind must be watched again once below the limit")
	second := w.runningInformer(configMapsGVR)
	require.NotNil(t, second)
	assert.NotSame(t, first, second)
	assert.Equal(t, int64(3), second.objects.Load())

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, added["cm-2"], 2, "the new informer replays its objects to the handlers")
}

func TestCheckObjectLimit(t *testing.T) {
	cases := []struct {
		name    string
		limit   int
		objects int
		wantErr bool
	}{
		{name: "disabled", limit: 0, objects: 3},
		{name: "at limit", limit: 3, objects: 3},
		{name: "above limit", limit: 2, objects: 3, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWatcher(newFakeDynamicClient(configMaps(tc.objects)...), tc.limit)
			err := w.checkObjectLimit(context.Background(), configMapsGVR)
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}
			var limitErr *objectLimitError
			require.ErrorAs(t, err, &limitErr)
			assert.Equal(t, tc.objects, limitErr.count)
			assert.Equal(t, tc.limit, limitErr.limit)
		})
	}
}

func TestListObjectCount(t *testing.T) {
	list := &unstructured.UnstructuredList{Items: make([]unstructured.Unstructured, 1)}
	count, known := listObjectCount(list)
	assert.Equal(t, 1, count)
	assert.True(t, known, "single page without continue token")

	remaining := int64(373876)
	list.SetRemainingItemCount(&remaining)
	list.SetContinue("token")
	count, known = listObjectCount(list)
	assert.Equal(t, 373877, count)
	assert.True(t, known)

	list.SetRemainingItemCount(nil)
	_, known = listObjectCount(list)
	assert.False(t, known, "more pages but no remainingItemCount: count unknown")
}
