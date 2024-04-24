package structs

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"time"

	"github.com/fatih/color"
	punq "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
)

type DefaultResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type Job struct {
	Id                      string            `json:"id"`
	ProjectId               string            `json:"projectId"`
	Namespace               string            `json:"namespace,omitempty"`
	Service                 string            `json:"service,omitempty"`
	Title                   string            `json:"title"`
	Message                 string            `json:"message"`
	Commands                []Command         `json:"Commands"`
	DurationMs              int64             `json:"durationMs"`
	State                   punq.JobStateEnum `json:"state"`
	ReportToNotificationSvc bool              `json:"reportToNotificationService"`
	Started                 time.Time
}

func K8sNotificationDtoFromJob(job *Job) *dtos.K8sNotificationDto {
	return &dtos.K8sNotificationDto{
		Id:         job.Id,
		JobId:      job.Id,
		ProjectId:  job.ProjectId,
		Namespace:  job.Namespace,
		Service:    job.Service,
		Title:      job.Title,
		Message:    job.Message,
		State:      job.State,
		DurationMs: job.DurationMs,
	}
}

func CreateJob(title string, projectId string, namespace string, service string) Job {
	job := Job{
		Id:                      punqUtils.NanoId(),
		ProjectId:               projectId,
		Namespace:               namespace,
		Service:                 service,
		Title:                   title,
		Message:                 "",
		Commands:                []Command{},
		State:                   punq.JobStatePending,
		DurationMs:              0,
		ReportToNotificationSvc: true,
		Started:                 time.Now(),
	}
	return job
}

func (j *Job) Start() {
	j.State = punq.JobStateStarted
	j.DurationMs = time.Now().UnixMilli() - j.Started.UnixMilli()
	ReportStateToServer(j)
}

func (j *Job) DefaultReponse() DefaultResponse {
	dr := DefaultResponse{}
	if j.State == punq.JobStateFailed {
		dr.Success = false
		if j.Message != "" {
			dr.Error = fmt.Sprintf("%s\n", j.Message)
		}
		for _, cmd := range j.Commands {
			if cmd.State == JobStateFailed {
				dr.Error += fmt.Sprintf("%s\n", cmd.Message)
			}
		}
	} else {
		dr.Success = true
	}
	return dr
}

func (j *Job) Fail(msg string) {
	j.State = punq.JobStateFailed
	j.Message = msg
}

func (j *Job) Finish() {
	var allSuccess = true
	var failedCmd = ""
	for _, cmd := range j.Commands {
		if cmd.State != JobStateSucceeded {
			allSuccess = false
			failedCmd = cmd.Message
		}
	}
	if j.State == punq.JobStateFailed {
		allSuccess = false
		failedCmd = j.Title
	}
	if allSuccess {
		j.State = punq.JobStateSucceeded
		j.DurationMs = time.Now().UnixMilli() - j.Started.UnixMilli()
	} else {
		j.State = punq.JobStateFailed
		j.Message = fmt.Sprintf("%s FAILED.", failedCmd)
		j.DurationMs = time.Now().UnixMilli() - j.Started.UnixMilli()
	}
	ReportStateToServer(j)
}

func (j *Job) AddCmd(cmd Command) {
	j.Commands = append(j.Commands, cmd)
}

func (j *Job) AddCmds(cmds []Command) {
	for _, cmd := range cmds {
		j.AddCmd(cmd)
	}
}

func ReportStateToServer(job *Job) {
	var data *dtos.K8sNotificationDto = K8sNotificationDtoFromJob(job)
	stateLog("JOB", data)
	result := CreateDatagramFromNotification(data)
	EventServerSendData(result, "", "", "", 1)
}

func stateLog(typeName string, data *dtos.K8sNotificationDto) {
	PEND := color.New(color.FgWhite, color.BgBlue).SprintFunc()
	STAR := color.New(color.FgWhite, color.BgYellow).SprintFunc()
	ERRO := color.New(color.FgWhite, color.BgRed).SprintFunc()
	SUCC := color.New(color.FgWhite, color.BgGreen).SprintFunc()
	DEFA := color.New(color.FgWhite, color.BgCyan).SprintFunc()
	LONG := color.New(color.FgRed).SprintFunc()

	// COLOR MILLISECONDS IF >500
	duration := fmt.Sprint(data.DurationMs)
	if data.DurationMs > 500 {
		duration = LONG(duration)
	}

	switch data.State {
	case punq.JobStatePending:
		log.Infof("   %s %s %s (%sms)\n", typeName, PEND(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	case punq.JobStateStarted:
		log.Infof("   %s %s %s (%sms)\n", typeName, STAR(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	case punq.JobStateFailed, punq.JobStateTimeout, punq.JobStateCanceled:
		log.Infof("   %s %s %s (%sms)\n", typeName, ERRO(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
		log.Infof("      %s %s %s\n", "", ERRO(punqUtils.FillWith("--> ", 15, " ")), data.Message)
	case punq.JobStateSucceeded:
		log.Infof("   %s %s %s (%sms)\n", typeName, SUCC(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	default:
		log.Infof("   %s %s %s (%sms)\n", typeName, DEFA(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	}
}

func StateDebugLog(debugStr string) {
	DEBUG := color.New(color.FgWhite, color.BgHiMagenta).SprintFunc()
	log.Infof("%-6s %-26s %s\n", "DEBUG", DEBUG(punqUtils.FillWith("DEBUG", 15, " ")), debugStr)
}
