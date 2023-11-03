package dtos

type K8sServiceSettingsDto struct {
	LimitCpuCores         float32               `json:"limitCpuCores" validate:"required"`
	LimitMemoryMB         int                   `json:"limitMemoryMB" validate:"required"`
	EphemeralStorageMB    int                   `json:"ephemeralStorageMB" validate:"required"`
	ReplicaCount          int32                 `json:"replicaCount" validate:"required"`
	DeploymentStrategy    string                `json:"deploymentStrategy" validate:"required"` // "rolling", "recreate"
	ImagePullPolicy       string                `json:"imagePullPolicy"`                        // "Always", "Never", "IfNotPresent"
	ProbesOn              bool                  `json:"probesOn,omitempty"`
	K8sCronJobSettingsDto K8sCronJobSettingsDto `json:"k8sCronJobSettingsDto,omitempty"`
}

func K8sServiceSettingsDtoExampleData() K8sServiceSettingsDto {
	return K8sServiceSettingsDto{
		LimitCpuCores:         100,
		LimitMemoryMB:         128,
		EphemeralStorageMB:    200,
		ReplicaCount:          1,
		DeploymentStrategy:    "recreate",
		ImagePullPolicy:       "Always",
		ProbesOn:              false,
		K8sCronJobSettingsDto: K8sCronJobSettingsDtoExampleData(),
	}
}
