package core

import (
	"log/slog"
	"mogenius-operator/src/assert"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/helm"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"slices"
	"sort"
	"sync"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// api layer to be accessed through websockets, http and other exposed apis
//
//	[Layer 1: Exposed APIs]
//	+-----------------+     +----------------+
//	|  WebsocketAPI   |     |     HttpAPI     |
//	|-----------------|     |-----------------|
//	| - Parse Inputs  |     | - Parse Inputs  |
//	| - Serialize Data|     | - Serialize Data|
//	+-----------------+     +-----------------+
//	        |                     |
//	        \_____________________/
//	                  |
//	                  V
//	[Layer 2: API Interface]
//	+---------------------------------+
//	|         API Interface           |
//	|---------------------------------|
//	| - Unified High-Level API Calls  |
//	+---------------------------------+
//	                  |
//	                  V
//	[Layer 3: Services]
//	+--------------------+   +--------------------+   +--------------------+
//	|     Service 1      |   |     Service 2      |   |     Service N      |
//	|--------------------|   |--------------------|   |--------------------|
//	| - Manages Subsystem|   | - Manages Subsystem|   | - Manages Subsystem|
//	+--------------------+   +--------------------+   +--------------------+
//	                  |
//	                  V
//	[Layer 4: Packages/Modules]
//	+-------------------+   +-------------------+   +-------------------+
//	|   Package/Mod1    |   |   Package/Mod2    |   |   Package/ModN    |
//	|-------------------|   |-------------------|   |-------------------|
//	| - Low-Level Ops   |   | - Low-Level Ops   |   | - Low-Level Ops   |
//	+-------------------+   +-------------------+   +-------------------+
type Api interface {
	GetAllWorkspaces() ([]GetWorkspaceResult, error)
	GetWorkspace(name string) (*GetWorkspaceResult, error)
	CreateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error)
	UpdateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error)
	DeleteWorkspace(name string) (string, error)

	GetAllUsers(email *string) ([]v1alpha1.User, error)
	GetUser(name string) (*v1alpha1.User, error)
	CreateUser(name string, spec v1alpha1.UserSpec) (string, error)
	UpdateUser(name string, spec v1alpha1.UserSpec) (string, error)
	DeleteUser(name string) (string, error)

	GetAllGrants(targetType, targetName *string) ([]v1alpha1.Grant, error)
	GetGrant(name string) (*v1alpha1.Grant, error)
	CreateGrant(name string, spec v1alpha1.GrantSpec) (string, error)
	UpdateGrant(name string, spec v1alpha1.GrantSpec) (string, error)
	DeleteGrant(name string) (string, error)

	GetWorkspaceResources(workspaceName string, whitelist []*utils.ResourceDescriptor, blacklist []*utils.ResourceDescriptor, namespaceWhitelist []string) ([]unstructured.Unstructured, error)
	GetResourceListByWhitelistPaginated(req ResourcesPaginatedRequest) (ResourcesPaginatedResponse, error)
	GetWorkspaceResourcesPaginated(workspaceName string, req WorkspaceResourcesPaginatedRequest) (WorkspaceResourcesPaginatedResponse, error)
	GetWorkspaceControllers(workspaceName string) ([]unstructured.Unstructured, error)
	GetWorkspacePods(workspaceName string) ([]unstructured.Unstructured, error)
	GetWorkspacePodsNames(workspaceName string) ([]string, error)
	GetWorkspaceNamespaces(workspaceName string) ([]string, error)

	Link(workspaceManager WorkspaceManager)
}

type api struct {
	workspaceManager WorkspaceManager
	logger           *slog.Logger
	valkeyClient     valkeyclient.ValkeyClient
	config           cfg.ConfigModule
}

func NewApi(logger *slog.Logger, valkeyClient valkeyclient.ValkeyClient, config cfg.ConfigModule) Api {
	self := &api{}

	self.logger = logger
	self.valkeyClient = valkeyClient
	self.config = config

	return self
}

func (self *api) Link(workspaceManager WorkspaceManager) {
	assert.Assert(workspaceManager != nil)

	self.workspaceManager = workspaceManager
}

type GetWorkspaceResult struct {
	Name              string                                 `json:"name" validate:"required"`
	CreationTimestamp v1.Time                                `json:"creationTimestamp,omitempty"`
	Resources         []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
}

func NewGetWorkspaceResult(name string, creationTimestamp v1.Time, resources []v1alpha1.WorkspaceResourceIdentifier) GetWorkspaceResult {
	return GetWorkspaceResult{
		Name:              name,
		CreationTimestamp: creationTimestamp,
		Resources:         resources,
	}
}

func (self *api) GetAllWorkspaces() ([]GetWorkspaceResult, error) {
	namespace := self.config.Get("MO_OWN_NAMESPACE")
	resources, err := store.GetAllWorkspaces(namespace)
	if err != nil {
		return []GetWorkspaceResult{}, err
	}

	result := make([]GetWorkspaceResult, 0, len(resources))
	for _, resource := range resources {
		result = append(result, NewGetWorkspaceResult(
			resource.GetName(),
			resource.ObjectMeta.CreationTimestamp,
			resource.Spec.Resources,
		))
	}

	return result, nil
}

func (self *api) GetWorkspace(name string) (*GetWorkspaceResult, error) {
	namespace := self.config.Get("MO_OWN_NAMESPACE")
	resource, err := store.GetWorkspace(namespace, name)
	if err != nil {
		return nil, err
	}

	result := NewGetWorkspaceResult(
		resource.GetName(),
		resource.ObjectMeta.CreationTimestamp,
		resource.Spec.Resources,
	)

	return &result, nil
}

func (self *api) CreateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error) {
	_, err := self.workspaceManager.CreateWorkspace(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource created successfully", nil
}

func (self *api) UpdateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error) {
	_, err := self.workspaceManager.UpdateWorkspace(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource updated successfully", nil
}

func (self *api) DeleteWorkspace(name string) (string, error) {
	// Clean up NFS volumes in workspace namespaces before deleting the workspace CRD,
	// otherwise namespace deletion orphans cluster-scoped PVs and the platform retains stale entries.
	// Only clean up namespaces that are not shared with other workspaces.
	namespaces, err := self.GetWorkspaceNamespaces(name)
	if err != nil {
		self.logger.Warn("failed to get workspace namespaces for NFS cleanup, proceeding with deletion", "workspace", name, "error", err)
	} else if len(namespaces) > 0 {
		exclusiveNamespaces := self.findExclusiveNamespaces(name, namespaces)
		for _, ns := range exclusiveNamespaces {
			kubernetes.CleanupNfsVolumesInNamespace(ns)
		}
	}

	err = self.workspaceManager.DeleteWorkspace(name)
	if err != nil {
		return "", err
	}

	return "Resource deleted successfully", nil
}

// findExclusiveNamespaces returns only namespaces that are not referenced by any other workspace.
func (self *api) findExclusiveNamespaces(workspaceName string, namespaces []string) []string {
	allWorkspaces, err := self.GetAllWorkspaces()
	if err != nil {
		self.logger.Warn("failed to list workspaces for shared namespace check, skipping NFS cleanup", "error", err)
		return nil
	}

	// Collect all namespaces used by other workspaces
	sharedNamespaces := map[string]struct{}{}
	for _, ws := range allWorkspaces {
		if ws.Name == workspaceName {
			continue
		}
		for _, res := range ws.Resources {
			if res.Type == "namespace" {
				sharedNamespaces[res.Id] = struct{}{}
			}
		}
	}

	exclusive := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		if _, shared := sharedNamespaces[ns]; !shared {
			exclusive = append(exclusive, ns)
		}
	}
	return exclusive
}

func (self *api) GetAllUsers(email *string) ([]v1alpha1.User, error) {
	resources, err := self.workspaceManager.GetAllUsers(email)
	if err != nil {
		return []v1alpha1.User{}, err
	}

	return resources, nil
}

func (self *api) GetUser(name string) (*v1alpha1.User, error) {
	resource, err := self.workspaceManager.GetUser(name)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (self *api) CreateUser(name string, spec v1alpha1.UserSpec) (string, error) {
	_, err := self.workspaceManager.CreateUser(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource created successfully", nil
}

func (self *api) UpdateUser(name string, spec v1alpha1.UserSpec) (string, error) {
	_, err := self.workspaceManager.UpdateUser(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource updated successfully", nil
}

func (self *api) DeleteUser(name string) (string, error) {
	err := self.workspaceManager.DeleteUser(name)
	if err != nil {
		return "", err
	}

	return "Resource deleted successfully", nil
}

func (self *api) GetAllGrants(targetType, targetName *string) ([]v1alpha1.Grant, error) {
	resources, err := self.workspaceManager.GetAllGrants(targetType, targetName)
	if err != nil {
		return []v1alpha1.Grant{}, err
	}

	return resources, nil
}

func (self *api) GetGrant(name string) (*v1alpha1.Grant, error) {
	resource, err := self.workspaceManager.GetGrant(name)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (self *api) CreateGrant(name string, spec v1alpha1.GrantSpec) (string, error) {
	_, err := self.workspaceManager.CreateGrant(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource created successfully", nil
}

func (self *api) UpdateGrant(name string, spec v1alpha1.GrantSpec) (string, error) {
	_, err := self.workspaceManager.UpdateGrant(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource updated successfully", nil
}

func (self *api) DeleteGrant(name string) (string, error) {
	err := self.workspaceManager.DeleteGrant(name)
	if err != nil {
		return "", err
	}

	return "Resource deleted successfully", nil
}

type ResourcesPaginatedRequest struct {
	Whitelist          []*utils.ResourceDescriptor `json:"whitelist"`
	Blacklist          []*utils.ResourceDescriptor `json:"blacklist"`
	NamespaceWhitelist []string                    `json:"namespaceWhitelist"`
	Offset             int                         `json:"offset"`
	Limit              int                         `json:"limit"`
	SortBy             string                      `json:"sortBy"`
	SortOrder          string                      `json:"sortOrder"`
	WithData           *bool                       `json:"withData"`
}
type ResourcesPaginatedResponse struct {
	Items      []unstructured.Unstructured `json:"items"`
	TotalCount int                         `json:"totalCount"`
}

func (self *api) GetResourceListByWhitelistPaginated(req ResourcesPaginatedRequest) (ResourcesPaginatedResponse, error) {
	page, err := store.GetResourcesByWhitelistPaginated(self.valkeyClient, req.Whitelist, req.Blacklist, req.NamespaceWhitelist, req.Offset, req.Limit, req.SortBy, req.SortOrder, self.logger)
	if err != nil {
		return ResourcesPaginatedResponse{Items: []unstructured.Unstructured{}, TotalCount: page.TotalCount}, err
	}

	if req.WithData == nil || !*req.WithData {
		for i := range page.Items {
			delete(page.Items[i].Object, "data")
		}
	}

	return ResourcesPaginatedResponse{Items: page.Items, TotalCount: page.TotalCount}, nil
}

// WorkspaceResourcesPaginatedRequest expresses an offset/limit query on a
// workspace's resources. Compared to the non-paginated path, the operator
// itself performs dedup + sort + slice so the wire payload stays small
// regardless of how many resources exist in Valkey.
type WorkspaceResourcesPaginatedRequest struct {
	Whitelist          []*utils.ResourceDescriptor
	Blacklist          []*utils.ResourceDescriptor
	NamespaceWhitelist []string
	Offset             int
	// Limit of 0 means "no limit" - the full result is returned. Frontends
	// that paginate must always set Limit > 0; the open-ended form is kept
	// for symmetry with the non-paginated path.
	Limit int
	// SortBy may be "creationTimestamp" (default) or "name". Anything else
	// falls back to creationTimestamp.
	SortBy string
	// SortOrder may be "asc" or "desc". Default depends on SortBy:
	// creationTimestamp -> desc (newest first), name -> asc.
	SortOrder string
}

type WorkspaceResourcesPaginatedResponse struct {
	Items      []unstructured.Unstructured `json:"items"`
	TotalCount int                         `json:"totalCount"`
}

// GetWorkspaceResourcesPaginated returns a sorted, deduped, sliced view of a
// workspace's resources. The underlying fetch is identical to
// GetWorkspaceResources; the new work is purely in-memory post-processing.
// At 2000+ items per request this still keeps the wire payload to one page
// (e.g. 50 items) instead of the full set.
func (self *api) GetWorkspaceResourcesPaginated(workspaceName string, req WorkspaceResourcesPaginatedRequest) (WorkspaceResourcesPaginatedResponse, error) {
	// Fast path: when the selection maps cleanly onto the (kind, namespace)
	// pagination index, ZRANGE+MGET only the requested page instead of reading
	// every matching resource into memory and sorting it. Helm-scoped workspaces
	// and whitelist-less ("all kinds") queries fall through to the in-memory
	// path below - see indexableWorkspaceNamespaces for the exact conditions.
	if namespaces, ok := self.indexableWorkspaceNamespaces(workspaceName, req); ok {
		page, err := store.GetResourcesByWhitelistPaginated(
			self.valkeyClient, req.Whitelist, req.Blacklist, namespaces,
			req.Offset, req.Limit, req.SortBy, req.SortOrder, self.logger,
		)
		if err != nil {
			return WorkspaceResourcesPaginatedResponse{Items: []unstructured.Unstructured{}, TotalCount: page.TotalCount}, err
		}
		return WorkspaceResourcesPaginatedResponse{Items: page.Items, TotalCount: page.TotalCount}, nil
	}

	items, err := self.GetWorkspaceResources(workspaceName, req.Whitelist, req.Blacklist, req.NamespaceWhitelist)
	if err != nil {
		return WorkspaceResourcesPaginatedResponse{Items: []unstructured.Unstructured{}}, err
	}

	items = dedupeUnstructuredByUID(items)
	sortUnstructured(items, req.SortBy, req.SortOrder)
	total := len(items)

	if req.Limit > 0 {
		start := req.Offset
		if start < 0 {
			start = 0
		}
		if start > total {
			start = total
		}
		end := start + req.Limit
		if end > total {
			end = total
		}
		items = items[start:end]
	}

	if items == nil {
		items = []unstructured.Unstructured{}
	}
	return WorkspaceResourcesPaginatedResponse{Items: items, TotalCount: total}, nil
}

// indexableWorkspaceNamespaces decides whether a paginated workspace query can
// be served by the (kind, namespace) pagination index, and if so returns the
// (deduped) namespace list to scope it to. ok==false means "fall back to the
// in-memory GetWorkspaceResources path".
//
// The fast path requires:
//   - a non-empty whitelist: the index enumerates shards per kind, so it can't
//     answer the "all kinds" query an empty whitelist expresses.
//   - a namespace-only selection: a helm entry selects by release label (plus
//     Pod ownerRef traversal), which the index does not model. argocd entries
//     are ignored by GetWorkspaceResources too, so they don't affect the result.
//
// For the cluster-wide case (empty workspaceName) the namespace list is
// req.NamespaceWhitelist verbatim: empty there means "all namespaces", which the
// index resolves via its namespace registry. For a named workspace an empty
// namespace list means nothing indexable is selected, so we return (nil, false)
// and let the in-memory path produce the canonical empty result - passing an
// empty list to the index would wrongly be read as "all namespaces".
func (self *api) indexableWorkspaceNamespaces(workspaceName string, req WorkspaceResourcesPaginatedRequest) ([]string, bool) {
	if len(req.Whitelist) == 0 {
		return nil, false
	}
	// The (kind, namespace) pagination index only models namespaced resources.
	// Cluster-scoped kinds (e.g. Namespace) are stored without a namespace and
	// would never match a namespace-scoped shard, so fall back to the in-memory
	// path, which resolves them explicitly (see GetUnstructuredNamespaceResourceList).
	for _, w := range req.Whitelist {
		if w != nil && !w.Namespaced {
			return nil, false
		}
	}
	if workspaceName == "" {
		return req.NamespaceWhitelist, true
	}

	workspace, err := store.GetWorkspace(self.config.Get("MO_OWN_NAMESPACE"), workspaceName)
	if err != nil {
		return nil, false
	}

	seen := make(map[string]struct{}, len(workspace.Spec.Resources))
	namespaces := make([]string, 0, len(workspace.Spec.Resources))
	for _, v := range workspace.Spec.Resources {
		switch v.Type {
		case "helm":
			// Release-label selection is not representable in the index.
			return nil, false
		case "namespace":
			if len(req.NamespaceWhitelist) > 0 && !slices.Contains(req.NamespaceWhitelist, v.Id) {
				continue
			}
			if _, dup := seen[v.Id]; dup {
				continue
			}
			seen[v.Id] = struct{}{}
			namespaces = append(namespaces, v.Id)
		}
		// other types (e.g. "argocd") are ignored, matching GetWorkspaceResources.
	}
	if len(namespaces) == 0 {
		return nil, false
	}
	return namespaces, true
}

// dedupeUnstructuredByUID keeps the first occurrence of each metadata.uid.
// Used to be in the platform API; moved here so pagination ordering is
// deterministic at the slice boundary.
func dedupeUnstructuredByUID(items []unstructured.Unstructured) []unstructured.Unstructured {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]unstructured.Unstructured, 0, len(items))
	for _, it := range items {
		uid := string(it.GetUID())
		// Resources without a UID (shouldn't normally happen, but Helm
		// release templates and similar can be missing one) are kept as-is
		// since there's no key to dedupe on.
		if uid == "" {
			out = append(out, it)
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, it)
	}
	return out
}

// sortUnstructured sorts items in place. UID is the tiebreaker so the order
// is stable across requests when two items share a creationTimestamp.
func sortUnstructured(items []unstructured.Unstructured, sortBy, sortOrder string) {
	desc := sortOrder == "desc"
	switch sortBy {
	case "name":
		// Names default to ascending when no explicit order is given.
		if sortOrder == "" {
			desc = false
		}
		sort.SliceStable(items, func(i, j int) bool {
			ni, nj := items[i].GetName(), items[j].GetName()
			if ni == nj {
				return string(items[i].GetUID()) < string(items[j].GetUID())
			}
			if desc {
				return ni > nj
			}
			return ni < nj
		})
	default: // creationTimestamp (and any unknown value)
		// Timestamps default to descending so newest items appear first.
		if sortOrder == "" {
			desc = true
		}
		sort.SliceStable(items, func(i, j int) bool {
			ti := items[i].GetCreationTimestamp().Time
			tj := items[j].GetCreationTimestamp().Time
			if ti.Equal(tj) {
				return string(items[i].GetUID()) < string(items[j].GetUID())
			}
			if desc {
				return ti.After(tj)
			}
			return ti.Before(tj)
		})
	}
}

func (self *api) GetWorkspaceResources(workspaceName string, whitelist []*utils.ResourceDescriptor, blacklist []*utils.ResourceDescriptor, namespaceWhitelist []string) ([]unstructured.Unstructured, error) {
	// Empty workspaceName means "cluster-wide": the caller wants every
	// resource matching whitelist across the whole cluster, not scoped to a
	// single workspace's spec.Resources. The Studio cluster view sends an
	// empty workspace header for exactly this case; without the fallback
	// store.GetWorkspace below would return an error and the list would be
	// empty.
	if workspaceName == "" {
		if len(namespaceWhitelist) == 0 {
			return kubernetes.GetUnstructuredNamespaceResourceList("", whitelist, blacklist)
		}
		// Caller restricted to specific namespaces - fan out per namespace
		// so the namespaceWhitelist still applies to the cluster-wide view.
		results := make([]unstructured.Unstructured, 0)
		for _, ns := range namespaceWhitelist {
			nsResources, err := kubernetes.GetUnstructuredNamespaceResourceList(ns, whitelist, blacklist)
			if err != nil {
				return results, err
			}
			results = append(results, nsResources...)
		}
		return results, nil
	}

	// Get workspace
	namespace := self.config.Get("MO_OWN_NAMESPACE")
	workspace, err := store.GetWorkspace(namespace, workspaceName)
	if err != nil {
		return []unstructured.Unstructured{}, err
	}

	result := make([]unstructured.Unstructured, 0, len(workspace.Spec.Resources)*5)
	var resultMutex sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for _, v := range workspace.Spec.Resources {
		if v.Type == "namespace" {
			if len(namespaceWhitelist) > 0 {
				if !slices.Contains(namespaceWhitelist, v.Id) {
					continue
				}
			}
			wg.Go(func() {
				nsResources, err := kubernetes.GetUnstructuredNamespaceResourceList(v.Id, whitelist, blacklist)
				resultMutex.Lock()
				defer resultMutex.Unlock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					return
				}
				result = appendIfNotExists(result, nsResources...)
			})
		}
		if v.Type == "helm" {
			if len(namespaceWhitelist) > 0 {
				if !slices.Contains(namespaceWhitelist, v.Namespace) {
					continue
				}
			}
			helmReq := helm.HelmReleaseGetWorkloadsRequest{
				Namespace: v.Namespace,
				Release:   v.Id,
				Whitelist: whitelist,
			}
			wg.Go(func() {
				helmResources, err := helm.HelmReleaseGetWorkloads(self.valkeyClient, helmReq)
				resultMutex.Lock()
				defer resultMutex.Unlock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					return
				}
				result = appendIfNotExists(result, helmResources...)
			})
		}
	}
	wg.Wait()

	return result, firstErr
}

func (self *api) GetWorkspaceControllers(workspaceName string) ([]unstructured.Unstructured, error) {
	whiteList := []*utils.ResourceDescriptor{
		&utils.DaemonSetResource,
		&utils.StatefulSetResource,
		&utils.DeploymentResource,
	}
	return self.GetWorkspaceResources(workspaceName, whiteList, nil, nil)
}

func (self *api) GetWorkspacePods(workspaceName string) ([]unstructured.Unstructured, error) {
	whiteList := []*utils.ResourceDescriptor{
		&utils.PodResource,
	}
	return self.GetWorkspaceResources(workspaceName, whiteList, nil, nil)
}

func (self *api) GetWorkspacePodsNames(workspaceName string) ([]string, error) {
	whiteList := []*utils.ResourceDescriptor{
		&utils.PodResource,
	}
	pods, err := self.GetWorkspaceResources(workspaceName, whiteList, nil, nil)
	if err != nil {
		return nil, err
	}

	podNames := []string{}
	for _, pod := range pods {
		podNames = append(podNames, pod.GetName())
	}

	return podNames, nil
}

func (self *api) GetWorkspaceNamespaces(workspaceName string) ([]string, error) {
	namespace := self.config.Get("MO_OWN_NAMESPACE")
	workspace, err := store.GetWorkspace(namespace, workspaceName)
	if err != nil {
		return nil, err
	}

	namespaceNames := []string{}
	for _, v := range workspace.Spec.Resources {
		if v.Type == "namespace" {
			namespaceNames = append(namespaceNames, v.Id)
		}
	}

	return namespaceNames, nil
}

func appendIfNotExists(list []unstructured.Unstructured, item ...unstructured.Unstructured) []unstructured.Unstructured {
	for _, i := range item {
		if !slices.ContainsFunc(list, func(u unstructured.Unstructured) bool {
			return u.GetName() == i.GetName() && u.GetNamespace() == i.GetNamespace()
		}) {
			list = append(list, i)
		}
	}
	return list
}
