package dtos

type K8sProjectDto struct {
	Id                    string                `json:"id" validate:"required"`
	DisplayName           string                `json:"displayName" validate:"required"`
	GitAccessToken        string                `json:"gitAccessToken"`
	GitUserId             string                `json:"gitUserId"`
	GitConnectionType     GitConnectionTypeEnum `json:"gitConnectionType"`
	ClusterName           string                `json:"clusterName" validate:"required"`
	ContainerRegistryUrl  string                `json:"containerRegistryUrl"`
	ContainerRegistryUser string                `json:"containerRegistryUser"`
	ContainerRegistryPat  string                `json:"containerRegistryPat"`
}

func K8sProjectDtoExampleData() K8sProjectDto {
	return K8sProjectDto{
		Id:                    "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:           "displayName",
		GitAccessToken:        "gitAccessToken",
		GitUserId:             "gitUserId",
		GitConnectionType:     GitConGitHub,
		ClusterName:           "clusterName",
		ContainerRegistryUrl:  "https://index.docker.io/v1",
		ContainerRegistryUser: "XXX_FAKE_USER",
		ContainerRegistryPat:  "XXX_FAKE_PAT-pqKg4",
	}
}
