package dtos

type K8sServiceSettingsDto struct {
	LimitCpuCores      float32             `json:"limitCpuCores"`
	LimitMemoryMB      int                 `json:"limitMemoryMB"`
	EphemeralStorageMB int                 `json:"ephemeralStorageMB"`
	ImagePullPolicy    ImagePullPolicyEnum `json:"imagePullPolicy"`
	ProbesOn           bool                `json:"probesOn,omitempty"`
}

func K8sServiceSettingsDtoExampleData() K8sServiceSettingsDto {
	return K8sServiceSettingsDto{
		LimitCpuCores:      100,
		LimitMemoryMB:      128,
		EphemeralStorageMB: 200,
		ImagePullPolicy:    "Always",
		ProbesOn:           false,
	}
}

func (d *K8sServiceSettingsDto) IsLimitSetup() bool {
	return d.LimitCpuCores != 0 || d.LimitMemoryMB != 0 || d.EphemeralStorageMB != 0
}
