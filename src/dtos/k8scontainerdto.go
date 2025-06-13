package dtos

type K8sContainerDto struct {
	Id                                   string                `json:"id" validate:"required"`
	DisplayName                          string                `json:"displayName" validate:"required"`
	Name                                 string                `json:"name" validate:"required"`
	Type                                 K8sContainerTypeEnum  `json:"type" validate:"required"`
	ContainerImage                       *string               `json:"containerImage"`
	ContainerImageCommand                *string               `json:"containerImageCommand"`
	ContainerImageCommandArgs            *string               `json:"containerImageCommandArgs"`
	ContainerImageRepoSecretId           *string               `json:"containerImageRepoSecretId"`
	ContainerImageRepoSecretDecryptValue *string               `json:"containerImageRepoSecretDecryptValue" `
	GitRepository                        *string               `json:"gitRepository"`
	GitBranch                            *string               `json:"gitBranch"`
	GitCommitAuthor                      *string               `json:"gitCommitAuthor"`
	GitCommitHash                        *string               `json:"gitCommitHash"`
	GitCommitMessage                     *string               `json:"gitCommitMessage"`
	DockerfileName                       *string               `json:"dockerfileName"`
	DockerContext                        *string               `json:"dockerContext"`
	AppGitRepositoryCloneUrl             *string               `json:"appGitRepositoryCloneUrl"`
	AppSetupCommands                     *string               `json:"appSetupCommands"`
	KubernetesLimits                     K8sServiceSettingsDto `json:"KubernetesLimits"`
	Probes                               *K8sProbes            `json:"probes,omitempty"`
	EnvVars                              []K8sEnvVarDto        `json:"envVars"`
	SettingsYaml                         *string               `json:"settingsYaml,omitempty"`
}
