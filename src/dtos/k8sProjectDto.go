package dtos

import (
	"mogenius-k8s-manager/src/logging"

	"github.com/mogenius/punq/utils"
)

type K8sProjectDto struct {
	Id                    string                 `json:"id" validate:"required"`
	DisplayName           string                 `json:"displayName" validate:"required"`
	Name                  string                 `json:"name" validate:"required"`
	GitAccessToken        *string                `json:"gitAccessToken"`
	GitUserId             *string                `json:"gitUserId"`
	GitConnectionType     *GitConnectionTypeEnum `json:"gitConnectionType"`
	ClusterId             string                 `json:"clusterId" validate:"required"`
	ClusterDisplayName    string                 `json:"clusterDisplayName" validate:"required"`
	ClusterMfaId          string                 `json:"clusterMfaId" validate:"required"`
	ContainerRegistryPath *string                `json:"containerRegistryPath"`
	ContainerRegistryUrl  *string                `json:"containerRegistryUrl"`
	ContainerRegistryUser *string                `json:"containerRegistryUser"`
	ContainerRegistryPat  *string                `json:"containerRegistryPat"`
}

func K8sProjectDtoExampleData() K8sProjectDto {
	return K8sProjectDto{
		Id:                    "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:           "displayName",
		Name:                  "name",
		GitAccessToken:        utils.Pointer("gitAccessToken"),
		GitUserId:             utils.Pointer("gitUserId"),
		GitConnectionType:     utils.Pointer(GitConGitHub),
		ClusterId:             "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ClusterDisplayName:    "clusterName",
		ClusterMfaId:          "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ContainerRegistryPath: utils.Pointer("docker.io/mogee1"),
		ContainerRegistryUrl:  utils.Pointer("https://index.docker.io/v1"),
		ContainerRegistryUser: utils.Pointer("YYY_FAKE_USER"),
		ContainerRegistryPat:  utils.Pointer("YYY_FAKE_PAT-pqKg4"),
	}
}

func (p *K8sProjectDto) AddSecretsToRedaction() {
	if p.GitAccessToken != nil {
		logging.AddSecret(*p.GitAccessToken)
	}

	if p.GitUserId != nil {
		logging.AddSecret(*p.GitUserId)
	}

	logging.AddSecret(p.ClusterMfaId)

	if p.ContainerRegistryUser != nil {
		logging.AddSecret(*p.ContainerRegistryUser)
	}

	if p.ContainerRegistryPat != nil {
		logging.AddSecret(*p.ContainerRegistryPat)
	}
}
