package dtos

import (
	"fmt"

	"github.com/mogenius/punq/utils"
)

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
	CNames                               []string              `json:"cNames"`
	EnvVars                              []K8sEnvVarDto        `json:"envVars"`
	Ports                                []K8sPortsDto         `json:"ports"`
	SettingsYaml                         *string               `json:"settingsYaml,omitempty"`
}

func (k *K8sContainerDto) GetInjectDockerEnvVars() string {
	result := ""
	for _, v := range k.EnvVars {
		if v.Type == EnvVarPlainText || v.Type == EnvVarKeyVault {
			result += fmt.Sprintf("--build-arg %s=%s ", v.Name, v.Value)
		}
	}
	return result
}

func K8sContainerDtoExampleData() K8sContainerDto {
	return K8sContainerDto{
		Id:                                   "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		DisplayName:                          "displayName",
		Name:                                 "name",
		Type:                                 CONTAINER_CONTAINER_IMAGE,
		ContainerImage:                       utils.Pointer("nginx:latest"),
		ContainerImageCommand:                utils.Pointer("[\"/bin/sh\"]"),
		ContainerImageCommandArgs:            utils.Pointer("[\"-c\", \"while true; do date; sleep 1; done\"]"),
		ContainerImageRepoSecretId:           utils.Pointer("B0919ACB-92"),
		ContainerImageRepoSecretDecryptValue: utils.Pointer("containerImageRepoSecretDecryptValue"),
		GitRepository:                        utils.Pointer("gitRepository"),
		GitBranch:                            utils.Pointer("main"),
		DockerfileName:                       utils.Pointer("Dockerfile"),
		DockerContext:                        utils.Pointer("."),
		AppGitRepositoryCloneUrl:             utils.Pointer("YYY_git_clone_url"),
		KubernetesLimits:                     K8sServiceSettingsDtoExampleData(),
		CNames:                               []string{"cname1.com", "cname2.com"},
		EnvVars:                              []K8sEnvVarDto{K8sEnvVarDtoExampleData(), K8sEnvVarVolumeMountDtoExampleData()},
		Ports:                                []K8sPortsDto{K8sPortsDtoExampleData(), K8sPortsDtoExternalExampleData()},
		SettingsYaml:                         utils.Pointer("settingsYaml"),
	}
}
