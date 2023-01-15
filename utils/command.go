package utils

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

type Command struct {
	Id                      string `json:"id"`
	JobId                   string `json:"jobId"`
	NamespaceId             string `json:"namespaceId"`
	StageId                 string `json:"stageId,omitempty"`
	ServiceId               string `json:"serviceId,omitempty"`
	Title                   string `json:"title"`
	Message                 string `json:"message,omitempty"`
	StartedAt               string `json:"startedAt"`
	State                   string `json:"state"`
	DurationMs              int    `json:"durationMs"`
	MustSucceed             bool   `json:"mustSucceed"`
	ReportToNotificationSvc bool   `json:"reportToNotificationService"`
	IgnoreError             bool   `json:"ignoreError"`
}

func K8sNotificationDtoFromCommand(cmd Command) *dtos.K8sNotificationDto {
	return &dtos.K8sNotificationDto{
		Id:          cmd.Id,
		JobId:       cmd.JobId,
		NamespaceId: cmd.NamespaceId,
		StageId:     cmd.StageId,
		ServiceId:   cmd.ServiceId,
		Title:       cmd.Title,
		Message:     cmd.Message,
		StartedAt:   cmd.StartedAt,
		State:       cmd.State,
		DurationMs:  cmd.DurationMs,
	}
}

func CreateCommand(title string, c *websocket.Conn) Command {
	cmd := Command{
		Id:                      uuid.NewV4().String(),
		Title:                   title,
		Message:                 "",
		StartedAt:               time.Now().Format(time.RFC3339),
		State:                   "PENDING",
		DurationMs:              0,
		MustSucceed:             false,
		ReportToNotificationSvc: true,
		IgnoreError:             false,
	}
	ReportStateToServer(cmd, c)
	return cmd
}

func CreateBashCommand(title string, shellCmd string, c *websocket.Conn) Command {
	cmd := CreateCommand(title, c)
	go func(cmd Command) {
		cmd.Start(title, c)

		bashCommand := exec.Command("bash", "-c", shellCmd)
		err := bashCommand.Run()

		if err != nil {
			cmd.Fail(fmt.Sprintf("'%s' ERROR: %s", title, err.Error()), c)
		} else {
			cmd.Success(title, c)
		}
	}(cmd)
	return cmd
}

func (cmd *Command) Start(msg string, c *websocket.Conn) {
	cmd.State = "STARTED"
	cmd.Message = msg
	ReportStateToServer(cmd, c)
}

func (cmd *Command) Fail(error string, c *websocket.Conn) {
	cmd.State = "FAILED"
	cmd.Message = error
	ReportStateToServer(cmd, c)
}

func (cmd *Command) Success(msg string, c *websocket.Conn) {
	cmd.State = "SUCCEEDED"
	cmd.Message = msg
	ReportStateToServer(cmd, c)
}
