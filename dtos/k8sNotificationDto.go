package dtos

type K8sNotificationDto struct {
	Id          string  `json:"id"`
	JobId       string  `json:"jobId"`
	NamespaceId string  `json:"namespaceId"`
	StageId     *string `json:"stageId,omitempty"`
	ServiceId   *string `json:"serviceId,omitempty"`
	Title       string  `json:"title"`
	Message     string  `json:"message"`
	StartedAt   string  `json:"startedAt"`
	State       string  `json:"state"`
	DurationMs  int64   `json:"durationMs"`
}
