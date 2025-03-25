package dtos

import "strings"

type K8sEnvVarDto struct {
	Id    string           `json:"id" validate:"required"`
	Name  string           `json:"name" validate:"required"`
	Value string           `json:"value" validate:"required"`
	Type  K8sEnvVarDtoEnum `json:"type" validate:"required"`
	Data  K8sEnvVarDataDto `json:"data" validate:"required"`
}

type K8sEnvVarDataDto struct {
	Type  K8sEnvVarDtoEnum `json:"type" validate:"required"`
	Value string           `json:"value" validate:"required"`

	// KeyVault type
	VaultType  EnvVarVaultTypeEnum `json:"vaultType,omitempty"`
	VaultStore string              `json:"vaultStore,omitempty"`
	VaultKey   string              `json:"vaultKey,omitempty"`

	// VolumeMount type
	VolumeName        string `json:"volumeName,omitempty"`
	VolumeSource      string `json:"volumeSource,omitempty"`
	VolumeDestination string `json:"volumeDestination,omitempty"`
}

func SplitEsoEnvVarValues(envVar K8sEnvVarDto) (string, string) {
	result := strings.Split(envVar.Value, "/")
	return result[0], result[1]
}
