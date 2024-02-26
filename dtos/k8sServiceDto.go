package dtos

type K8sServiceDto struct {
	Id                 string                   `json:"id" validate:"required"`
	DisplayName        string                   `json:"displayName" validate:"required"`
	ControllerName     string                   `json:"controllerName"`
	Controller         K8sServiceControllerEnum `json:"controller"`
	ReplicaCount       int                      `json:"replicaCount"`
	DeploymentStrategy DeploymentStrategyEnum   `json:"deploymentStrategy"`
	CronJobSettings    *K8sCronJobSettingsDto   `json:"cronJobSettings"`
	Containers         []K8sContainerDto        `json:"containers"`
}

func (k *K8sServiceDto) HasContainerWithGitRepo() bool {
	for _, v := range k.Containers {
		if v.Type == CONTAINER_GIT_REPOSITORY {
			return true
		}
	}
	return false
}

func (k *K8sServiceDto) HasSeedRepo() bool {
	for _, v := range k.Containers {
		if v.AppGitRepositoryCloneUrl != nil {
			return true
		}
	}
	return false
}

func (k *K8sServiceDto) HasPorts() bool {
	for _, v := range k.Containers {
		if len(v.Ports) > 0 {
			return true
		}
	}
	return false
}

func (k *K8sServiceDto) GetImageRepoSecretDecryptValue() *string {
	for _, v := range k.Containers {
		if v.ContainerImageRepoSecretDecryptValue != nil {
			return v.ContainerImageRepoSecretDecryptValue
		}
	}
	return nil
}

func K8sServiceDtoExampleData() K8sServiceDto {
	return K8sServiceDto{
		Id:                 "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:        "displayName",
		ControllerName:     "controllerName",
		Controller:         DEPLOYMENT,
		ReplicaCount:       1,
		DeploymentStrategy: StrategyRecreate,
		CronJobSettings:    nil,
		Containers:         []K8sContainerDto{K8sContainerDtoExampleData()},
	}
}

func K8sServiceContainerImageDtoExampleData() K8sServiceDto {
	return K8sServiceDto{
		Id:                 "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:        "displayName",
		ControllerName:     "controllerName",
		Controller:         DEPLOYMENT,
		ReplicaCount:       1,
		DeploymentStrategy: StrategyRecreate,
		CronJobSettings:    nil,
		Containers:         []K8sContainerDto{K8sContainerDtoExampleData()},
	}
}

func K8sServiceCronJobExampleData() K8sServiceDto {
	return K8sServiceDto{
		Id:                 "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:        "displayName",
		ControllerName:     "controllerName",
		Controller:         CRON_JOB,
		ReplicaCount:       1,
		DeploymentStrategy: StrategyRecreate,
		CronJobSettings:    &K8sCronJobSettingsDto{},
		Containers:         []K8sContainerDto{K8sContainerDtoExampleData()},
	}
}
