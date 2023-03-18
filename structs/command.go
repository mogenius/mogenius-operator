package structs

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

type Command struct {
	Id                      string  `json:"id"`
	JobId                   string  `json:"jobId"`
	NamespaceId             string  `json:"namespaceId"`
	StageId                 *string `json:"stageId,omitempty"`
	ServiceId               *string `json:"serviceId,omitempty"`
	Title                   string  `json:"title"`
	Message                 string  `json:"message,omitempty"`
	StartedAt               string  `json:"startedAt"`
	State                   string  `json:"state"`
	DurationMs              int64   `json:"durationMs"`
	MustSucceed             bool    `json:"mustSucceed"`
	ReportToNotificationSvc bool    `json:"reportToNotificationService"`
	IgnoreError             bool    `json:"ignoreError"`
	Started                 time.Time
}

func K8sNotificationDtoFromCommand(cmd *Command) *dtos.K8sNotificationDto {
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

func CreateCommand(title string, job *Job, c *websocket.Conn) *Command {
	cmd := Command{
		Id:                      uuid.NewV4().String(),
		JobId:                   job.Id,
		StageId:                 job.StageId,
		ServiceId:               job.ServiceId,
		Title:                   title,
		NamespaceId:             job.NamespaceId,
		Message:                 "",
		StartedAt:               time.Now().Format(time.RFC3339),
		State:                   "PENDING",
		DurationMs:              0,
		MustSucceed:             false,
		ReportToNotificationSvc: true,
		IgnoreError:             false,
		Started:                 time.Now(),
	}
	ReportStateToServer(nil, &cmd, c)
	return &cmd
}

func CreateBashCommand(title string, job *Job, shellCmd string, c *websocket.Conn) *Command {
	cmd := CreateCommand(title, job, c)
	go func(cmd *Command) {
		cmd.Start(title, c)

		_, err := exec.Command("bash", "-c", shellCmd).Output()
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			errorMsg := string(exitErr.Stderr)
			logger.Log.Error(shellCmd)
			logger.Log.Errorf("%d: %s", exitCode, errorMsg)
			cmd.Fail(fmt.Sprintf("'%s' ERROR: %s", title, errorMsg), c)
		} else if err != nil {
			logger.Log.Error("exec.Command: %s", err.Error())
		} else {
			cmd.Success(title, c)
		}
	}(cmd)
	return cmd
}

func ExecuteBashCommandSilent(title string, shellCmd string) {
	_, err := exec.Command("bash", "-c", shellCmd).Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		errorMsg := string(exitErr.Stderr)
		logger.Log.Error(shellCmd)
		logger.Log.Errorf("%d: %s", exitCode, errorMsg)
	} else if err != nil {
		logger.Log.Errorf("ERROR: '%s': %s\n", title, err.Error())
	} else {
		logger.Log.Infof("SUCCESS '%s': %s\n", title, shellCmd)
	}
}

func (cmd *Command) Start(msg string, c *websocket.Conn) {
	cmd.State = "STARTED"
	cmd.Message = msg
	cmd.DurationMs = time.Now().UnixMilli() - cmd.Started.UnixMilli()
	ReportStateToServer(nil, cmd, c)
}

func (cmd *Command) Fail(error string, c *websocket.Conn) {
	cmd.State = "FAILED"
	cmd.Message = error
	cmd.DurationMs = time.Now().UnixMilli() - cmd.Started.UnixMilli()
	ReportStateToServer(nil, cmd, c)
}

func (cmd *Command) Success(msg string, c *websocket.Conn) {
	cmd.State = "SUCCEEDED"
	cmd.Message = msg
	cmd.DurationMs = time.Now().UnixMilli() - cmd.Started.UnixMilli()
	ReportStateToServer(nil, cmd, c)
}
