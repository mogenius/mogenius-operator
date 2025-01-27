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
