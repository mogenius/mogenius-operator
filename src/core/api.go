package core

import (
	"log/slog"
	"mogenius-k8s-manager/src/crds/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	GetAllUsers() ([]v1alpha1.User, error)
	GetUser(name string) (*v1alpha1.User, error)
	CreateUser(name string, spec v1alpha1.UserSpec) (string, error)
	UpdateUser(name string, spec v1alpha1.UserSpec) (string, error)
	DeleteUser(name string) (string, error)

	GetAllTeams() ([]v1alpha1.Team, error)
	GetTeam(name string) (*v1alpha1.Team, error)
	CreateTeam(name string, spec v1alpha1.TeamSpec) (string, error)
	UpdateTeam(name string, spec v1alpha1.TeamSpec) (string, error)
	DeleteTeam(name string) (string, error)

	GetAllGrants() ([]v1alpha1.Grant, error)
	GetGrant(name string) (*v1alpha1.Grant, error)
	CreateGrant(name string, spec v1alpha1.GrantSpec) (string, error)
	UpdateGrant(name string, spec v1alpha1.GrantSpec) (string, error)
	DeleteGrant(name string) (string, error)
}

type api struct {
	workspaceManager WorkspaceManager
	logger           *slog.Logger
}

func NewApi(logger *slog.Logger, workspaceManager WorkspaceManager) Api {
	apiModule := &api{}

	apiModule.logger = logger
	apiModule.workspaceManager = workspaceManager

	return apiModule
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
	result := []GetWorkspaceResult{}

	resources, err := self.workspaceManager.GetAllWorkspaces()
	if err != nil {
		return result, err
	}

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
	resource, err := self.workspaceManager.GetWorkspace(name)
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
	err := self.workspaceManager.DeleteWorkspace(name)
	if err != nil {
		return "", err
	}

	return "Resource deleted successfully", nil
}

func (self *api) GetAllUsers() ([]v1alpha1.User, error) {
	resources, err := self.workspaceManager.GetAllUsers()
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

func (self *api) GetAllTeams() ([]v1alpha1.Team, error) {
	resources, err := self.workspaceManager.GetAllTeams()
	if err != nil {
		return []v1alpha1.Team{}, err
	}

	return resources, nil
}

func (self *api) GetTeam(name string) (*v1alpha1.Team, error) {
	resource, err := self.workspaceManager.GetTeam(name)
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (self *api) CreateTeam(name string, spec v1alpha1.TeamSpec) (string, error) {
	_, err := self.workspaceManager.CreateTeam(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource created successfully", nil
}

func (self *api) UpdateTeam(name string, spec v1alpha1.TeamSpec) (string, error) {
	_, err := self.workspaceManager.UpdateTeam(name, spec)
	if err != nil {
		return "", err
	}

	return "Resource updated successfully", nil
}

func (self *api) DeleteTeam(name string) (string, error) {
	err := self.workspaceManager.DeleteTeam(name)
	if err != nil {
		return "", err
	}

	return "Resource deleted successfully", nil
}

func (self *api) GetAllGrants() ([]v1alpha1.Grant, error) {
	resources, err := self.workspaceManager.GetAllGrants()
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
