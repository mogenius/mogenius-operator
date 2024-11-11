package dtos

import (
	"mogenius-k8s-manager/assert"
	"mogenius-k8s-manager/logging"

	punqUtils "github.com/mogenius/punq/utils"
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
		GitAccessToken:        punqUtils.Pointer("gitAccessToken"),
		GitUserId:             punqUtils.Pointer("gitUserId"),
		GitConnectionType:     punqUtils.Pointer(GitConGitHub),
		ClusterId:             "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ClusterDisplayName:    "clusterName",
		ClusterMfaId:          "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		ContainerRegistryPath: punqUtils.Pointer("docker.io/mogee1"),
		ContainerRegistryUrl:  punqUtils.Pointer("https://index.docker.io/v1"),
		ContainerRegistryUser: punqUtils.Pointer("YYY_FAKE_USER"),
		ContainerRegistryPat:  punqUtils.Pointer("YYY_FAKE_PAT-pqKg4"),
	}
}

func (p *K8sProjectDto) AddSecretsToRedaction() {
	assert.Assert(p.GitAccessToken != nil)
	logging.AddSecret(*p.GitAccessToken)

	assert.Assert(p.GitUserId != nil)
	logging.AddSecret(*p.GitUserId)

	logging.AddSecret(p.ClusterMfaId)

	assert.Assert(p.ContainerRegistryUser != nil)
	logging.AddSecret(*p.ContainerRegistryUser)

	assert.Assert(p.ContainerRegistryPat != nil)
	logging.AddSecret(*p.ContainerRegistryPat)
}
