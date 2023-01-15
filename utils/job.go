package utils

import (
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

type Job struct {
	Id                      string    `json:"id"`
	NamespaceId             string    `json:"namespaceId"`
	StageId                 string    `json:"stageId,omitempty"`
	ServiceId               *string   `json:"serviceId,omitempty"`
	Title                   string    `json:"title"`
	Message                 string    `json:"message"`
	Commads                 []Command `json:"commands"`
	DurationMs              int       `json:"durationMs"`
	State                   string    `json:"state"`
	ReportToNotificationSvc bool      `json:"reportToNotificationService"`
}

func K8sNotificationDtoFromJob(job Job) *dtos.K8sNotificationDto {
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

func CreateJob(title string, namespaceId string, stageId string, serviceId *string) Job {
	return Job{
		Id:                      uuid.NewV4().String(),
		NamespaceId:             namespaceId,
		StageId:                 stageId,
		ServiceId:               serviceId,
		Title:                   title,
		Message:                 "",
		Commads:                 []Command{},
		State:                   "STARTED",
		DurationMs:              0,
		ReportToNotificationSvc: true,
	}
}

func (j *Job) AddCmd(cmd Command) {
	j.Commads = append(j.Commads, cmd)
}

func (j *Job) AddCmds(cmds []Command) {
	j.Commads = append(j.Commads, cmds...)
}

func (j *Job) Report(c *websocket.Conn) {
	logger.Log.Infof("Job %s started (Cmds=%d).", j.Id, len(j.Commads))
	ReportStateToServer(j, c)
}

func ReportStateToServer(cmdOrJob interface{}, c *websocket.Conn) {
	if c != nil {
		var data *dtos.K8sNotificationDto = nil
		switch cmdOrJob.(type) {
		case Job:
			if cmdOrJob.(Job).ReportToNotificationSvc && cmdOrJob.(Job).NamespaceId != "" {
				data = K8sNotificationDtoFromCommand(cmdOrJob.(Command))
			}

		case Command:
			if cmdOrJob.(Command).ReportToNotificationSvc && cmdOrJob.(Command).NamespaceId != "" {
				data = K8sNotificationDtoFromJob(cmdOrJob.(Job))
			}
		}

		if data != nil {
			if data.State == "ERROR" || data.State == "FAILED" {
				logger.Log.Errorf("%s %s (%sms)", data.State, data.Title, data.DurationMs)
			} else {
				logger.Log.Infof("%s %s (%sms)", data.State, data.Title, data.DurationMs)
			}

			datagram := structs.Datagram{}
			result := structs.CreateDatagramRequest(datagram, data)
			c.WriteJSON(result)
		}
	} else {
		logger.Log.Error("No connection to server.")
	}
}
