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
	GetAllWorkspaces() (*[]GetAllWorkspacesResultWorkspace, error)
	GetWorkspace(name string) (*GetWorkspaceResult, error)
	CreateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error)
	UpdateWorkspace(name string, spec v1alpha1.WorkspaceSpec) (string, error)
	DeleteWorkspace(name string) (string, error)
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

//type GetAllWorkspacesResult struct {
//	Workspaces []GetAllWorkspacesResultWorkspace `json:"workspaces,inline" validate:"required"`
//}

type GetAllWorkspacesResultWorkspace struct {
	Name              string                           `json:"name" validate:"required"`
	CreationTimestamp v1.Time                          `json:"creationTimestamp,omitempty"`
	Resources         []GetAllWorkspacesResultResource `json:"resources" validate:"required"`
}

type GetAllWorkspacesResultResource struct {
	Id   string `json:"id" validate:"required"`
	Type string `json:"type" validate:"required"`
}

func (self *api) GetAllWorkspaces() (*[]GetAllWorkspacesResultWorkspace, error) {
	workspaces, err := self.workspaceManager.GetAllWorkspaces()
	if err != nil {
		return nil, err
	}

	var result []GetAllWorkspacesResultWorkspace
	for _, workspace := range workspaces {
		workspaceResult := GetAllWorkspacesResultWorkspace{
			Name:              workspace.GetName(),
			CreationTimestamp: workspace.ObjectMeta.CreationTimestamp,
			Resources:         []GetAllWorkspacesResultResource{},
		}
		for _, resource := range workspace.Spec.Resources {
			workspaceResult.Resources = append(workspaceResult.Resources, GetAllWorkspacesResultResource{
				Id:   resource.Id,
				Type: string(resource.Type),
			})
		}
		result = append(result, workspaceResult)
	}

	return &result, nil
}

type GetWorkspaceResult struct {
	Name              string                       `json:"name" validate:"required"`
	CreationTimestamp v1.Time                      `json:"creationTimestamp,omitempty"`
	Resources         []GetWorkspaceResultResource `json:"resources" validate:"required"`
}

type GetWorkspaceResultResource struct {
	Id   string `json:"id" validate:"required"`
	Type string `json:"type" validate:"required"`
}

func (self *api) GetWorkspace(name string) (*GetWorkspaceResult, error) {
	workspace, err := self.workspaceManager.GetWorkspace(name)
	if err != nil {
		return nil, err
	}

	result := &GetWorkspaceResult{
		Name:              workspace.Name,
		CreationTimestamp: workspace.ObjectMeta.CreationTimestamp,
	}
	for _, resource := range workspace.Spec.Resources {
		result.Resources = append(result.Resources, GetWorkspaceResultResource{
			Id:   resource.Id,
			Type: string(resource.Type),
		})
	}

	return result, nil
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
