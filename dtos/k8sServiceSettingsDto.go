package dtos

type K8sServiceSettingsDto struct {
	LimitCpuCores      float32 `json:"limitCpuCores" validate:"required"`
	LimitMemoryMB      int     `json:"limitMemoryMB" validate:"required"`
	EphemeralStorageMB int     `json:"ephemeralStorageMB" validate:"required"`
	ReplicaCount       int32   `json:"replicaCount" validate:"required"`
	DeploymentStrategy string  `json:"deploymentStrategy" validate:"required"` // "rolling", "recreate"
}

func K8sServiceSettingsDtoExampleData() K8sServiceSettingsDto {
	return K8sServiceSettingsDto{
		LimitCpuCores:      1,
		LimitMemoryMB:      1,
		EphemeralStorageMB: 1,
		ReplicaCount:       1,
		DeploymentStrategy: "recreate",
	}
}
