package utils

import "mogenius-k8s-manager/src/crds/v1alpha1"

type WebsocketRequestCreateWorkspace struct {
	Name        string                                 `json:"name" validate:"required"`
	DisplayName string                                 `json:"displayName" validate:"required"`
	Resources   []v1alpha1.WorkspaceResourceIdentifier `json:"resources" validate:"required"`
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
	Name       string `json:"name" validate:"required"`
	MogeniusId string `json:"mogeniusId" validate:"required"`
}

type WebsocketRequestGetUser struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestUpdateUser struct {
	Name       string `json:"name" validate:"required"`
	MogeniusId string `json:"mogeniusId" validate:"required"`
}

type WebsocketRequestDeleteUser struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestCreateTeam struct {
	Name        string   `json:"name" validate:"required"`
	DisplayName string   `json:"displayName" validate:"required"`
	Users       []string `json:"users" validate:"required"`
}

type WebsocketRequestGetTeam struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestUpdateTeam struct {
	Name        string   `json:"name" validate:"required"`
	DisplayName string   `json:"displayName" validate:"required"`
	Users       []string `json:"users" validate:"required"`
}

type WebsocketRequestDeleteTeam struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestCreateGrant struct {
	Name       string `json:"name" validate:"required"`
	Grantee    string `json:"grantee" validate:"required"`
	TargetType string `json:"targetType" validate:"required"`
	TargetName string `json:"targetName" validate:"required"`
	Role       string `json:"role" validate:"required"`
}

type WebsocketRequestGetGrant struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestUpdateGrant struct {
	Name       string `json:"name" validate:"required"`
	Grantee    string `json:"grantee" validate:"required"`
	TargetType string `json:"targetType" validate:"required"`
	TargetName string `json:"targetName" validate:"required"`
	Role       string `json:"role" validate:"required"`
}

type WebsocketRequestDeleteGrant struct {
	Name string `json:"name" validate:"required"`
}

type WorkspaceStatsRequest struct {
	WorkspaceName     string `json:"workspaceName"`
	TimeOffSetMinutes int    `json:"timeOffSetMinutes"`
}

type WorkspaceWorkloadRequest struct {
	WorkspaceName string               `json:"workspaceName"`
	Whitelist     []*SyncResourceEntry `json:"whitelist"`
	Blacklist     []*SyncResourceEntry `json:"blacklist"`
}
