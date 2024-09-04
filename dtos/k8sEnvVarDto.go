package dtos

import "strings"

type K8sEnvVarDto struct {
	Name  string           `json:"name" validate:"required"`
	Value string           `json:"value" validate:"required"`
	Type  K8sEnvVarDtoEnum `json:"type" validate:"required"`
}

func K8sEnvVarDtoExampleData() K8sEnvVarDto {
	return K8sEnvVarDto{
		Name:  "name",
		Value: "value",
		Type:  EnvVarPlainText,
	}
}

func K8sEnvVarVolumeMountDtoExampleData() K8sEnvVarDto {
	return K8sEnvVarDto{
		Name:  "yyy name",
		Value: "benetest:/:/test",
		Type:  EnvVarVolumeMount,
	}
}

func K8sEnvVarExternalSecretDtoExampleData() K8sEnvVarDto {
	return K8sEnvVarDto{
		Name:  "postgresURL",
		Value: "namePrefix/propertyname",
		Type:  EnvVarExternalSecret,
	}
}

func SplitEsoEnvVarValues(envVar K8sEnvVarDto) (string, string) {
	result := strings.Split(envVar.Value, "/")
	return result[0], result[1]
}
