package dtos

type K8sServiceSettingsDto struct {
	LimitCpuCores      float32             `json:"limitCpuCores"`
	LimitMemoryMB      int                 `json:"limitMemoryMB"`
	EphemeralStorageMB int                 `json:"ephemeralStorageMB"`
	ImagePullPolicy    ImagePullPolicyEnum `json:"imagePullPolicy"`
}

func (d *K8sServiceSettingsDto) IsLimitSetup() bool {
	return d.LimitCpuCores != 0 || d.LimitMemoryMB != 0 || d.EphemeralStorageMB != 0
}
