package dtos

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
