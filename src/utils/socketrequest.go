package utils

type WebsocketRequestCreateWorkspace struct {
	Name        string `json:"name" validate:"required"`
	DisplayName string `json:"displayName" validate:"required"`
	Resources   []struct {
		Id   string `json:"id" validate:"required"`
		Type string `json:"type" validate:"required"`
	} `json:"resources" validate:"required"`
}

type WebsocketRequestGetWorkspace struct {
	Name string `json:"name" validate:"required"`
}

type WebsocketRequestUpdateWorkspace struct {
	Name        string `json:"name" validate:"required"`
	DisplayName string `json:"displayName" validate:"required"`
	Resources   []struct {
		Id   string `json:"id" validate:"required"`
		Type string `json:"type" validate:"required"`
	} `json:"resources" validate:"required"`
}

type WebsocketRequestDeleteWorkspace struct {
	Name string `json:"name" validate:"required"`
}
