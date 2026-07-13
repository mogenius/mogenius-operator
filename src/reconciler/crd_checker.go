package reconciler

import (
	"sync"
	"time"

	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/utils"
)

// crdChecker verifies whether a Kubernetes API resource (backed by a CRD) is
// registered in the cluster. A confirmed presence is cached forever — CRDs are
// almost never removed once installed. A confirmed absence is cached for
// absentTTL so that successive reconcile calls retry without hammering the
// discovery API.
type crdChecker struct {
	clientProvider k8sclient.K8sClientProvider

	mu        sync.RWMutex
	present   map[string]struct{}  // permanently confirmed present
	absentAt  map[string]time.Time // confirmed absent, with timestamp for TTL
	absentTTL time.Duration
}

func newCRDChecker(clientProvider k8sclient.K8sClientProvider) *crdChecker {
	return &crdChecker{
		clientProvider: clientProvider,
		present:        make(map[string]struct{}),
		absentAt:       make(map[string]time.Time),
		absentTTL:      time.Minute,
	}
}

// IsAvailable returns true if the given API resource is registered in the
// cluster. Presence is cached permanently; absence is cached for absentTTL.
func (c *crdChecker) IsAvailable(resource utils.ResourceDescriptor) bool {
	key := resource.ApiVersion + "/" + resource.Kind

	c.mu.RLock()
	if _, ok := c.present[key]; ok {
		c.mu.RUnlock()
		return true
	}
	if ts, ok := c.absentAt[key]; ok && time.Since(ts) < c.absentTTL {
		c.mu.RUnlock()
		return false
	}
	c.mu.RUnlock()

	available := c.checkAPI(resource)

	c.mu.Lock()
	defer c.mu.Unlock()
	if available {
		delete(c.absentAt, key)
		c.present[key] = struct{}{}
	} else {
		c.absentAt[key] = time.Now()
	}
	return available
}

// checkAPI calls the discovery API for the exact group/version and checks
// whether the Kind is listed. Any error is treated as absent.
func (c *crdChecker) checkAPI(resource utils.ResourceDescriptor) bool {
	list, err := c.clientProvider.K8sClientSet().Discovery().ServerResourcesForGroupVersion(resource.ApiVersion)
	if err != nil {
		return false
	}
	for _, r := range list.APIResources {
		if r.Kind == resource.Kind {
			return true
		}
	}
	return false
}
