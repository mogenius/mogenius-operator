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
