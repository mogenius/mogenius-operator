package dtos

type K8sServiceDto struct {
	Id                                   string                     `json:"id" validate:"required"`
	DisplayName                          string                     `json:"displayName" validate:"required"`
	ShortId                              string                     `json:"shortId" validate:"required"`
	FullHostname                         string                     `json:"fullHostname" validate:"required"`
	K8sName                              string                     `json:"k8sName" validate:"required"`
	CNames                               []NamespaceServiceCnameDto `json:"cNames" validate:"required"`
	GitRepository                        string                     `json:"gitRepository" validate:"required"`
	GitBranch                            string                     `json:"gitBranch" validate:"required"`
	ContainerImage                       string                     `json:"containerImage" validate:"required"`
	ContainerImageRepoSecretDecryptValue string                     `json:"containerImageRepoSecretDecryptValue" validate:"required"`
	ContainerImageCommand                string                     `json:"containerImageCommand" validate:"required"`
	ContainerImageCommandArgs            string                     `json:"containerImageCommandArgs" validate:"required"`
	DockerfileName                       string                     `json:"dockerfileName" validate:"required"`
	DockerContext                        string                     `json:"dockerContext" validate:"required"`
	App                                  K8sAppDto                  `json:"app" validate:"required"`
	Name                                 string                     `json:"name" validate:"required"`
	K8sSettings                          K8sServiceSettingsDto      `json:"k8sSettings" validate:"required"`
	EnvVars                              []K8sEnvVarDto             `json:"envVars" validate:"required"`
	Ports                                []K8sPortsDto              `json:"ports" validate:"required"`
	SwitchedOn                           bool                       `json:"switchedOn" validate:"required"`
}
