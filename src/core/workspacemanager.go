package core

import (
	"context"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/crds"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/k8sclient"
	"sync"
)

type WorkspaceManager interface {
	GetAllWorkspaces() ([]v1alpha1.Workspace, error)
	CreateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (*v1alpha1.Workspace, error)
	GetWorkspace(name string) (*v1alpha1.Workspace, error)
	UpdateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (*v1alpha1.Workspace, error)
	DeleteWorkspace(name string) error

	GetAllUsers(email *string) ([]v1alpha1.User, error)
	CreateUser(name string, spec v1alpha1.UserSpec) (*v1alpha1.User, error)
	GetUser(name string) (*v1alpha1.User, error)
	UpdateUser(name string, spec v1alpha1.UserSpec) (*v1alpha1.User, error)
	DeleteUser(name string) error

	GetAllGrants(targetType, targetName *string) ([]v1alpha1.Grant, error)
	CreateGrant(name string, spec v1alpha1.GrantSpec) (*v1alpha1.Grant, error)
	GetGrant(name string) (*v1alpha1.Grant, error)
	UpdateGrant(name string, spec v1alpha1.GrantSpec) (*v1alpha1.Grant, error)
	DeleteGrant(name string) error

	GetAllAgents() ([]v1alpha1.Agent, error)
	CreateAgent(name string, spec v1alpha1.AgentSpec) (*v1alpha1.Agent, error)
	GetAgent(name string) (*v1alpha1.Agent, error)
	UpdateAgent(name string, spec v1alpha1.AgentSpec) (*v1alpha1.Agent, error)
	DeleteAgent(name string) error
	RequestAgentRun(name string) error
	RequestAiModelUsageReset(name string) error

	GetAllAiModels() ([]v1alpha1.AiModel, error)
	CreateAiModel(name string, spec v1alpha1.AiModelSpec, apiKey string) (*v1alpha1.AiModel, error)
	GetAiModel(name string) (*v1alpha1.AiModel, error)
	UpdateAiModel(name string, spec v1alpha1.AiModelSpec, apiKey string) (*v1alpha1.AiModel, error)
	DeleteAiModel(name string) error
}

type workspaceManager struct {
	logger            *slog.Logger
	config            cfg.ConfigModule
	clientProvider    k8sclient.K8sClientProvider
	mogeniusClientSet *crds.MogeniusClientSet
	namespace         string
	namespaceLock     sync.RWMutex
}

func NewWorkspaceManager(logger *slog.Logger, configModule cfg.ConfigModule, clientProvider k8sclient.K8sClientProvider) WorkspaceManager {
	wm := &workspaceManager{}
	wm.logger = logger
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

func (self *workspaceManager) GetAllUsers(email *string) ([]v1alpha1.User, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	result, err := self.mogeniusClientSet.MogeniusV1alpha1.ListUsers(self.namespace)
	if err == nil && email != nil {
		filteredResult := make([]v1alpha1.User, 0, len(result))
		for _, grant := range result {
			if grant.Spec.Email == *email {
				filteredResult = append(filteredResult, grant)
			}
		}
		result = filteredResult
	}

	return result, err
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

func (self *workspaceManager) GetAllGrants(targetType, targetName *string) ([]v1alpha1.Grant, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	result, err := self.mogeniusClientSet.MogeniusV1alpha1.ListGrants(self.namespace)
	if err == nil && targetType != nil && targetName != nil {
		filteredResult := make([]v1alpha1.Grant, 0, len(result))
		for _, grant := range result {
			if grant.Spec.TargetType == *targetType && grant.Spec.TargetName == *targetName {
				filteredResult = append(filteredResult, grant)
			}
		}
		result = filteredResult
	}

	return result, err
}

func (self *workspaceManager) GetGrant(name string) (*v1alpha1.Grant, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.GetGrant(self.namespace, name)
}

func (self *workspaceManager) CreateGrant(name string, spec v1alpha1.GrantSpec) (*v1alpha1.Grant, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.CreateGrant(self.namespace, name, spec)
}

func (self *workspaceManager) UpdateGrant(name string, spec v1alpha1.GrantSpec) (*v1alpha1.Grant, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.UpdateGrant(self.namespace, name, spec)
}

func (self *workspaceManager) DeleteGrant(name string) error {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.DeleteGrant(self.namespace, name)
}

func (self *workspaceManager) GetAllAgents() ([]v1alpha1.Agent, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.ListAgents(self.namespace)
}

func (self *workspaceManager) GetAgent(name string) (*v1alpha1.Agent, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.GetAgent(self.namespace, name)
}

func (self *workspaceManager) CreateAgent(name string, spec v1alpha1.AgentSpec) (*v1alpha1.Agent, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.CreateAgent(self.namespace, name, spec)
}

func (self *workspaceManager) UpdateAgent(name string, spec v1alpha1.AgentSpec) (*v1alpha1.Agent, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.UpdateAgent(self.namespace, name, spec)
}

func (self *workspaceManager) DeleteAgent(name string) error {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.DeleteAgent(self.namespace, name)
}

func (self *workspaceManager) RequestAgentRun(name string) error {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	_, err := self.mogeniusClientSet.MogeniusV1alpha1.RequestAgentRun(self.namespace, name)
	return err
}

func (self *workspaceManager) RequestAiModelUsageReset(name string) error {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	_, err := self.mogeniusClientSet.MogeniusV1alpha1.RequestAiModelUsageReset(self.namespace, name)
	return err
}

func (self *workspaceManager) GetAllAiModels() ([]v1alpha1.AiModel, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.ListAiModels(self.namespace)
}

func (self *workspaceManager) GetAiModel(name string) (*v1alpha1.AiModel, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.GetAiModel(self.namespace, name)
}

func (self *workspaceManager) CreateAiModel(name string, spec v1alpha1.AiModelSpec, apiKey string) (*v1alpha1.AiModel, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return createAiModelWithManagedSecret(context.Background(), self.clientProvider.K8sClientSet(), mogeniusAiModelCrdOps{self.mogeniusClientSet}, self.logger, self.namespace, name, spec, apiKey)
}

func (self *workspaceManager) UpdateAiModel(name string, spec v1alpha1.AiModelSpec, apiKey string) (*v1alpha1.AiModel, error) {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return updateAiModelWithManagedSecret(context.Background(), self.clientProvider.K8sClientSet(), mogeniusAiModelCrdOps{self.mogeniusClientSet}, self.namespace, name, spec, apiKey)
}

func (self *workspaceManager) DeleteAiModel(name string) error {
	self.namespaceLock.RLock()
	defer self.namespaceLock.RUnlock()
	return self.mogeniusClientSet.MogeniusV1alpha1.DeleteAiModel(self.namespace, name)
}

// mogeniusAiModelCrdOps adapts the MogeniusV1alpha1 client to the minimal
// aiModelCrdOps surface the managed-secret orchestration works against.
type mogeniusAiModelCrdOps struct {
	clientSet *crds.MogeniusClientSet
}

func (self mogeniusAiModelCrdOps) Create(namespace string, name string, spec v1alpha1.AiModelSpec) (*v1alpha1.AiModel, error) {
	return self.clientSet.MogeniusV1alpha1.CreateAiModel(namespace, name, spec)
}

func (self mogeniusAiModelCrdOps) Update(namespace string, name string, spec v1alpha1.AiModelSpec) (*v1alpha1.AiModel, error) {
	return self.clientSet.MogeniusV1alpha1.UpdateAiModel(namespace, name, spec)
}

func (self mogeniusAiModelCrdOps) Get(namespace string, name string) (*v1alpha1.AiModel, error) {
	return self.clientSet.MogeniusV1alpha1.GetAiModel(namespace, name)
}
