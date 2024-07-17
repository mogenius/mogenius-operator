package dtos

type K8sCronJobSettingsDto struct {
	// number of allowed restarts
	Schedule     string `json:"schedule" validate:"required"`
	BackoffLimit int32  `json:"backoffLimit,omitempty"`
	// maximum duration
	ActiveDeadlineSeconds int64 `json:"activeDeadlineSeconds,omitempty"`
	// pod history limits
	FailedJobsHistoryLimit     int32 `json:"failedJobsHistoryLimit,omitempty"`
	SuccessfulJobsHistoryLimit int32 `json:"successfulJobsHistoryLimit,omitempty"`
}

func K8sCronJobSettingsDtoExampleData() *K8sCronJobSettingsDto {
	return &K8sCronJobSettingsDto{
		Schedule:              "*/15 * * * *",
		BackoffLimit:          2,
		ActiveDeadlineSeconds: 120,
	}
}
