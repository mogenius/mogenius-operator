package reconciler

import (
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// objectCache is a thread-safe in-memory store of unstructured objects, keyed
// by namespace/name (or just name for cluster-scoped resources).
type objectCache struct {
	mu      sync.RWMutex
	objects map[string]*unstructured.Unstructured
}

func newObjectCache() *objectCache {
	return &objectCache{objects: make(map[string]*unstructured.Unstructured)}
}

func (c *objectCache) key(obj *unstructured.Unstructured) string {
	if ns := obj.GetNamespace(); ns != "" {
		return ns + "/" + obj.GetName()
	}
	return obj.GetName()
}

func (c *objectCache) set(obj *unstructured.Unstructured) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.objects[c.key(obj)] = obj.DeepCopy()
}

func (c *objectCache) remove(obj *unstructured.Unstructured) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.objects, c.key(obj))
}

// snapshot returns deep copies of all cached objects. Safe to call concurrently.
func (c *objectCache) snapshot() []*unstructured.Unstructured {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*unstructured.Unstructured, 0, len(c.objects))
	for _, v := range c.objects {
		out = append(out, v.DeepCopy())
	}
	return out
}

func (c *objectCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.objects = make(map[string]*unstructured.Unstructured)
}
