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

func K8sNamespaceDtoExampleData() K8sNamespaceDto {
	return K8sNamespaceDto{
		Id:                "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ShortId:           "shortId",
		DisplayName:       "displayName",
		GitAccessToken:    "gitAccessToken",
		GitUserId:         "gitUserId",
		GitConnectionType: "GIT_HUB",
		ClusterName:       "clusterName",
	}
}
