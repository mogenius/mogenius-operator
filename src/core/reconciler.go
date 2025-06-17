package core

import (
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/crds/v1alpha1"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/watcher"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Reconciler interface {
	Link(leaderElector LeaderElector)
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

	self.crdResources = []watcher.WatcherResourceIdentifier{
		{Name: "workspaces", Kind: "Workspace", Version: "", GroupVersion: "mogenius.com/v1alpha1", Namespaced: false},
		{Name: "users", Kind: "User", Version: "", GroupVersion: "mogenius.com/v1alpha1", Namespaced: false},
		{Name: "grants", Kind: "Grant", Version: "", GroupVersion: "mogenius.com/v1alpha1", Namespaced: false},
	}
	self.workspaces = []v1alpha1.Workspace{}
	self.workspacesLock = sync.RWMutex{}
	self.grants = []v1alpha1.Grant{}
	self.grantsLock = sync.RWMutex{}
	self.users = []v1alpha1.User{}
	self.usersLock = sync.RWMutex{}

	self.managedResources = []watcher.WatcherResourceIdentifier{
		{Name: "clusterroles", Kind: "ClusterRole", Version: "", GroupVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
		{Name: "clusterrolebindings", Kind: "ClusterRoleBinding", Version: "", GroupVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
		{Name: "rolebindings", Kind: "RoleBinding", Version: "", GroupVersion: "rbac.authorization.k8s.io/v1", Namespaced: false},
	}
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

		for {
			select {
			case <-updateTicker.C:
				if !self.active.Load() {
					continue
				}
				self.reconcile()
			}
		}
	}()
}

func (self *reconciler) Start() {
	self.clearCaches()
	self.enableWatcher()
	self.active.Store(true)
}

func (self *reconciler) Stop() {
	self.active.Store(false)
	self.disableWatcher()
	self.clearCaches()
}

func (self *reconciler) reconcile() {
	// startTime := time.Now()

	workspaces := self.getWorkspaces()
	users := self.getUsers()
	clusterRoles := self.getClusterRoles()
	grants := self.getGrants()
	roleBindings := self.getRoleBindings()
	clusterRoleBindings := self.getClusterRoleBindings()

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		self.setReconcilerState(
			workspaces,
			users,
			clusterRoles,
			grants,
			roleBindings,
			clusterRoleBindings,
		)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		self.deleteObsoleteRoleBindings(
			workspaces,
			users,
			clusterRoles,
			grants,
			roleBindings,
			clusterRoleBindings,
		)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		self.createMissingRoleBindings(
			workspaces,
			users,
			clusterRoles,
			grants,
			roleBindings,
			clusterRoleBindings,
		)
		wg.Done()
	}()

	wg.Wait()

	// updateTime := time.Since(startTime)
	// updateTimeSeconds := updateTime.Seconds()

	// if updateTimeSeconds < 2 {
	// 	self.logger.Info("reconciler update finished", "updateTimeSeconds", updateTimeSeconds)
	// } else if updateTimeSeconds < 5 {
	// 	self.logger.Warn("reconciler update finished", "updateTimeSeconds", updateTimeSeconds)
	// } else {
	// 	self.logger.Error("reconciler update finished", "updateTimeSeconds", updateTimeSeconds)
	// }
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

func (self *reconciler) setReconcilerState(
	workspaces []v1alpha1.Workspace,
	users []v1alpha1.User,
	clusterRoles []rbacv1.ClusterRole,
	grants []v1alpha1.Grant,
	roleBindings []rbacv1.RoleBinding,
	clusterRoleBindings []rbacv1.ClusterRoleBinding,
) error {
	// self.logger.Info(
	// 	"TICK: reconciler.setReconcilerState",
	// 	"workspaces", len(workspaces),
	// 	"users", len(users),
	// 	"clusterRoles", len(clusterRoles),
	// 	"grants", len(grants),
	// 	"roleBindings", len(roleBindings),
	// 	"clusterRoleBindings", len(clusterRoleBindings),
	// )

	// check if all grants are pointing to existing users
	// collect all errors
	// store all errors
	// provide information for healthcheck

	return nil
}

func (self *reconciler) deleteObsoleteRoleBindings(
	workspaces []v1alpha1.Workspace,
	users []v1alpha1.User,
	clusterRoles []rbacv1.ClusterRole,
	grants []v1alpha1.Grant,
	roleBindings []rbacv1.RoleBinding,
	clusterRoleBindings []rbacv1.ClusterRoleBinding,
) error {
	// self.logger.Info(
	// 	"TICK: reconciler.deleteObsoleteRoleBindings",
	// 	"workspaces", len(workspaces),
	// 	"users", len(users),
	// 	"clusterRoles", len(clusterRoles),
	// 	"grants", len(grants),
	// 	"roleBindings", len(roleBindings),
	// 	"clusterRoleBindings", len(clusterRoleBindings),
	// )

	// create a list of expected rolebindings
	//   skip grants which point to non-existent sources
	// create a list of expected clusterrolebindings
	//   skip grants which point to non-existent sources
	// check if every existing rolebinding (with "managed-by:mogenius") should exist
	//   delete if not
	// check if every existing clusterrolebinding (with "managed-by:mogenius") should exist
	//   delete if not

	return nil
}

func (self *reconciler) createMissingRoleBindings(
	workspaces []v1alpha1.Workspace,
	users []v1alpha1.User,
	clusterRoles []rbacv1.ClusterRole,
	grants []v1alpha1.Grant,
	roleBindings []rbacv1.RoleBinding,
	clusterRoleBindings []rbacv1.ClusterRoleBinding,
) error {
	// self.logger.Info(
	// 	"TICK: reconciler.createMissingRoleBindings",
	// 	"workspaces", len(workspaces),
	// 	"users", len(users),
	// 	"clusterRoles", len(clusterRoles),
	// 	"grants", len(grants),
	// 	"roleBindings", len(roleBindings),
	// 	"clusterRoleBindings", len(clusterRoleBindings),
	// )

	// create a list of expected rolebindings
	//   skip grants which point to non-existent sources
	// create a list of expected clusterrolebindings
	//   skip grants which point to non-existent sources
	// check if all expected rolebindings exist
	//   create if not
	// check if all expected clusterrolebindings exist
	//   create if not

	return nil
}
