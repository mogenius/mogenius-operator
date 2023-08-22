package dtos

type K8sCronJobSettingsDto struct {
	SourceType            K8sServiceTypeEnum    `json:"source" validate:"required"`
	// number of allowed restarts
	Schedule              string                `json:"schedule" validate:"required"`
	BackOffLimit          int                   `json:"backOffLimit,omitempty"`
	// maximum duration
	ActiveDeadlineSeconds int                   `json:"activeDeadlineSeconds,omitempty"`
}

func K8sCronJobSettingsDtoExampleData() K8sCronJobSettingsDto {
	return K8sCronJobSettingsDto{
		SourceType: 		   ContainerImage,
		Schedule:              "*/15 * * * *",
		BackOffLimit:          2,
		ActiveDeadlineSeconds: 120,
	}
}
