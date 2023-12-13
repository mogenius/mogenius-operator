package dtos

type K8sCronJobSettingsDto struct {
	SourceType K8sServiceTypeEnum `json:"source" validate:"required"`
	// number of allowed restarts
	Schedule     string `json:"schedule" validate:"required"`
	BackoffLimit int32  `json:"backoffLimit,omitempty"`
	// maximum duration
	ActiveDeadlineSeconds int64 `json:"activeDeadlineSeconds,omitempty"`
}

func K8sCronJobSettingsDtoExampleData() *K8sCronJobSettingsDto {
	return &K8sCronJobSettingsDto{
		SourceType:            ContainerImage,
		Schedule:              "*/15 * * * *",
		BackoffLimit:          2,
		ActiveDeadlineSeconds: 120,
	}
}
