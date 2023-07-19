package structs

type BuildJobStatusNotification struct {
	Id              string `json:"id"`
	JobId           string `json:"jobId"`
	ProjectId       string `json:"projectId"`
	NamespaceId     string `json:"namespaceId"`
	ServiceId       string `json:"serviceId"`
	Title           string `json:"title"`
	Message         string `json:"message"`
	StartedAt       string `json:"startedAt"`
	State           string `json:"state"` // FAILED, SUCCEEDED, STARTED, PENDING
	DurationMs      string `json:"durationMs"`
	BuildId         int    `json:"buildId"`
	BuildState      string `json:"buildState"`      // FAILED, FINISHED, STARTED, PENDING
	DeploymentState string `json:"deploymentState"` // FAILED, FINISHED, STARTED, PENDING
}
