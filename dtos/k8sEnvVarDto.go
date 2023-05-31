package dtos

type K8sEnvVarDto struct {
	Name  string `json:"name" validate:"required"`
	Value string `json:"value" validate:"required"`
	Type  string `json:"type" validate:"required"` // "PLAINTEXT", "KEY_VAULT", "VOLUME_MOUNT", "VOLUME_MOUNT_SEED", "CHANGE_OWNER", "HOSTNAME"
}

func K8sEnvVarDtoExampleData() K8sEnvVarDto {
	return K8sEnvVarDto{
		Name:  "name",
		Value: "value",
		Type:  "PLAINTEXT",
	}
}

func K8sEnvVarVolumeMountDtoExampleData() K8sEnvVarDto {
	return K8sEnvVarDto{
		Name:  "xxx name",
		Value: "benetest:/:/test",
		Type:  "VOLUME_MOUNT",
	}
}
