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
