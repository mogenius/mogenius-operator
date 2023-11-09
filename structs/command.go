package structs

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"os/exec"
	"sync"
	"time"

	punqStructs "github.com/mogenius/punq/structs"
	uuid "github.com/satori/go.uuid"
)

type Command punqStructs.Command

func K8sNotificationDtoFromCommand(cmd *Command) *dtos.K8sNotificationDto {
	return &dtos.K8sNotificationDto{
		Id:          cmd.Id,
		JobId:       cmd.JobId,
		ProjectId:   cmd.ProjectId,
		NamespaceId: cmd.NamespaceId,
		ServiceId:   cmd.ServiceId,
		Title:       cmd.Title,
		Message:     cmd.Message,
		StartedAt:   cmd.StartedAt,
		State:       cmd.State,
		DurationMs:  cmd.DurationMs,
	}
}

func CreateCommand(title string, job *Job) *Command {
	cmd := Command{
		Id:                      uuid.NewV4().String(),
		JobId:                   job.Id,
		ProjectId:               job.ProjectId,
		NamespaceId:             job.NamespaceId,
		ServiceId:               job.ServiceId,
		Title:                   title,
		Message:                 "",
		StartedAt:               time.Now().Format(time.RFC3339),
		State:                   "PENDING",
		DurationMs:              0,
		MustSucceed:             false,
		ReportToNotificationSvc: true,
		IgnoreError:             false,
		Started:                 time.Now(),
	}
	ReportStateToServer(nil, &cmd)
	return &cmd
}

func CreateBashCommand(title string, job *Job, shellCmd string, wg *sync.WaitGroup) *Command {
	wg.Add(1)
	cmd := CreateCommand(title, job)
	go func(cmd *Command) {
		defer wg.Done()
		cmd.Start(title)

		output, err := exec.Command("bash", "-c", shellCmd).Output()
		fmt.Println(string(shellCmd))
		fmt.Println(string(output))
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			errorMsg := string(exitErr.Stderr)
			logger.Log.Error(shellCmd)
			logger.Log.Errorf("%d: %s", exitCode, errorMsg)
			cmd.Fail(fmt.Sprintf("'%s' ERROR: %s", title, errorMsg))
		} else if err != nil {
			logger.Log.Error("exec.Command: %s", err.Error())
		} else {
			cmd.Success(title)
		}
	}(cmd)
	return cmd
}

func (cmd *Command) Start(msg string) {
	cmd.State = "STARTED"
	cmd.Message = msg
	cmd.DurationMs = time.Now().UnixMilli() - cmd.Started.UnixMilli()
	ReportStateToServer(nil, cmd)
}

func (cmd *Command) Fail(error string) {
	cmd.State = "FAILED"
	cmd.Message = error
	cmd.DurationMs = time.Now().UnixMilli() - cmd.Started.UnixMilli()
	if utils.CONFIG.Misc.Debug {
		logger.Log.Errorf("Command '%s' failed: %s", cmd.Title, error)
	}
	ReportStateToServer(nil, cmd)
}

func (cmd *Command) Success(msg string) {
	cmd.State = "SUCCEEDED"
	cmd.Message = msg
	cmd.DurationMs = time.Now().UnixMilli() - cmd.Started.UnixMilli()
	ReportStateToServer(nil, cmd)
}
