package core

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/crds/v1alpha1"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/watcher"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Reconciler interface {
	Link(leaderElector LeaderElector)
	Status() ReconcilerStatus
	Run()
	Start()
	Stop()
}

type reconciler struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider
	leaderElector  LeaderElector
	active         atomic.Bool
	watcher        watcher.WatcherModule
	status         ReconcilerStatus
	statusLock     sync.RWMutex

	// crd resources
	crdResources   []watcher.WatcherResourceIdentifier
	workspaces     []v1alpha1.Workspace
	workspacesLock sync.RWMutex
	grants         []v1alpha1.Grant
	grantsLock     sync.RWMutex
	users          []v1alpha1.User
	usersLock      sync.RWMutex

	// managed resources
	managedResources        []watcher.WatcherResourceIdentifier
	namespaces              []v1.Namespace
	namespacesLock          sync.RWMutex
	clusterRoles            []rbacv1.ClusterRole
	clusterRolesLock        sync.RWMutex
	clusterRoleBindings     []rbacv1.ClusterRoleBinding
	clusterRoleBindingsLock sync.RWMutex
	roleBindings            []rbacv1.RoleBinding
	roleBindingsLock        sync.RWMutex
}

func NewReconciler(
	logger *slog.Logger,
	config config.ConfigModule,
	clientProvider k8sclient.K8sClientProvider,
) Reconciler {
	self := &reconciler{}
	self.logger = logger
	self.config = config
	self.clientProvider = clientProvider
	self.active = atomic.Bool{}
	self.watcher = watcher.NewWatcher(self.logger.With("scope", "watcher"), self.clientProvider)
	self.status = NewReconcilerStatus()
	self.statusLock = sync.RWMutex{}

	self.crdResources = []watcher.WatcherResourceIdentifier{
		{Plural: "workspaces", Kind: "Workspace", ApiVersion: "mogenius.com/v1alpha1", Namespaced: false},
		{Plural: "users", Kind: "User", ApiVersion: "mogenius.com/v1alpha1", Namespaced: false},
		{Plural: "grants", Kind: "Grant", ApiVersion: "mogenius.com/v1alpha1", Namespaced: false},
	}
	self.workspaces = []v1alpha1.Workspace{}
	self.workspacesLock = sync.RWMutex{}
	self.grants = []v1alpha1.Grant{}
	self.grantsLock = sync.RWMutex{}
	self.users = []v1alpha1.User{}
	self.usersLock = sync.RWMutex{}

	self.managedResources = []watcher.WatcherResourceIdentifier{
		{Plural: "namespaces", Kind: "Namespace", ApiVersion: "v1", Namespaced: false},
		{Plural: "clusterroles", Kind: "ClusterRole", ApiVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
		{Plural: "clusterrolebindings", Kind: "ClusterRoleBinding", ApiVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
		{Plural: "rolebindings", Kind: "RoleBinding", ApiVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
	}
	self.namespaces = []v1.Namespace{}
	self.namespacesLock = sync.RWMutex{}
	self.clusterRoles = []rbacv1.ClusterRole{}
	self.clusterRolesLock = sync.RWMutex{}
	self.clusterRoleBindings = []rbacv1.ClusterRoleBinding{}
	self.clusterRoleBindingsLock = sync.RWMutex{}
	self.roleBindings = []rbacv1.RoleBinding{}
	self.roleBindingsLock = sync.RWMutex{}

	self.assertAllMogeniusCrdsAreWatched()

	return self
}

// between the embedded CRDs and self.watchResources
func (self *reconciler) assertAllMogeniusCrdsAreWatched() {
	kinds := []string{}
	for _, crd := range crds.GetCRDs() {
		var data struct {
			Spec struct {
				Names struct {
					Kind string `json:"kind"`
				} `json:"names"`
			} `json:"spec"`
		}
		err := yaml.Unmarshal([]byte(crd.Content), &data)
		assert.Assert(err == nil, err)
		kind := data.Spec.Names.Kind
		assert.Assert(kind != "", kind)
		kinds = append(kinds, kind)
	}

	for _, kind := range kinds {
		found := false
		for _, res := range self.crdResources {
			if res.Kind == kind {
				found = true
				break
			}
		}
		assert.Assert(found, "self.watchResources should contain every mogenius CRD kind but could not find this one", kind)
	}
}

func (self *reconciler) Link(leaderElector LeaderElector) {
	self.leaderElector = leaderElector
}

func (self *reconciler) Run() {
	assert.Assert(self.leaderElector != nil)

	self.leaderElector.OnLeading(self.Start)
	self.leaderElector.OnLeadingEnded(self.Stop)

	go func() {
		updateTicker := time.NewTicker(15 * time.Second)
		defer updateTicker.Stop()

		for range updateTicker.C {
			if !self.active.Load() {
				continue
			}
			self.reconcile()
		}
	}()
}

func (self *reconciler) Start() {
	self.clearCaches()
	self.enableWatcher()
	self.active.Store(true)
	self.statusLock.Lock()
	self.status.IsActive = true
	self.statusLock.Unlock()
}

func (self *reconciler) Stop() {
	self.active.Store(false)
	self.disableWatcher()
	self.clearCaches()
	self.statusLock.Lock()
	self.status.IsActive = false
	self.statusLock.Unlock()
}

type ReconcilerStatus struct {
	IsActive                       bool                      `json:"is_active"`
	LastUpdate                     *time.Time                `json:"last_update,omitempty"`
	ResourceWarnings               []ReconcilerResourceError `json:"resource_warnings"`
	ResourceErrors                 []ReconcilerResourceError `json:"resource_errors"`
	ManagedRoleBindingCount        int                       `json:"managed_role_binding_count"`
	ManagedClusterRoleBindingCount int                       `json:"managed_cluster_role_binding_count"`
	MogeniusUsersCount             int                       `json:"mogenius_users_count"`
	MogeniusWorkspacesCount        int                       `json:"mogenius_workspaces_count"`
	MogeniusGrantsCount            int                       `json:"mogenius_grants_count"`
}

func NewReconcilerStatus() ReconcilerStatus {
	status := ReconcilerStatus{}
	status.ResourceWarnings = []ReconcilerResourceError{}
	status.ResourceErrors = []ReconcilerResourceError{}
	return status
}

func (self *ReconcilerStatus) Clone() ReconcilerStatus {
	other := ReconcilerStatus{
		self.IsActive,
		self.LastUpdate,
		make([]ReconcilerResourceError, 0, len(self.ResourceWarnings)),
		make([]ReconcilerResourceError, 0, len(self.ResourceErrors)),
		self.ManagedRoleBindingCount,
		self.ManagedClusterRoleBindingCount,
		self.MogeniusUsersCount,
		self.MogeniusWorkspacesCount,
		self.MogeniusGrantsCount,
	}
	for _, r := range self.ResourceWarnings {
		other.ResourceWarnings = append(other.ResourceWarnings, r.Clone())
	}
	for _, r := range self.ResourceErrors {
		other.ResourceErrors = append(other.ResourceErrors, r.Clone())
	}
	return other
}

type ReconcilerResourceError struct {
	ResourceGroup     string
	ResourceVersion   string
	ResourceKind      string
	ResourceName      string
	ResourceNamespace string
	Error             string
}

func (self *ReconcilerResourceError) Clone() ReconcilerResourceError {
	return ReconcilerResourceError{
		self.ResourceGroup,
		self.ResourceVersion,
		self.ResourceKind,
		self.ResourceName,
		self.ResourceNamespace,
		self.Error,
	}
}

func (self *reconciler) Status() ReconcilerStatus {
	self.statusLock.RLock()
	defer self.statusLock.RUnlock()
	return self.status.Clone()
}

func (self *reconciler) updateStatus(
	namespaces []v1.Namespace,
	workspaces []v1alpha1.Workspace,
	users []v1alpha1.User,
	clusterRoles []rbacv1.ClusterRole,
	grants []v1alpha1.Grant,
	roleBindings []rbacv1.RoleBinding,
	clusterRoleBindings []rbacv1.ClusterRoleBinding,
) {
	self.statusLock.RLock()
	status := self.status.Clone()
	self.statusLock.RUnlock()
	status.LastUpdate = utils.Pointer(time.Now())
	status.MogeniusUsersCount = len(users)
	status.MogeniusWorkspacesCount = len(workspaces)
	status.MogeniusGrantsCount = len(grants)

	for _, user := range users {
		if user.Spec.Email == "" {
			resourceErr := ReconcilerResourceError{}
			resourceErr.ResourceGroup = user.GetObjectKind().GroupVersionKind().Group
			resourceErr.ResourceVersion = user.GetObjectKind().GroupVersionKind().Version
			resourceErr.ResourceKind = user.GetObjectKind().GroupVersionKind().Kind
			resourceErr.ResourceName = user.GetName()
			resourceErr.ResourceNamespace = user.GetNamespace()
			resourceErr.Error = "User email is not set - mogenius does not have an identifier for this user"
			status.ResourceErrors = append(status.ResourceErrors, resourceErr)
		}
		if user.Spec.Subject == nil {
			resourceErr := ReconcilerResourceError{}
			resourceErr.ResourceGroup = user.GetObjectKind().GroupVersionKind().Group
			resourceErr.ResourceVersion = user.GetObjectKind().GroupVersionKind().Version
			resourceErr.ResourceKind = user.GetObjectKind().GroupVersionKind().Kind
			resourceErr.ResourceName = user.GetName()
			resourceErr.ResourceNamespace = user.GetNamespace()
			resourceErr.Error = "User subject is not set - no permissions in the cluster can be granted"
			status.ResourceWarnings = append(status.ResourceErrors, resourceErr)
		}
	}

	for _, workspace := range workspaces {
		for _, resource := range workspace.Spec.Resources {
			switch resource.Type {
			case "namespace":
				namespace := resource.Id
				if namespace == "" {
					resourceErr := ReconcilerResourceError{}
					resourceErr.ResourceGroup = workspace.GetObjectKind().GroupVersionKind().Group
					resourceErr.ResourceVersion = workspace.GetObjectKind().GroupVersionKind().Version
					resourceErr.ResourceKind = workspace.GetObjectKind().GroupVersionKind().Kind
					resourceErr.ResourceName = workspace.GetName()
					resourceErr.ResourceNamespace = workspace.GetNamespace()
					resourceErr.Error = "Workspace contains a resource of type 'namespace' which does not specifiy the namespace name in resource.Id"
					status.ResourceErrors = append(status.ResourceErrors, resourceErr)
				}
				_, err := self.findNamespaceByName(namespaces, namespace)
				if err != nil {
					resourceErr := ReconcilerResourceError{}
					resourceErr.ResourceGroup = workspace.GetObjectKind().GroupVersionKind().Group
					resourceErr.ResourceVersion = workspace.GetObjectKind().GroupVersionKind().Version
					resourceErr.ResourceKind = workspace.GetObjectKind().GroupVersionKind().Kind
					resourceErr.ResourceName = workspace.GetName()
					resourceErr.ResourceNamespace = workspace.GetNamespace()
					resourceErr.Error = fmt.Sprintf("Workspace contains a resource of type 'namespace' pointing to a namespace which does not exist: %#v", namespace)
					status.ResourceErrors = append(status.ResourceErrors, resourceErr)
				}
			case "helm":
				namespace := resource.Namespace
				if namespace == "" {
					resourceErr := ReconcilerResourceError{}
					resourceErr.ResourceGroup = workspace.GetObjectKind().GroupVersionKind().Group
					resourceErr.ResourceVersion = workspace.GetObjectKind().GroupVersionKind().Version
					resourceErr.ResourceKind = workspace.GetObjectKind().GroupVersionKind().Kind
					resourceErr.ResourceName = workspace.GetName()
					resourceErr.ResourceNamespace = workspace.GetNamespace()
					resourceErr.Error = "Workspace contains a resource of type 'helm' which does not specifiy the namespace name in resource.Namespace"
					status.ResourceErrors = append(status.ResourceErrors, resourceErr)
				}
				_, err := self.findNamespaceByName(namespaces, namespace)
				if err != nil {
					resourceErr := ReconcilerResourceError{}
					resourceErr.ResourceGroup = workspace.GetObjectKind().GroupVersionKind().Group
					resourceErr.ResourceVersion = workspace.GetObjectKind().GroupVersionKind().Version
					resourceErr.ResourceKind = workspace.GetObjectKind().GroupVersionKind().Kind
					resourceErr.ResourceName = workspace.GetName()
					resourceErr.ResourceNamespace = workspace.GetNamespace()
					resourceErr.Error = fmt.Sprintf("Workspace contains a resource of type 'helm' pointing to a namespace which does not exist: %#v", namespace)
					status.ResourceErrors = append(status.ResourceErrors, resourceErr)
				}
			default:
				resourceErr := ReconcilerResourceError{}
				resourceErr.ResourceGroup = workspace.GetObjectKind().GroupVersionKind().Group
				resourceErr.ResourceVersion = workspace.GetObjectKind().GroupVersionKind().Version
				resourceErr.ResourceKind = workspace.GetObjectKind().GroupVersionKind().Kind
				resourceErr.ResourceName = workspace.GetName()
				resourceErr.ResourceNamespace = workspace.GetNamespace()
				resourceErr.Error = fmt.Sprintf("Workspace contains a resource with the invalid type: %#v", resource.Type)
				status.ResourceErrors = append(status.ResourceErrors, resourceErr)
			}
		}
	}

	for _, grant := range grants {
		_, err := self.findUserByName(users, grant.Spec.Grantee)
		if err != nil {
			resourceErr := ReconcilerResourceError{}
			resourceErr.ResourceGroup = grant.GetObjectKind().GroupVersionKind().Group
			resourceErr.ResourceVersion = grant.GetObjectKind().GroupVersionKind().Version
			resourceErr.ResourceKind = grant.GetObjectKind().GroupVersionKind().Kind
			resourceErr.ResourceName = grant.GetName()
			resourceErr.ResourceNamespace = grant.GetNamespace()
			resourceErr.Error = fmt.Sprintf("Grant is pointing to a user which does not exist: %#v", grant.Spec.Grantee)
			status.ResourceErrors = append(status.ResourceErrors, resourceErr)
		}

		_, err = self.findClusterRoleByRolename(clusterRoles, grant.Spec.Role)
		if err != nil {
			resourceErr := ReconcilerResourceError{}
			resourceErr.ResourceGroup = grant.GetObjectKind().GroupVersionKind().Group
			resourceErr.ResourceVersion = grant.GetObjectKind().GroupVersionKind().Version
			resourceErr.ResourceKind = grant.GetObjectKind().GroupVersionKind().Kind
			resourceErr.ResourceName = grant.GetName()
			resourceErr.ResourceNamespace = grant.GetNamespace()
			resourceErr.Error = fmt.Sprintf("Grant is pointing to a ClusterRole which does not exist: %#v", grant.Spec.Role)
			status.ResourceErrors = append(status.ResourceErrors, resourceErr)
		}

		switch grant.Spec.TargetType {
		case "workspace":
			_, err := self.findWorkspaceByName(workspaces, grant.Spec.TargetName)
			if err != nil {
				resourceErr := ReconcilerResourceError{}
				resourceErr.ResourceGroup = grant.GetObjectKind().GroupVersionKind().Group
				resourceErr.ResourceVersion = grant.GetObjectKind().GroupVersionKind().Version
				resourceErr.ResourceKind = grant.GetObjectKind().GroupVersionKind().Kind
				resourceErr.ResourceName = grant.GetName()
				resourceErr.ResourceNamespace = grant.GetNamespace()
				resourceErr.Error = fmt.Sprintf("Grant is pointing to a Workspace which does not exist: %#v", grant.Spec.TargetName)
				status.ResourceErrors = append(status.ResourceErrors, resourceErr)
			}
		default:
			resourceErr := ReconcilerResourceError{}
			resourceErr.ResourceGroup = grant.GetObjectKind().GroupVersionKind().Group
			resourceErr.ResourceVersion = grant.GetObjectKind().GroupVersionKind().Version
			resourceErr.ResourceKind = grant.GetObjectKind().GroupVersionKind().Kind
			resourceErr.ResourceName = grant.GetName()
			resourceErr.ResourceNamespace = grant.GetNamespace()
			resourceErr.Error = fmt.Sprintf("Grant contains a resource with the invalid type: %#v", grant.Spec.TargetType)
			status.ResourceErrors = append(status.ResourceErrors, resourceErr)
		}
	}

	status.ManagedRoleBindingCount = 0
	for _, rolebinding := range roleBindings {
		managedLabel := self.getLabelManagedByMogenius()
		labels := rolebinding.GetLabels()
		_, isManaged := labels[managedLabel]
		if !isManaged {
			continue // dont care for stuff that isnt managed by mogenius
		}
		_, err := self.managedRoleBindingLabelFields(labels)
		if err != nil {
			resourceErr := ReconcilerResourceError{}
			resourceErr.ResourceGroup = rolebinding.GetObjectKind().GroupVersionKind().Group
			resourceErr.ResourceVersion = rolebinding.GetObjectKind().GroupVersionKind().Version
			resourceErr.ResourceKind = rolebinding.GetObjectKind().GroupVersionKind().Kind
			resourceErr.ResourceName = rolebinding.GetName()
			resourceErr.ResourceNamespace = rolebinding.GetNamespace()
			resourceErr.Error = fmt.Sprintf("RoleBinding should contain all required managed labels as it is managed by mogenius: %s", err)
			status.ResourceErrors = append(status.ResourceErrors, resourceErr)
		}
		status.ManagedRoleBindingCount += 1
	}

	status.ManagedClusterRoleBindingCount = 0
	for _, rolebinding := range clusterRoleBindings {
		managedLabel := self.getLabelManagedByMogenius()
		labels := rolebinding.GetLabels()
		_, isManaged := labels[managedLabel]
		if !isManaged {
			continue // dont care for stuff that isnt managed by mogenius
		}
		_, err := self.managedRoleBindingLabelFields(labels)
		if err != nil {
			resourceErr := ReconcilerResourceError{}
			resourceErr.ResourceGroup = rolebinding.GetObjectKind().GroupVersionKind().Group
			resourceErr.ResourceVersion = rolebinding.GetObjectKind().GroupVersionKind().Version
			resourceErr.ResourceKind = rolebinding.GetObjectKind().GroupVersionKind().Kind
			resourceErr.ResourceName = rolebinding.GetName()
			resourceErr.ResourceNamespace = rolebinding.GetNamespace()
			resourceErr.Error = fmt.Sprintf("ClusterRoleBinding should contain all required managed labels as it is managed by mogenius: %s", err)
			status.ResourceErrors = append(status.ResourceErrors, resourceErr)
		}
		status.ManagedClusterRoleBindingCount += 1
	}

	self.statusLock.Lock()
	self.status = status
	self.statusLock.Unlock()
}

func (self *reconciler) reconcile() {
	namespaces := self.getNamespaces()
	workspaces := self.getWorkspaces()
	users := self.getUsers()
	clusterRoles := self.getClusterRoles()
	grants := self.getGrants()
	roleBindings := self.getRoleBindings()
	clusterRoleBindings := self.getClusterRoleBindings()

	requiredRoleBindings, requiredClusterRoleBindings := self.generateMissingRoleBindings(namespaces, workspaces, users, clusterRoles, grants)
	managedRoleBindings := self.filterManagedRoleBindings(roleBindings)
	managedClusterRoleBindings := self.filterManagedClusterRoleBindings(clusterRoleBindings)

	wg := sync.WaitGroup{}

	wg.Go(func() {
		self.updateStatus(
			namespaces,
			workspaces,
			users,
			clusterRoles,
			grants,
			roleBindings,
			clusterRoleBindings,
		)
	})

	wg.Go(func() {
		self.deleteObsoleteRoleBindings(
			requiredRoleBindings,
			managedRoleBindings,
			requiredClusterRoleBindings,
			managedClusterRoleBindings,
		)
	})

	wg.Go(func() {
		self.createMissingRoleBindings(
			requiredRoleBindings,
			managedRoleBindings,
			requiredClusterRoleBindings,
			managedClusterRoleBindings,
		)
	})

	wg.Wait()
}

func (self *reconciler) enableWatcher() {
	resources := append(self.crdResources, self.managedResources...)
	for _, resource := range resources {
		err := self.watcher.Watch(
			resource,
			func(resource watcher.WatcherResourceIdentifier, obj *unstructured.Unstructured) {
				assert.Assert(obj != nil)
				switch obj.GetKind() {
				case "Workspace":
					self.addWorkspace(obj)
				case "User":
					self.addUser(obj)
				case "Grant":
					self.addGrant(obj)
				case "RoleBinding":
					self.addRoleBinding(obj)
				case "ClusterRoleBinding":
					self.addClusterRoleBinding(obj)
				case "ClusterRole":
					self.addClusterRole(obj)
				case "Namespace":
					self.addNamespace(obj)
				default:
					assert.Assert(false, "Unreachable", "All allowed kinds should be handled", obj.GetKind(), obj)
				}
			},
			func(resource watcher.WatcherResourceIdentifier, oldObj *unstructured.Unstructured, newObj *unstructured.Unstructured) {
				assert.Assert(oldObj != nil)
				assert.Assert(newObj != nil)
				assert.Assert(oldObj.GetKind() == newObj.GetKind(), "The object kind should not change", oldObj, newObj)
				switch newObj.GetKind() {
				case "Workspace":
					self.removeWorkspace(oldObj)
					self.addWorkspace(newObj)
				case "User":
					self.removeUser(oldObj)
					self.addUser(newObj)
				case "Grant":
					self.removeGrant(oldObj)
					self.addGrant(newObj)
				case "RoleBinding":
					self.removeRoleBinding(oldObj)
					self.addRoleBinding(newObj)
				case "ClusterRoleBinding":
					self.removeClusterRoleBinding(oldObj)
					self.addClusterRoleBinding(newObj)
				case "ClusterRole":
					self.removeClusterRole(oldObj)
					self.addClusterRole(newObj)
				case "Namespace":
					self.removeNamespace(oldObj)
					self.addNamespace(newObj)
				default:
					assert.Assert(false, "Unreachable", "All allowed kinds should be handled", newObj.GetKind(), newObj)
				}
			},
			func(resource watcher.WatcherResourceIdentifier, obj *unstructured.Unstructured) {
				assert.Assert(obj != nil)
				switch obj.GetKind() {
				case "Workspace":
					self.removeWorkspace(obj)
				case "User":
					self.removeUser(obj)
				case "Grant":
					self.removeGrant(obj)
				case "RoleBinding":
					self.removeRoleBinding(obj)
				case "ClusterRoleBinding":
					self.removeClusterRoleBinding(obj)
				case "ClusterRole":
					self.removeClusterRole(obj)
				case "Namespace":
					self.removeNamespace(obj)
				default:
					assert.Assert(false, "Unreachable", "All allowed kinds should be handled", obj.GetKind(), obj)
				}
			},
		)
		if err != nil {
			self.logger.Error("failed to watch resource", "resource", resource, "error", err)
		}
	}
}

func (self *reconciler) disableWatcher() {
	for _, resource := range self.crdResources {
		err := self.watcher.Unwatch(resource)
		if err != nil {
			self.logger.Error("failed to unwatch resource", "resource", resource, "error", err)
		}
	}
}

func (self *reconciler) clearCaches() {
	self.workspacesLock.Lock()
	self.workspaces = []v1alpha1.Workspace{}
	self.workspacesLock.Unlock()

	self.usersLock.Lock()
	self.users = []v1alpha1.User{}
	self.usersLock.Unlock()

	self.grantsLock.Lock()
	self.grants = []v1alpha1.Grant{}
	self.grantsLock.Unlock()

	self.clusterRolesLock.Lock()
	self.clusterRoles = []rbacv1.ClusterRole{}
	self.clusterRolesLock.Unlock()

	self.clusterRoleBindingsLock.Lock()
	self.clusterRoleBindings = []rbacv1.ClusterRoleBinding{}
	self.clusterRoleBindingsLock.Unlock()

	self.roleBindingsLock.Lock()
	self.roleBindings = []rbacv1.RoleBinding{}
	self.roleBindingsLock.Unlock()
}

func (self *reconciler) getWorkspaces() []v1alpha1.Workspace {
	self.workspacesLock.Lock()
	defer self.workspacesLock.Unlock()

	workspaces := make([]v1alpha1.Workspace, len(self.workspaces))
	for idx, workspace := range self.workspaces {
		workspaces[idx] = *workspace.DeepCopy()
	}

	return workspaces
}

func (self *reconciler) addWorkspace(obj *unstructured.Unstructured) {
	var workspace v1alpha1.Workspace
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &workspace)
	assert.Assert(err == nil, "failed to cast unstructured object as workspace", err, obj)

	self.workspacesLock.Lock()
	defer self.workspacesLock.Unlock()
	self.workspaces = append(self.workspaces, workspace)
}

func (self *reconciler) removeWorkspace(obj *unstructured.Unstructured) {
	self.workspacesLock.Lock()
	defer self.workspacesLock.Unlock()

	needle := -1
	for idx, workspace := range self.workspaces {
		if obj.GetName() == workspace.Name {
			needle = idx
			break
		}
	}

	if needle == -1 {
		return
	}

	self.workspaces = append(self.workspaces[:needle], self.workspaces[needle+1:]...)
}

func (self *reconciler) getUsers() []v1alpha1.User {
	self.usersLock.Lock()
	defer self.usersLock.Unlock()

	users := make([]v1alpha1.User, len(self.users))
	for idx, user := range self.users {
		users[idx] = *user.DeepCopy()
	}

	return users
}

func (self *reconciler) addUser(obj *unstructured.Unstructured) {
	var user v1alpha1.User
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &user)
	assert.Assert(err == nil, "failed to cast unstructured object as user", err, obj)

	self.usersLock.Lock()
	defer self.usersLock.Unlock()
	self.users = append(self.users, user)
}

func (self *reconciler) removeUser(obj *unstructured.Unstructured) {
	self.usersLock.Lock()
	defer self.usersLock.Unlock()

	needle := -1
	for idx, user := range self.users {
		if obj.GetName() == user.Name {
			needle = idx
			break
		}
	}

	if needle == -1 {
		return
	}

	self.users = append(self.users[:needle], self.users[needle+1:]...)
}

func (self *reconciler) getGrants() []v1alpha1.Grant {
	self.grantsLock.Lock()
	defer self.grantsLock.Unlock()

	grants := make([]v1alpha1.Grant, len(self.grants))
	for idx, grant := range self.grants {
		grants[idx] = *grant.DeepCopy()
	}

	return grants
}

func (self *reconciler) addGrant(obj *unstructured.Unstructured) {
	var grant v1alpha1.Grant
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &grant)
	assert.Assert(err == nil, "failed to cast unstructured object as grant", err, obj)

	self.grantsLock.Lock()
	defer self.grantsLock.Unlock()
	self.grants = append(self.grants, grant)
}

func (self *reconciler) removeGrant(obj *unstructured.Unstructured) {
	self.grantsLock.Lock()
	defer self.grantsLock.Unlock()

	needle := -1
	for idx, grant := range self.grants {
		if obj.GetName() == grant.Name {
			needle = idx
			break
		}
	}

	if needle == -1 {
		return
	}

	self.grants = append(self.grants[:needle], self.grants[needle+1:]...)
}

func (self *reconciler) getRoleBindings() []rbacv1.RoleBinding {
	self.roleBindingsLock.Lock()
	defer self.roleBindingsLock.Unlock()

	roleBindings := make([]rbacv1.RoleBinding, len(self.roleBindings))
	for idx, roleBinding := range self.roleBindings {
		roleBindings[idx] = *roleBinding.DeepCopy()
	}

	return roleBindings
}

func (self *reconciler) addRoleBinding(obj *unstructured.Unstructured) {
	var roleBinding rbacv1.RoleBinding
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &roleBinding)
	assert.Assert(err == nil, "failed to cast unstructured object as roleBinding", err, obj)

	self.roleBindingsLock.Lock()
	defer self.roleBindingsLock.Unlock()
	self.roleBindings = append(self.roleBindings, roleBinding)
}

func (self *reconciler) removeRoleBinding(obj *unstructured.Unstructured) {
	self.roleBindingsLock.Lock()
	defer self.roleBindingsLock.Unlock()

	needle := -1
	for idx, roleBinding := range self.roleBindings {
		if obj.GetName() == roleBinding.Name {
			needle = idx
			break
		}
	}

	if needle == -1 {
		return
	}

	self.roleBindings = append(self.roleBindings[:needle], self.roleBindings[needle+1:]...)
}

func (self *reconciler) getClusterRoleBindings() []rbacv1.ClusterRoleBinding {
	self.clusterRoleBindingsLock.Lock()
	defer self.clusterRoleBindingsLock.Unlock()

	clusterRoleBindings := make([]rbacv1.ClusterRoleBinding, len(self.clusterRoleBindings))
	for idx, clusterRoleBinding := range self.clusterRoleBindings {
		clusterRoleBindings[idx] = *clusterRoleBinding.DeepCopy()
	}

	return clusterRoleBindings
}

func (self *reconciler) addClusterRoleBinding(obj *unstructured.Unstructured) {
	var clusterRoleBinding rbacv1.ClusterRoleBinding
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &clusterRoleBinding)
	assert.Assert(err == nil, "failed to cast unstructured object as clusterRoleBinding", err, obj)

	self.clusterRoleBindingsLock.Lock()
	defer self.clusterRoleBindingsLock.Unlock()
	self.clusterRoleBindings = append(self.clusterRoleBindings, clusterRoleBinding)
}

func (self *reconciler) removeClusterRoleBinding(obj *unstructured.Unstructured) {
	self.clusterRoleBindingsLock.Lock()
	defer self.clusterRoleBindingsLock.Unlock()

	needle := -1
	for idx, clusterRoleBinding := range self.clusterRoleBindings {
		if obj.GetName() == clusterRoleBinding.Name {
			needle = idx
			break
		}
	}

	if needle == -1 {
		return
	}

	self.clusterRoleBindings = append(self.clusterRoleBindings[:needle], self.clusterRoleBindings[needle+1:]...)
}

func (self *reconciler) getClusterRoles() []rbacv1.ClusterRole {
	self.clusterRolesLock.Lock()
	defer self.clusterRolesLock.Unlock()

	clusterRoles := make([]rbacv1.ClusterRole, len(self.clusterRoles))
	for idx, clusterRole := range self.clusterRoles {
		clusterRoles[idx] = *clusterRole.DeepCopy()
	}

	return clusterRoles
}

func (self *reconciler) addClusterRole(obj *unstructured.Unstructured) {
	var clusterRole rbacv1.ClusterRole
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &clusterRole)
	assert.Assert(err == nil, "failed to cast unstructured object as clusterRole", err, obj)

	self.clusterRolesLock.Lock()
	defer self.clusterRolesLock.Unlock()
	self.clusterRoles = append(self.clusterRoles, clusterRole)
}

func (self *reconciler) removeClusterRole(obj *unstructured.Unstructured) {
	self.clusterRolesLock.Lock()
	defer self.clusterRolesLock.Unlock()

	needle := -1
	for idx, clusterRole := range self.clusterRoles {
		if obj.GetName() == clusterRole.Name {
			needle = idx
			break
		}
	}

	if needle == -1 {
		return
	}

	self.clusterRoles = append(self.clusterRoles[:needle], self.clusterRoles[needle+1:]...)
}

func (self *reconciler) getNamespaces() []v1.Namespace {
	self.namespacesLock.Lock()
	defer self.namespacesLock.Unlock()

	namespaces := make([]v1.Namespace, len(self.namespaces))
	for idx, namespace := range self.namespaces {
		namespaces[idx] = *namespace.DeepCopy()
	}

	return namespaces
}

func (self *reconciler) addNamespace(obj *unstructured.Unstructured) {
	var namespace v1.Namespace
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &namespace)
	assert.Assert(err == nil, "failed to cast unstructured object as namespace", err, obj)

	self.namespacesLock.Lock()
	defer self.namespacesLock.Unlock()
	self.namespaces = append(self.namespaces, namespace)
}

func (self *reconciler) removeNamespace(obj *unstructured.Unstructured) {
	self.namespacesLock.Lock()
	defer self.namespacesLock.Unlock()

	needle := -1
	for idx, namespace := range self.namespaces {
		if obj.GetName() == namespace.Name {
			needle = idx
			break
		}
	}

	if needle == -1 {
		return
	}

	self.namespaces = append(self.namespaces[:needle], self.namespaces[needle+1:]...)
}

func (self *reconciler) deleteObsoleteRoleBindings(
	requiredRoleBindings []rbacv1.RoleBinding,
	managedRoleBindings []rbacv1.RoleBinding,
	requiredClusterRoleBindings []rbacv1.ClusterRoleBinding,
	managedClusterRoleBindings []rbacv1.ClusterRoleBinding,
) {
	superfluousRoleBindings, superfluousClusterRoleBindings := self.findSuperfluousRoleBindings(
		requiredRoleBindings,
		managedRoleBindings,
		requiredClusterRoleBindings,
		managedClusterRoleBindings,
	)

	client := self.clientProvider.K8sClientSet()

	for _, rb := range superfluousRoleBindings {
		err := client.RbacV1().RoleBindings(rb.Namespace).Delete(context.Background(), rb.GetName(), metav1.DeleteOptions{})
		if err != nil {
			self.logger.Error("failed to delete rolebinding", "rolebinding", rb, "error", err)
			continue
		}
		self.logger.Debug("deleted resource", "RoleBinding.Name", rb.GetName(), "RoleBinding.Namespace", rb.GetNamespace())
	}

	for _, rb := range superfluousClusterRoleBindings {
		err := client.RbacV1().ClusterRoleBindings().Delete(context.Background(), rb.GetName(), metav1.DeleteOptions{})
		if err != nil {
			self.logger.Error("failed to delete clusterrolebinding", "rolebinding", rb, "error", err)
			continue
		}
		self.logger.Debug("deleted resource", "ClusterRoleBinding.Name", rb.GetName())
	}
}

func (self *reconciler) createMissingRoleBindings(
	requiredRoleBindings []rbacv1.RoleBinding,
	managedRoleBindings []rbacv1.RoleBinding,
	requiredClusterRoleBindings []rbacv1.ClusterRoleBinding,
	managedClusterRoleBindings []rbacv1.ClusterRoleBinding,
) {
	newRoleBindings, newClusterRoleBindings := self.findMissingRoleBindings(
		requiredRoleBindings,
		managedRoleBindings,
		requiredClusterRoleBindings,
		managedClusterRoleBindings,
	)

	client := self.clientProvider.K8sClientSet()

	for _, rb := range newRoleBindings {
		_, err := client.RbacV1().RoleBindings(rb.Namespace).Update(context.Background(), &rb, metav1.UpdateOptions{})
		if err != nil {
			self.logger.Error("failed to create rolebinding", "rolebinding", rb, "error", err)
			continue
		}
		self.logger.Debug("created resource", "RoleBinding.Name", rb.GetName(), "RoleBinding.Namespace", rb.GetNamespace())
	}
	for _, rb := range newClusterRoleBindings {
		_, err := client.RbacV1().ClusterRoleBindings().Update(context.Background(), &rb, metav1.UpdateOptions{})
		if err != nil {
			self.logger.Error("failed to create rolebinding", "rolebinding", rb, "error", err)
			continue
		}
		self.logger.Debug("created resource", "ClusterRoleBinding.Name", rb.GetName())
	}
}

func (self *reconciler) generateMissingRoleBindings(
	namespaces []v1.Namespace,
	workspaces []v1alpha1.Workspace,
	users []v1alpha1.User,
	clusterRoles []rbacv1.ClusterRole,
	grants []v1alpha1.Grant,
) ([]rbacv1.RoleBinding, []rbacv1.ClusterRoleBinding) {
	requiredRoleBindings := []rbacv1.RoleBinding{}
	requiredClusterRoleBindings := []rbacv1.ClusterRoleBinding{}

	for _, grant := range grants {
		user, err := self.findUserByName(users, grant.Spec.Grantee)
		if err != nil {
			// the referenced user does not exist
			continue
		}
		if user.Spec.Subject == nil {
			// no rolebinding needed as the user has no identity in the cluster
			continue
		}
		clusterRole, err := self.findClusterRoleByRolename(clusterRoles, grant.Spec.Role)
		if err != nil {
			// the referenced clusterrole does not exist
			continue
		}
		switch grant.Spec.TargetType {
		case "workspace":
			workspace, err := self.findWorkspaceByName(workspaces, grant.Spec.TargetName)
			if err != nil {
				// the referenced workspace does not exist
				continue
			}
			for _, resource := range workspace.Spec.Resources {
				namespace := ""
				switch resource.Type {
				case "namespace":
					namespace = resource.Id
				case "helm":
					// assigning permissions to helm charts means allowing access to the namespace they are installed in
					namespace = resource.Namespace
				default:
					// invalid type of resource reference
					continue
				}
				if !slices.ContainsFunc(namespaces, func(ns v1.Namespace) bool {
					return ns.GetName() == namespace
				}) {
					// the namespace doesn't exist
					continue
				}
				roleBinding := rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mogenius-rb-" + self.generateManagedRoleBindingNameSuffixGrantResource(grant, resource),
						Namespace: namespace,
						Labels:    self.createManagedRoleBindingLabels(grant, resource),
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     clusterRole.GetName(),
					},
					Subjects: []rbacv1.Subject{*user.Spec.Subject},
				}
				requiredRoleBindings = append(requiredRoleBindings, roleBinding)
			}
		default:
			continue
		}
	}

	return requiredRoleBindings, requiredClusterRoleBindings
}

func (self *reconciler) filterManagedRoleBindings(rolebindings []rbacv1.RoleBinding) []rbacv1.RoleBinding {
	managedRoleBindings := make([]rbacv1.RoleBinding, 0, len(rolebindings))

	for _, rb := range rolebindings {
		labels := rb.GetLabels()
		_, ok := labels[self.getLabelManagedByMogenius()]
		if !ok {
			continue
		}
		_, err := self.managedRoleBindingLabelFields(rb.GetLabels())
		if err != nil {
			continue
		}

		managedRoleBindings = append(managedRoleBindings, rb)
	}

	return managedRoleBindings
}

func (self *reconciler) filterManagedClusterRoleBindings(rolebindings []rbacv1.ClusterRoleBinding) []rbacv1.ClusterRoleBinding {
	managedRoleBindings := make([]rbacv1.ClusterRoleBinding, 0, len(rolebindings))

	for _, rb := range rolebindings {
		labels := rb.GetLabels()
		_, ok := labels[self.getLabelManagedByMogenius()]
		if !ok {
			continue
		}
		_, err := self.managedRoleBindingLabelFields(rb.GetLabels())
		if err != nil {
			continue
		}

		managedRoleBindings = append(managedRoleBindings, rb)
	}

	return managedRoleBindings
}

func (self *reconciler) findSuperfluousRoleBindings(
	requiredRoleBindings []rbacv1.RoleBinding,
	existingRoleBindings []rbacv1.RoleBinding,
	requiredClusterRoleBindings []rbacv1.ClusterRoleBinding,
	existingClusterRoleBindings []rbacv1.ClusterRoleBinding,
) ([]rbacv1.RoleBinding, []rbacv1.ClusterRoleBinding) {
	superfluousRoleBindings := make([]rbacv1.RoleBinding, 0, len(requiredRoleBindings))
	superfluousClusterRoleBindings := make([]rbacv1.ClusterRoleBinding, 0, len(requiredClusterRoleBindings))

	for _, erb := range existingRoleBindings {
		erbManagedFields, err := self.managedRoleBindingLabelFields(erb.GetLabels())
		if err != nil {
			continue
		}
		if len(erb.Subjects) != 1 {
			continue // the managed rolebindings generated by mogenius always have a single subject
		}
		erbSubject := &erb.Subjects[0]
		found := false
		for _, rb := range requiredRoleBindings {
			rbManagedFields, err := self.managedRoleBindingLabelFields(rb.GetLabels())
			if err != nil {
				continue
			}
			if len(rb.Subjects) != 1 {
				continue // the managed rolebindings generated by mogenius always have a single subject
			}
			rbSubject := &rb.Subjects[0]
			if erbManagedFields.equals(&rbManagedFields) && self.subjectsEqual(rbSubject, erbSubject) {
				found = true
				break
			}
		}
		if !found {
			superfluousRoleBindings = append(superfluousRoleBindings, erb)
		}
	}

	for _, erb := range existingClusterRoleBindings {
		erbManagedFields, err := self.managedRoleBindingLabelFields(erb.GetLabels())
		if err != nil {
			continue
		}
		if len(erb.Subjects) != 1 {
			continue // the managed rolebindings generated by mogenius always have a single subject
		}
		erbSubject := &erb.Subjects[0]
		found := false
		for _, rb := range requiredClusterRoleBindings {
			rbManagedFields, err := self.managedRoleBindingLabelFields(rb.GetLabels())
			if err != nil {
				continue
			}
			if len(rb.Subjects) != 1 {
				continue // the managed rolebindings generated by mogenius always have a single subject
			}
			rbSubject := &rb.Subjects[0]
			if erbManagedFields.equals(&rbManagedFields) && self.subjectsEqual(rbSubject, erbSubject) {
				found = true
				break
			}
		}
		if !found {
			superfluousClusterRoleBindings = append(superfluousClusterRoleBindings, erb)
		}
	}

	return superfluousRoleBindings, superfluousClusterRoleBindings
}

func (self *reconciler) subjectsEqual(a *rbacv1.Subject, b *rbacv1.Subject) bool {
	// nil checks
	if a == nil && b == nil {
		return true // both are nil
	}
	if (a == nil) != (b == nil) {
		return false // one is nil and one isnt
	}

	// previous checks should guarantee both values are set
	assert.Assert(a != nil)
	assert.Assert(b != nil)

	// check all fields for equality
	if a.APIGroup != b.APIGroup {
		return false
	}
	if a.Kind != b.Kind {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if a.Namespace != b.Namespace {
		return false
	}

	return true
}

func (self *reconciler) findMissingRoleBindings(
	requiredRoleBindings []rbacv1.RoleBinding,
	existingRoleBindings []rbacv1.RoleBinding,
	requiredClusterRoleBindings []rbacv1.ClusterRoleBinding,
	existingClusterRoleBindings []rbacv1.ClusterRoleBinding,
) ([]rbacv1.RoleBinding, []rbacv1.ClusterRoleBinding) {
	filteredRoleBindings := make([]rbacv1.RoleBinding, 0, len(requiredRoleBindings))
	filteredClusterRoleBindings := make([]rbacv1.ClusterRoleBinding, 0, len(requiredClusterRoleBindings))

	for _, rb := range requiredRoleBindings {
		rbManagedFields, err := self.managedRoleBindingLabelFields(rb.GetLabels())
		if err != nil {
			continue
		}
		if len(rb.Subjects) != 1 {
			continue // the managed rolebindings generated by mogenius always have a single subject
		}
		rbSubject := &rb.Subjects[0]
		found := false
		for _, erb := range existingRoleBindings {
			erbManagedFields, err := self.managedRoleBindingLabelFields(erb.GetLabels())
			if err != nil {
				continue
			}
			if len(erb.Subjects) != 1 {
				continue // the managed rolebindings generated by mogenius always have a single subject
			}
			erbSubject := &erb.Subjects[0]
			if rbManagedFields.equals(&erbManagedFields) && self.subjectsEqual(rbSubject, erbSubject) {
				found = true
				break
			}
		}
		if !found {
			filteredRoleBindings = append(filteredRoleBindings, rb)
		}
	}

	for _, rb := range requiredClusterRoleBindings {
		rbManagedFields, err := self.managedRoleBindingLabelFields(rb.GetLabels())
		assert.Assert(err == nil, "the newly constructed and not yet applied managed resource should not be lacking a field", err)
		if len(rb.Subjects) != 1 {
			continue // the managed rolebindings generated by mogenius always have a single subject
		}
		rbSubject := &rb.Subjects[0]
		found := false
		for _, erb := range existingClusterRoleBindings {
			erbManagedFields, err := self.managedRoleBindingLabelFields(erb.GetLabels())
			if err != nil {
				continue
			}
			if len(erb.Subjects) != 1 {
				continue // the managed rolebindings generated by mogenius always have a single subject
			}
			erbSubject := &erb.Subjects[0]
			if rbManagedFields.equals(&erbManagedFields) && self.subjectsEqual(rbSubject, erbSubject) {
				found = true
				break
			}
		}
		if !found {
			filteredClusterRoleBindings = append(filteredClusterRoleBindings, rb)
		}
	}

	return filteredRoleBindings, filteredClusterRoleBindings
}

func (self *reconciler) getLabelOrg() string {
	return "app.mogenius.com"
}

func (self *reconciler) getLabelManagedByMogenius() string {
	return self.getLabelOrg() + "/" + "managed-by-mogenius"
}

func (self *reconciler) getLabelRoleName() string {
	return self.getLabelOrg() + "/" + "role-name"
}

func (self *reconciler) findWorkspaceByName(workspaces []v1alpha1.Workspace, name string) (v1alpha1.Workspace, error) {
	for _, workspace := range workspaces {
		if workspace.GetName() == name {
			return workspace, nil
		}
	}

	return v1alpha1.Workspace{}, fmt.Errorf("workspace not found")
}

func (self *reconciler) findUserByName(users []v1alpha1.User, name string) (v1alpha1.User, error) {
	for _, user := range users {
		if user.GetObjectMeta().GetName() == name {
			return user, nil
		}
	}

	return v1alpha1.User{}, fmt.Errorf("user not found")
}

func (self *reconciler) findNamespaceByName(namespaces []v1.Namespace, name string) (v1.Namespace, error) {
	for _, namespace := range namespaces {
		if namespace.GetName() == name {
			return namespace, nil
		}
	}

	return v1.Namespace{}, fmt.Errorf("namespace not found")
}

func (self *reconciler) findClusterRoleByRolename(clusterRoles []rbacv1.ClusterRole, rolename string) (rbacv1.ClusterRole, error) {
	// check for matches in labels
	for _, clusterRole := range clusterRoles {
		labels := clusterRole.GetObjectMeta().GetLabels()
		for key, val := range labels {
			if key == self.getLabelRoleName() {
				if val == rolename {
					return clusterRole, nil
				}
				break
			}
		}
	}

	// continue search by regular resource names
	for _, clusterRole := range clusterRoles {
		if clusterRole.GetObjectMeta().GetName() == rolename {
			return clusterRole, nil
		}
	}

	return rbacv1.ClusterRole{}, fmt.Errorf("clusterrole not found")
}

func (self *reconciler) generateManagedRoleBindingNameSuffixGrantResource(
	grant v1alpha1.Grant,
	resource v1alpha1.WorkspaceResourceIdentifier,
) string {
	h := fnv.New64a()
	h.Write([]byte(grant.Spec.Grantee))
	h.Write([]byte(grant.Spec.TargetType))
	h.Write([]byte(grant.Spec.TargetName))
	h.Write([]byte(grant.Spec.Role))
	h.Write([]byte(resource.Id))
	h.Write([]byte(resource.Type))
	h.Write([]byte(resource.Namespace))
	return fmt.Sprintf("%x", h.Sum64())
}

type ManagedRoleBindingLabelFields struct {
	GrantGrantee      string
	GrantTargetType   string
	GrantTargetName   string
	GrantRole         string
	ResourceId        string
	ResourceType      string
	ResourceNamespace string
}

func (self *ManagedRoleBindingLabelFields) equals(other *ManagedRoleBindingLabelFields) bool {
	if self.GrantGrantee != other.GrantGrantee {
		return false
	}
	if self.GrantTargetType != other.GrantTargetType {
		return false
	}
	if self.GrantTargetName != other.GrantTargetName {
		return false
	}
	if self.GrantRole != other.GrantRole {
		return false
	}
	if self.ResourceId != other.ResourceId {
		return false
	}
	if self.ResourceType != other.ResourceType {
		return false
	}
	if self.ResourceNamespace != other.ResourceNamespace {
		return false
	}

	return true
}

func (self *reconciler) getReconcilerLabelGrantGrantee() string {
	return self.getLabelOrg() + "/grant-grantee"
}

func (self *reconciler) getReconcilerLabelGrantTargetType() string {
	return self.getLabelOrg() + "/grant-target-type"
}

func (self *reconciler) getReconcilerLabelGrantTargetName() string {
	return self.getLabelOrg() + "/grant-target-name"
}

func (self *reconciler) getReconcilerLabelGrantRole() string {
	return self.getLabelOrg() + "/grant-role"
}

func (self *reconciler) getReconcilerLabelResourceId() string {
	return self.getLabelOrg() + "/resource-id"
}

func (self *reconciler) getReconcilerLabelResourceType() string {
	return self.getLabelOrg() + "/resource-type"
}

func (self *reconciler) getReconcilerLabelResourceNamespace() string {
	return self.getLabelOrg() + "/resource-namespace"
}

func (self *reconciler) createManagedRoleBindingLabels(
	grant v1alpha1.Grant,
	resource v1alpha1.WorkspaceResourceIdentifier,
) map[string]string {
	labels := make(map[string]string)

	labels[self.getLabelManagedByMogenius()] = ""
	labels[self.getReconcilerLabelGrantGrantee()] = grant.Spec.Grantee
	labels[self.getReconcilerLabelGrantTargetType()] = grant.Spec.TargetType
	labels[self.getReconcilerLabelGrantTargetName()] = grant.Spec.TargetName
	labels[self.getReconcilerLabelGrantRole()] = grant.Spec.Role
	labels[self.getReconcilerLabelResourceId()] = resource.Id
	labels[self.getReconcilerLabelResourceType()] = resource.Type
	labels[self.getReconcilerLabelResourceNamespace()] = resource.Namespace

	return labels
}

func (self *reconciler) managedRoleBindingLabelFields(labels map[string]string) (ManagedRoleBindingLabelFields, error) {
	data := ManagedRoleBindingLabelFields{}

	grantee, ok := labels[self.getReconcilerLabelGrantGrantee()]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: grant-grantee")
	}
	data.GrantGrantee = grantee

	targetType, ok := labels[self.getReconcilerLabelGrantTargetType()]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: grant-target-type")
	}
	data.GrantTargetType = targetType

	targetName, ok := labels[self.getReconcilerLabelGrantTargetName()]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: grant-target-name")
	}
	data.GrantTargetName = targetName

	role, ok := labels[self.getReconcilerLabelGrantRole()]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: grant-role")
	}
	data.GrantRole = role

	resourceId, ok := labels[self.getReconcilerLabelResourceId()]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: resource-id")
	}
	data.ResourceId = resourceId

	resourceType, ok := labels[self.getReconcilerLabelResourceType()]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: resource-type")
	}
	data.ResourceType = resourceType

	resourceNamespace, ok := labels[self.getReconcilerLabelResourceNamespace()]
	if !ok {
		return ManagedRoleBindingLabelFields{}, fmt.Errorf("missing label: resource-namespace")
	}
	data.ResourceNamespace = resourceNamespace

	return data, nil
}
