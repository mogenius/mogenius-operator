package utils

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"time"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

type Job struct {
	Id                      string     `json:"id"`
	NamespaceId             string     `json:"namespaceId"`
	StageId                 string     `json:"stageId,omitempty"`
	ServiceId               *string    `json:"serviceId,omitempty"`
	Title                   string     `json:"title"`
	Message                 string     `json:"message"`
	Commands                []*Command `json:"Commands"`
	DurationMs              int64      `json:"durationMs"`
	State                   string     `json:"state"`
	ReportToNotificationSvc bool       `json:"reportToNotificationService"`
	Started                 time.Time
}

func K8sNotificationDtoFromJob(job *Job) *dtos.K8sNotificationDto {
	return &dtos.K8sNotificationDto{
		Id:          job.Id,
		JobId:       job.Id,
		NamespaceId: job.NamespaceId,
		StageId:     job.StageId,
		Title:       job.Title,
		Message:     job.Message,
		State:       job.State,
		DurationMs:  job.DurationMs,
	}
}

func CreateJob(title string, namespaceId string, stageId string, serviceId *string, c *websocket.Conn) Job {
	job := Job{
		Id:                      uuid.NewV4().String(),
		NamespaceId:             namespaceId,
		StageId:                 stageId,
		ServiceId:               serviceId,
		Title:                   title,
		Message:                 "",
		Commands:                []*Command{},
		State:                   "PENDING",
		DurationMs:              0,
		ReportToNotificationSvc: true,
		Started:                 time.Now(),
	}
	ReportStateToServer(&job, nil, c)
	return job
}

func (j *Job) Start(c *websocket.Conn) {
	j.State = "STARTED"
	j.DurationMs = time.Now().UnixMilli() - j.Started.UnixMilli()
	ReportStateToServer(j, nil, c)
}

func (j *Job) Finish(c *websocket.Conn) {
	var allSuccess = true
	var failedCmd = ""
	for _, cmd := range j.Commands {
		if cmd.State != "SUCCEEDED" {
			allSuccess = false
			failedCmd = cmd.Title
		}
	}
	if allSuccess {
		j.State = "SUCCEEDED"
		j.DurationMs = time.Now().UnixMilli() - j.Started.UnixMilli()
	} else {
		j.State = "FAILED"
		j.Message = fmt.Sprintf("%s FAILED.", failedCmd)
		j.DurationMs = time.Now().UnixMilli() - j.Started.UnixMilli()
	}
	ReportStateToServer(j, nil, c)
}

func (j *Job) AddCmd(cmd *Command) {
	j.Commands = append(j.Commands, cmd)
}

func (j *Job) AddCmds(cmds []*Command) {
	j.Commands = append(j.Commands, cmds...)
}

func ReportStateToServer(job *Job, cmd *Command, c *websocket.Conn) {
	if c != nil {
		var data *dtos.K8sNotificationDto = nil
		typeName := ""

		if cmd != nil {
			typeName = "CMD"
			if cmd.ReportToNotificationSvc && cmd.NamespaceId != "" {
				data = K8sNotificationDtoFromCommand(cmd)
			}
		} else if job != nil {
			typeName = "JOB"
			if job.ReportToNotificationSvc && job.NamespaceId != "" {
				data = K8sNotificationDtoFromJob(job)
			}
		} else {
			logger.Log.Error("Job AND Command cannot be nil")
		}

		if data != nil {
			stateLog(typeName, data)
			result := structs.CreateDatagramFrom("K8sNotificationDto", data, c)
			result.Send()
		} else {
			logger.Log.Error("Serialization failed.")
		}
	} else {
		logger.Log.Error("No connection to server.")
	}
}

func stateLog(typeName string, data *dtos.K8sNotificationDto) {
	if data.State == "ERROR" || data.State == "FAILED" {
		fmt.Printf("%-10s %-6s %-60s (%dms)\n", data.State, typeName, data.Title, data.DurationMs)
	} else {
		fmt.Printf("%-10s %-6s %-60s (%dms)\n", data.State, typeName, truncateText(data.Title, 50), data.DurationMs)
	}
}

func truncateText(s string, max int) string {
	if max > len(s) {
		return s
	}
	return s[:max] + " ..."
}
