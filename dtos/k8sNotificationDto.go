package dtos

type K8sNotificationDto struct {
	Id              string `json:"id"`
	JobId           string `json:"jobId"`
	NamespaceId     string `json:"namespaceId"`
	StageId         string `json:"stageId,omitempty"`
	ServiceId       string `json:"serviceId,omitempty"`
	Title           string `json:"title"`
	Message         string `json:"message"`
	StartedAt       string `json:"startedAt"`
	State           string `json:"state"`
	DurationMs      int    `json:"durationMs"`
	CommitAuthor    string `json:"commitAuthor,omitempty"`
	CommitMessage   string `json:"commitMessage,omitempty"`
	CommitHash      string `json:"commitHash,omitempty"`
	BuildId         string `json:"buildId,omitempty"`
	BuildState      string `json:"buildState,omitempty"`
	DeploymentState string `json:"deploymentState,omitempty"`
}
