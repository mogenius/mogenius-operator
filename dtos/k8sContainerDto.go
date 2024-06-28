package dtos

import (
	"encoding/base64"
	"fmt"
	"strings"

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
	CNames                               []K8sCnameDto         `json:"cNames"`
	EnvVars                              []K8sEnvVarDto        `json:"envVars"`
	Ports                                []K8sPortsDto         `json:"ports"`
	SettingsYaml                         *string               `json:"settingsYaml,omitempty"`
}

func (k *K8sContainerDto) GetInjectDockerEnvVars(buildId uint64, gitTag string) string {
	gitTag = strings.ReplaceAll(gitTag, "\n", "")
	result := ""
	for _, v := range k.EnvVars {
		if v.Type == EnvVarPlainText || v.Type == EnvVarKeyVault {
			result += fmt.Sprintf("--build-arg %s=\"$(echo \"%s\" | base64 --decode)\" ", v.Name, base64.StdEncoding.EncodeToString([]byte(v.Value)))
		}
	}
	result += fmt.Sprintf("--build-arg MO_BUILD_ID=\"$(echo \"%s\" | base64 --decode)\" ", base64.StdEncoding.EncodeToString([]byte(string(buildId))))
	result += fmt.Sprintf("--build-arg MO_GIT_TAG=\"$(echo \"%s\" | base64 --decode)\" ", base64.StdEncoding.EncodeToString([]byte(gitTag)))
	result += fmt.Sprintf("--build-arg MO_GIT_COMMIT_HASH=\"$(echo \"%s\" | base64 --decode)\" ", base64.StdEncoding.EncodeToString([]byte(*k.GitCommitHash)))
	result += fmt.Sprintf("--build-arg MO_GIT_COMMIT_AUTHOR=\"$(echo \"%s\" | base64 --decode)\" ", base64.StdEncoding.EncodeToString([]byte(*k.GitCommitAuthor)))
	result += fmt.Sprintf("--build-arg MO_GIT_COMMIT_MESSAGE=\"$(echo \"%s\" | base64 --decode)\" ", base64.StdEncoding.EncodeToString([]byte(*k.GitCommitMessage)))
	result += fmt.Sprintf("--build-arg MO_GIT_BRANCH=\"$(echo \"%s\" | base64 --decode)\" ", base64.StdEncoding.EncodeToString([]byte(*k.GitBranch)))
	return result
}

func (k *K8sContainerDto) AvailableDockerBuildArgs(buildId uint64, gitTag string) string {
	gitTag = strings.ReplaceAll(gitTag, "\n", "")
	result := ""
	result += fmt.Sprintf("MO_BUILD_ID=\"$(echo \"%s\" | base64 --decode)\"\n", base64.StdEncoding.EncodeToString([]byte(string(buildId))))
	result += fmt.Sprintf("MO_GIT_TAG=\"$(echo \"%s\" | base64 --decode)\"\n", base64.StdEncoding.EncodeToString([]byte(gitTag)))
	result += fmt.Sprintf("MO_GIT_COMMIT_HASH=\"$(echo \"%s\" | base64 --decode)\"\n", base64.StdEncoding.EncodeToString([]byte(*k.GitCommitHash)))
	result += fmt.Sprintf("MO_GIT_COMMIT_AUTHOR=\"$(echo \"%s\" | base64 --decode)\"\n", base64.StdEncoding.EncodeToString([]byte(*k.GitCommitAuthor)))
	result += fmt.Sprintf("MO_GIT_COMMIT_MESSAGE=\"$(echo \"%s\" | base64 --decode)\"\n", base64.StdEncoding.EncodeToString([]byte(*k.GitCommitMessage)))
	result += fmt.Sprintf("MO_GIT_BRANCH=\"$(echo \"%s\" | base64 --decode)\"\n", base64.StdEncoding.EncodeToString([]byte(*k.GitBranch)))
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
		CNames:                               []K8sCnameDto{K8sCnameDtoExampleData()},
		EnvVars:                              []K8sEnvVarDto{K8sEnvVarDtoExampleData(), K8sEnvVarVolumeMountDtoExampleData()},
		Ports:                                []K8sPortsDto{K8sPortsDtoExampleData(), K8sPortsDtoExternalExampleData()},
		SettingsYaml:                         utils.Pointer("settingsYaml"),
	}
}
