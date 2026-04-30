package reconciler

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeObj(namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}

func TestObjectCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		objName   string
		wantKey   string
	}{
		{
			name:    "cluster-scoped resource uses name only",
			objName: "my-resource",
			wantKey: "my-resource",
		},
		{
			name:      "namespaced resource uses namespace/name",
			namespace: "default",
			objName:   "my-resource",
			wantKey:   "default/my-resource",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newObjectCache()
			obj := makeObj(tc.namespace, tc.objName)
			assert.Equal(t, tc.wantKey, c.key(obj))
		})
	}
}

func TestObjectCacheSetAndSnapshot(t *testing.T) {
	t.Parallel()
	c := newObjectCache()

	c.set(makeObj("ns", "a"))
	c.set(makeObj("ns", "b"))

	snap := c.snapshot()
	assert.Len(t, snap, 2)
}

func TestObjectCacheSetOverwritesExisting(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c := newObjectCache()

	first := makeObj("ns", "obj")
	first.SetLabels(map[string]string{"version": "1"})
	c.set(first)

	second := makeObj("ns", "obj")
	second.SetLabels(map[string]string{"version": "2"})
	c.set(second)

	snap := c.snapshot()
	assert.Len(snap, 1)
	assert.Equal("2", snap[0].GetLabels()["version"])
}

func TestObjectCacheSetStoresDeepCopy(t *testing.T) {
	t.Parallel()
	c := newObjectCache()

	obj := makeObj("ns", "obj")
	obj.SetLabels(map[string]string{"k": "original"})
	c.set(obj)

	// mutate original after set — cache must not reflect the change
	obj.SetLabels(map[string]string{"k": "mutated"})

	snap := c.snapshot()
	assert.Equal(t, "original", snap[0].GetLabels()["k"])
}

func TestObjectCacheSnapshotReturnsDeepCopies(t *testing.T) {
	t.Parallel()
	c := newObjectCache()

	obj := makeObj("ns", "obj")
	obj.SetLabels(map[string]string{"k": "original"})
	c.set(obj)

	snap := c.snapshot()
	// mutate the snapshot copy — cache must not reflect the change
	snap[0].SetLabels(map[string]string{"k": "mutated"})

	snap2 := c.snapshot()
	assert.Equal(t, "original", snap2[0].GetLabels()["k"])
}

func TestObjectCacheRemove(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	c := newObjectCache()

	c.set(makeObj("ns", "a"))
	c.set(makeObj("ns", "b"))
	c.remove(makeObj("ns", "a"))

	snap := c.snapshot()
	assert.Len(snap, 1)
	assert.Equal("b", snap[0].GetName())
}

func TestObjectCacheRemoveNonExistentIsNoop(t *testing.T) {
	t.Parallel()
	c := newObjectCache()

	c.set(makeObj("ns", "a"))
	c.remove(makeObj("ns", "ghost")) // does not exist

	assert.Len(t, c.snapshot(), 1)
}

func TestObjectCacheClear(t *testing.T) {
	t.Parallel()
	c := newObjectCache()

	c.set(makeObj("ns", "a"))
	c.set(makeObj("ns", "b"))
	c.clear()

	assert.Empty(t, c.snapshot())
}

func TestObjectCacheClearThenReuse(t *testing.T) {
	t.Parallel()
	c := newObjectCache()

	c.set(makeObj("ns", "a"))
	c.clear()
	c.set(makeObj("ns", "b"))

	snap := c.snapshot()
	assert.Len(t, snap, 1)
	assert.Equal(t, "b", snap[0].GetName())
}

func TestObjectCacheSnapshotEmptyCache(t *testing.T) {
	t.Parallel()
	c := newObjectCache()
	assert.Empty(t, c.snapshot())
}

func TestObjectCacheNamespaceIsolation(t *testing.T) {
	t.Parallel()
	c := newObjectCache()

	// same name, different namespaces — must be stored as separate entries
	c.set(makeObj("ns-a", "obj"))
	c.set(makeObj("ns-b", "obj"))

	assert.Len(t, c.snapshot(), 2)
}

func TestObjectCacheConcurrentSetAndSnapshot(t *testing.T) {
	t.Parallel()
	c := newObjectCache()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		i := i
		go func() {
			defer wg.Done()
			obj := makeObj("ns", fmt.Sprintf("obj-%d", i))
			obj.SetResourceVersion(fmt.Sprintf("%d", i))
			c.set(obj)
			c.snapshot()
		}()
	}
	wg.Wait()

	assert.Len(t, c.snapshot(), goroutines)
}

func TestObjectCacheConcurrentSetAndRemove(t *testing.T) {
	t.Parallel()
	c := newObjectCache()
	const n = 50

	// pre-populate
	for i := range n {
		c.set(makeObj("ns", fmt.Sprintf("obj-%d", i)))
	}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		i := i
		go func() {
			defer wg.Done()
			c.remove(makeObj("ns", fmt.Sprintf("obj-%d", i)))
		}()
	}
	wg.Wait()

	assert.Empty(t, c.snapshot())
}

func TestObjectCacheSetPreservesResourceVersion(t *testing.T) {
	t.Parallel()
	c := newObjectCache()

	obj := makeObj("", "cluster-obj")
	obj.SetResourceVersion("42")
	c.set(obj)

	snap := c.snapshot()
	assert.Equal(t, "42", snap[0].GetResourceVersion())
}

func TestObjectCacheSetPreservesOwnerReferences(t *testing.T) {
	t.Parallel()
	c := newObjectCache()

	obj := makeObj("ns", "owned")
	obj.SetOwnerReferences([]metav1.OwnerReference{
		{Name: "parent", UID: "uid-1"},
	})
	c.set(obj)

	snap := c.snapshot()
	assert.Len(t, snap[0].GetOwnerReferences(), 1)
	assert.Equal(t, "parent", snap[0].GetOwnerReferences()[0].Name)
}
