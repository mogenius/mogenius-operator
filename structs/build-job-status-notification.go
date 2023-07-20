package structs

type BuildJobStatusNotification struct {
	Id          string `json:"id"`
	JobId       string `json:"jobId"`
	ProjectId   string `json:"projectId"`
	NamespaceId string `json:"namespaceId"`
	ServiceId   string `json:"serviceId"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	StartedAt   string `json:"startedAt"`
	State       string `json:"state"` // FAILED, SUCCEEDED, STARTED, PENDING
	DurationMs  int    `json:"durationMs"`
	BuildId     int    `json:"buildId"`
}
