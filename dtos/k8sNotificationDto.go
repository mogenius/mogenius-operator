package dtos

import punqStructs "github.com/mogenius/punq/structs"

type K8sNotificationDto struct {
	Id         string                   `json:"id"`
	JobId      string                   `json:"jobId"`
	ProjectId  string                   `json:"projectId"`
	Namespace  string                   `json:"namespaceId"`
	Service    string                   `json:"serviceId"`
	Title      string                   `json:"title"`
	Message    string                   `json:"message"`
	StartedAt  string                   `json:"startedAt"`
	State      punqStructs.JobStateEnum `json:"state"`
	DurationMs int64                    `json:"durationMs"`
	BuildId    int                      `json:"buildId,omitempty"`
}
