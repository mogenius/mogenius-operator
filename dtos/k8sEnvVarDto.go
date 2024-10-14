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

func K8sEnvVarDtoExampleData() K8sEnvVarDto {
	return K8sEnvVarDto{
		Name:  "name",
		Value: "value",
		Type:  EnvVarPlainText,
		Data: K8sEnvVarDataDto{
			Type:  EnvVarPlainText,
			Value: "value",
		},
	}
}

func K8sEnvVarVolumeMountDtoExampleData() K8sEnvVarDto {
	return K8sEnvVarDto{
		Name:  "yyy name",
		Value: "testVolume:/data/html:/html",
		Type:  EnvVarVolumeMount,
		Data: K8sEnvVarDataDto{
			Type:              EnvVarVolumeMount,
			Value:             "testVolume:/data/html:/html",
			VolumeName:        "testVolume",
			VolumeSource:      "/data/html",
			VolumeDestination: "/html",
		},
	}
}

func K8sEnvVarKeyVaultDtoExampleData() K8sEnvVarDto {
	return K8sEnvVarDto{
		Name:  "postgresPWD",
		Value: "password123",
		Type:  EnvVarKeyVault,
		Data: K8sEnvVarDataDto{
			Type:      EnvVarKeyVault,
			Value:     "password123",
			VaultType: EnvVarVaultTypeMogeniusVault,
		},
	}
}

func K8sEnvVarExternalSecretDtoExampleData() K8sEnvVarDto {
	return K8sEnvVarDto{
		Name:  "postgresPWD",
		Value: "namePrefix/propertyname",
		Type:  EnvVarKeyVault,
		Data: K8sEnvVarDataDto{
			Type:       EnvVarKeyVault,
			Value:      "namePrefix/propertyname",
			VaultType:  EnvVarVaultTypeHashicorpExternalVault,
			VaultStore: "namePrefix",
			VaultKey:   "propertyname",
		},
	}
}

func SplitEsoEnvVarValues(envVar K8sEnvVarDto) (string, string) {
	result := strings.Split(envVar.Value, "/")
	return result[0], result[1]
}
