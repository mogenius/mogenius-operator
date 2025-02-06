package core

import (
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/crds/v1alpha1"
	"mogenius-k8s-manager/src/k8sclient"
	"sync"
)

type WorkspaceManager interface {
	GetAllWorkspaces() ([]v1alpha1.Workspace, error)
	CreateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (*v1alpha1.Workspace, error)
	GetWorkspace(name string) (*v1alpha1.Workspace, error)
	UpdateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (*v1alpha1.Workspace, error)
	DeleteWorkspace(name string) error

	GetAllUsers() ([]v1alpha1.User, error)
	CreateUser(name string, spec v1alpha1.UserSpec) (*v1alpha1.User, error)
	GetUser(name string) (*v1alpha1.User, error)
	UpdateUser(name string, spec v1alpha1.UserSpec) (*v1alpha1.User, error)
	DeleteUser(name string) error

	GetAllGroups() ([]v1alpha1.Group, error)
	CreateGroup(name string, spec v1alpha1.GroupSpec) (*v1alpha1.Group, error)
	GetGroup(name string) (*v1alpha1.Group, error)
	UpdateGroup(name string, spec v1alpha1.GroupSpec) (*v1alpha1.Group, error)
	DeleteGroup(name string) error

	GetAllPermissions() ([]v1alpha1.Permission, error)
	CreatePermission(name string, spec v1alpha1.PermissionSpec) (*v1alpha1.Permission, error)
	GetPermission(name string) (*v1alpha1.Permission, error)
	UpdatePermission(name string, spec v1alpha1.PermissionSpec) (*v1alpha1.Permission, error)
	DeletePermission(name string) error
}

type workspaceManager struct {
	config            config.ConfigModule
	clientProvider    k8sclient.K8sClientProvider
	mogeniusClientSet *crds.MogeniusClientSet
	namespace         string
	namespaceLock     sync.RWMutex
}

func NewWorkspaceManager(configModule config.ConfigModule, clientProvider k8sclient.K8sClientProvider) WorkspaceManager {
	wm := &workspaceManager{}
	wm.clientProvider = clientProvider
	wm.mogeniusClientSet = clientProvider.MogeniusClientSet()
	wm.config = configModule
	wm.namespace = configModule.Get("MO_OWN_NAMESPACE")
	wm.namespaceLock = sync.RWMutex{}

	configModule.OnChanged([]string{"MO_OWN_NAMESPACE"}, func(key string, value string, _isSecret bool) {
		if key == "MO_OWN_NAMESPACE" {
			wm.namespaceLock.Lock()
			wm.namespace = value
			wm.namespaceLock.Unlock()
		}
	})

	return wm
}

func (self *workspaceManager) GetAllWorkspaces() ([]v1alpha1.Workspace, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.ListWorkspaces(self.namespace)
}

func (self *workspaceManager) GetWorkspace(name string) (*v1alpha1.Workspace, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.GetWorkspace(self.namespace, name)
}

func (self *workspaceManager) CreateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (*v1alpha1.Workspace, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.CreateWorkspace(self.namespace, name, spec)
}

func (self *workspaceManager) UpdateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (*v1alpha1.Workspace, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.UpdateWorkspace(self.namespace, name, spec)
}

func (self *workspaceManager) DeleteWorkspace(name string) error {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.DeleteWorkspace(self.namespace, name)
}

func (self *workspaceManager) GetAllUsers() ([]v1alpha1.User, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.ListUsers(self.namespace)
}

func (self *workspaceManager) GetUser(name string) (*v1alpha1.User, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.GetUser(self.namespace, name)
}

func (self *workspaceManager) CreateUser(name string, spec v1alpha1.UserSpec) (*v1alpha1.User, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.CreateUser(self.namespace, name, spec)
}

func (self *workspaceManager) UpdateUser(name string, spec v1alpha1.UserSpec) (*v1alpha1.User, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.UpdateUser(self.namespace, name, spec)
}

func (self *workspaceManager) DeleteUser(name string) error {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.DeleteUser(self.namespace, name)
}

func (self *workspaceManager) GetAllGroups() ([]v1alpha1.Group, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.ListGroups(self.namespace)
}

func (self *workspaceManager) GetGroup(name string) (*v1alpha1.Group, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.GetGroup(self.namespace, name)
}

func (self *workspaceManager) CreateGroup(name string, spec v1alpha1.GroupSpec) (*v1alpha1.Group, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.CreateGroup(self.namespace, name, spec)
}

func (self *workspaceManager) UpdateGroup(name string, spec v1alpha1.GroupSpec) (*v1alpha1.Group, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.UpdateGroup(self.namespace, name, spec)
}

func (self *workspaceManager) DeleteGroup(name string) error {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.DeleteGroup(self.namespace, name)
}

func (self *workspaceManager) GetAllPermissions() ([]v1alpha1.Permission, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.ListPermissions(self.namespace)
}

func (self *workspaceManager) GetPermission(name string) (*v1alpha1.Permission, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.GetPermission(self.namespace, name)
}

func (self *workspaceManager) CreatePermission(name string, spec v1alpha1.PermissionSpec) (*v1alpha1.Permission, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.CreatePermission(self.namespace, name, spec)
}

func (self *workspaceManager) UpdatePermission(name string, spec v1alpha1.PermissionSpec) (*v1alpha1.Permission, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.UpdatePermission(self.namespace, name, spec)
}

func (self *workspaceManager) DeletePermission(name string) error {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.DeletePermission(self.namespace, name)
}
