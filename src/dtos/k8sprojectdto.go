package dtos

import "mogenius-k8s-manager/src/secrets"

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

func (p *K8sProjectDto) AddSecretsToRedaction() {
	if p.GitAccessToken != nil {
		secrets.AddSecret(*p.GitAccessToken)
	}

	if p.GitUserId != nil {
		secrets.AddSecret(*p.GitUserId)
	}

	secrets.AddSecret(p.ClusterMfaId)

	if p.ContainerRegistryUser != nil {
		secrets.AddSecret(*p.ContainerRegistryUser)
	}

	if p.ContainerRegistryPat != nil {
		secrets.AddSecret(*p.ContainerRegistryPat)
	}
}
