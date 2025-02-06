package utils

import "mogenius-k8s-manager/src/crds/v1alpha1"

type WebsocketRequestCreateWorkspace struct {
	Name      string                                 `json:"name" validate:"required"`
	Resources []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
}

type WebsocketRequestGetWorkspace struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestUpdateWorkspace struct {
	Name        string                                 `json:"name" validate:"required"`
	DisplayName string                                 `json:"displayName" validate:"required"`
	Resources   []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
}

type WebsocketRequestDeleteWorkspace struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestCreateUser struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestGetUser struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestUpdateUser struct {
	Name        string                                 `json:"name" validate:"required"`
	DisplayName string                                 `json:"displayName" validate:"required"`
	Resources   []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
}

type WebsocketRequestDeleteUser struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestCreateGroup struct {
	Name  string   `json:"name" validate:"required"`
	Users []string `json:"users" validate:"required"`
}

type WebsocketRequestGetGroup struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestUpdateGroup struct {
	Name  string   `json:"name" validate:"required"`
	Users []string `json:"users" validate:"required"`
}

type WebsocketRequestDeleteGroup struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestCreatePermission struct {
	Name      string `json:"name" validate:"required"`
	Group     string `json:"group" validate:"required"`
	Workspace string `json:"workspace" validate:"required"`
	Read      bool   `json:"read" validate:"required"`
	Write     bool   `json:"write" validate:"required"`
	Delete    bool   `json:"delete" validate:"required"`
}

type WebsocketRequestGetPermission struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestUpdatePermission struct {
	Name      string `json:"name" validate:"required"`
	Group     string `json:"group" validate:"required"`
	Workspace string `json:"workspace" validate:"required"`
	Read      bool   `json:"read" validate:"required"`
	Write     bool   `json:"write" validate:"required"`
	Delete    bool   `json:"delete" validate:"required"`
}

type WebsocketRequestDeletePermission struct {
	Name string `json:"name" validate:"required"`
}
