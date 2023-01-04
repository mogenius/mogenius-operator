package dtos

type K8sNamespaceDto struct {
	Id                string `json:"id" validate:"required"`
	ShortId           string `json:"shortId" validate:"required"`
	DisplayName       string `json:"displayName" validate:"required"`
	GitAccessToken    string `json:"gitAccessToken" validate:"required"`
	GitUserId         string `json:"gitUserId" validate:"required"`
	GitConnectionType string `json:"gitConnectionType" validate:"required"` // "GIT_HUB", "GIT_LAB", "BITBUCKET"
	ClusterName       string `json:"clusterName" validate:"required"`
}
