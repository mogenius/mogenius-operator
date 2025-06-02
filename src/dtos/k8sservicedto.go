package dtos

import "mogenius-k8s-manager/src/secrets"

type K8sServiceDto struct {
	Id                 string                   `json:"id" validate:"required"`
	DisplayName        string                   `json:"displayName" validate:"required"`
	ControllerName     string                   `json:"controllerName"`
	Controller         K8sServiceControllerEnum `json:"controller"`
	ReplicaCount       int                      `json:"replicaCount"`
	DeploymentStrategy DeploymentStrategyEnum   `json:"deploymentStrategy"`
	Ports              []K8sPortsDto            `json:"ports"`
	CronJobSettings    *K8sCronJobSettingsDto   `json:"cronJobSettings"`
	HpaSettings        *K8sHpaSettingsDto       `json:"hpaSettings,omitempty"`
	Containers         []K8sContainerDto        `json:"containers"`
}

func (s *K8sServiceDto) AddSecretsToRedaction() {
	for _, container := range s.Containers {
		if container.ContainerImageRepoSecretDecryptValue != nil {
			secrets.AddSecret(*container.ContainerImageRepoSecretDecryptValue)
		}
		if container.ContainerImageRepoSecretId != nil {
			secrets.AddSecret(*container.ContainerImageRepoSecretId)
		}
		for _, envVar := range container.EnvVars {
			if envVar.Type == EnvVarKeyVault && envVar.Data.VaultType == EnvVarVaultTypeMogeniusVault {
				secrets.AddSecret(envVar.Value)
			}
		}
	}
}

func (k *K8sServiceDto) HasContainerWithGitRepo() bool {
	for _, v := range k.Containers {
		if v.Type == CONTAINER_GIT_REPOSITORY {
			return true
		}
	}
	return false
}

func (k *K8sServiceDto) HpaEnabled() bool {
	return k.HpaSettings != nil
}

func (k *K8sServiceDto) GetImageRepoSecretDecryptValue() *string {
	for _, v := range k.Containers {
		if v.ContainerImageRepoSecretDecryptValue != nil {
			return v.ContainerImageRepoSecretDecryptValue
		}
	}
	return nil
}
