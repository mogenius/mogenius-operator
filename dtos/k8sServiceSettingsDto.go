package dtos

type K8sServiceSettingsDto struct {
	LimitCpuCores         float32                `json:"limitCpuCores"`
	LimitMemoryMB         int                    `json:"limitMemoryMB"`
	EphemeralStorageMB    int                    `json:"ephemeralStorageMB"`
	ReplicaCount          int32                  `json:"replicaCount"`
	DeploymentStrategy    DeploymentStrategyEnum `json:"deploymentStrategy"`
	ImagePullPolicy       ImagePullPolicyEnum    `json:"imagePullPolicy"`
	ProbesOn              bool                   `json:"probesOn,omitempty"`
	K8sCronJobSettingsDto *K8sCronJobSettingsDto `json:"k8sCronJobSettingsDto,omitempty"`
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

func (d *K8sServiceSettingsDto) IsLimitSetup() bool {
	return d.LimitCpuCores != 0 || d.LimitMemoryMB != 0 || d.EphemeralStorageMB == 0
}
