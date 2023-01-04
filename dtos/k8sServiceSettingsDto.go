package dtos

type K8sServiceSettingsDto struct {
	LimitCpuCores      int    `json:"limitCpuCores" validate:"required"`
	LimitMemoryMB      int    `json:"limitMemoryMB" validate:"required"`
	EphemeralStorageMB int    `json:"ephemeralStorageMB" validate:"required"`
	ReplicaCount       int    `json:"replicaCount" validate:"required"`
	DeploymentStrategy string `json:"deploymentStrategy" validate:"required"` // "rolling", "recreate"
}
