package dtos

type K8sServiceDto struct {
	Id                                   string                `json:"id" validate:"required"`
	DisplayName                          string                `json:"displayName" validate:"required"`
	FullHostname                         string                `json:"fullHostname"`
	CNames                               []string              `json:"cNames"`
	GitRepository                        string                `json:"gitRepository"`
	GitBranch                            string                `json:"gitBranch"`
	ContainerImage                       string                `json:"containerImage"`
	ContainerImageRepoSecretDecryptValue string                `json:"containerImageRepoSecretDecryptValue" `
	ContainerImageCommand                string                `json:"containerImageCommand"`
	ContainerImageCommandArgs            string                `json:"containerImageCommandArgs"`
	DockerfileName                       string                `json:"dockerfileName" validate:"required"`
	DockerContext                        string                `json:"dockerContext" validate:"required"`
	App                                  *K8sAppDto            `json:"app"`
	Name                                 string                `json:"name" validate:"required"`
	K8sSettings                          K8sServiceSettingsDto `json:"k8sSettings" validate:"required"`
	EnvVars                              []K8sEnvVarDto        `json:"envVars" validate:"required"`
	Ports                                []K8sPortsDto         `json:"ports" validate:"required"`
	SwitchedOn                           bool                  `json:"switchedOn"`
	ServiceType                          K8sServiceTypeEnum    `json:"serviceType,omitempty"`
	SettingsYaml                         string                `json:"settingsYaml,omitempty"`
}

func (dto *K8sServiceDto) ApplyDefaults() {
	if dto.ServiceType == "" {
		dto.ServiceType = K8SDeployment
	}
}

func K8sServiceDtoExampleData() K8sServiceDto {
	return K8sServiceDto{
		Id:                                   "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:                          "displayName",
		CNames:                               []string{},
		GitRepository:                        "gitRepository",
		GitBranch:                            "main",
		ContainerImage:                       "nginx:latest",
		ContainerImageRepoSecretDecryptValue: "containerImageRepoSecretDecryptValue",
		ContainerImageCommand:                "[\"/bin/sh\"]",
		ContainerImageCommandArgs:            "[\"-c\", \"while true; do date; sleep 1; done\"]",
		DockerfileName:                       "Dockerfile",
		DockerContext:                        ".",
		App:                                  K8sAppDtoDockerExampleData(),
		Name:                                 "name",
		K8sSettings:                          K8sServiceSettingsDtoExampleData(),
		EnvVars:                              []K8sEnvVarDto{K8sEnvVarDtoExampleData(), K8sEnvVarVolumeMountDtoExampleData()},
		Ports:                                []K8sPortsDto{K8sPortsDtoExampleData(), K8sPortsDtoExternalExampleData()},
		SwitchedOn:                           true,
		SettingsYaml:                         "",
	}
}

func K8sServiceContainerImageDtoExampleData() K8sServiceDto {
	return K8sServiceDto{
		Id:                                   "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:                          "displayName",
		FullHostname:                         "fullhostname.iltis.io",
		CNames:                               []string{},
		GitRepository:                        "gitRepository",
		GitBranch:                            "main",
		ContainerImage:                       "nginx:latest",
		ContainerImageRepoSecretDecryptValue: "",
		ContainerImageCommand:                "",
		ContainerImageCommandArgs:            "",
		DockerfileName:                       "Dockerfile",
		DockerContext:                        ".",
		App:                                  K8sAppDtoDockerExampleData(),
		Name:                                 "name",
		K8sSettings:                          K8sServiceSettingsDtoExampleData(),
		EnvVars:                              []K8sEnvVarDto{K8sEnvVarDtoExampleData()},
		Ports:                                []K8sPortsDto{K8sPortsDtoExampleData()},
		SwitchedOn:                           true,
		SettingsYaml:                         "",
	}
}

func K8sServiceCronJobExampleData() K8sServiceDto {
	return K8sServiceDto{
		Id:                                   "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:                          "displayName",
		FullHostname:                         "fullhostname.iltis.io",
		CNames:                               []string{},
		GitRepository:                        "",
		GitBranch:                            "",
		ContainerImage:                       "busybox:1.28",
		ContainerImageRepoSecretDecryptValue: "",
		ContainerImageCommand:                "[\"/bin/sh\"]",
		ContainerImageCommandArgs:            "[\"-c\", \"date; echo Hello, World\"]",
		DockerfileName:                       "",
		DockerContext:                        "",
		App:                                  K8sAppDtoDockerExampleData(),
		Name:                                 "name",
		K8sSettings:                          K8sServiceSettingsDtoExampleData(),
		EnvVars:                              []K8sEnvVarDto{K8sEnvVarDtoExampleData()},
		Ports:                                []K8sPortsDto{K8sPortsDtoExampleData()},
		SwitchedOn:                           true,
		ServiceType:                          "K8S_CRONJOB",
		SettingsYaml:                         "",
	}
}
