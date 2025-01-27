package utils

import "mogenius-k8s-manager/src/crds/v1alpha1"

type WebsocketRequestCreateWorkspace struct {
	Name      string `json:"name" validate:"required"`
	Resources []struct {
		Id        string                 `json:"id" validate:"required"`
		Type      v1alpha1.WorkspaceType `json:"type" validate:"required"`
		Namespace string                 `json:"namespace"`
	} `json:"resources" validate:"required"`
}

type WebsocketRequestGetWorkspace struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestUpdateWorkspace struct {
	Name        string `json:"name" validate:"required"`
	DisplayName string `json:"displayName" validate:"required"`
	Resources   []struct {
		Id        string                 `json:"id" validate:"required"`
		Type      v1alpha1.WorkspaceType `json:"type" validate:"required"`
		Namespace string                 `json:"namespace"`
	} `json:"resources" validate:"required"`
}

type WebsocketRequestDeleteWorkspace struct {
	Name string `json:"name" validate:"required"`
}
