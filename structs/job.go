package structs

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
)

type DefaultResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type Job struct {
	Id             string       `json:"id"`
	ProjectId      string       `json:"projectId"`
	NamespaceName  string       `json:"namespaceName"`
	ControllerName string       `json:"controllerName"`
	Title          string       `json:"title"`
	Message        string       `json:"message"`
	Commands       []*Command   `json:"commands"`
	State          JobStateEnum `json:"state"`
	Started        time.Time    `json:"started"`
	Finished       time.Time    `json:"finished"`
	BuildId        uint64       `json:"buildId,omitempty"`
	ContainerName  string       `json:"containerName,omitempty"`
}

func CreateJob(title string, projectId string, namespace string, controllerName string) *Job {
	job := &Job{
		Id:             punqUtils.NanoId(),
		ProjectId:      projectId,
		NamespaceName:  namespace,
		ControllerName: controllerName,
		Title:          title,
		Message:        "",
		Commands:       []*Command{},
		State:          JobStatePending,
		Started:        time.Now(),
	}
	ReportJobStateToServer(job)
	return job
}

func (j *Job) Start() {
	j.State = JobStateStarted
	ReportJobStateToServer(j)
}

func (j *Job) DefaultReponse() DefaultResponse {
	dr := DefaultResponse{}
	if j.State == JobStateFailed {
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
	j.State = JobStateFailed
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
	if j.State == JobStateFailed {
		allSuccess = false
		failedCmd = j.Title
	}
	if allSuccess {
		j.State = JobStateSucceeded
	} else {
		j.State = JobStateFailed
		j.Message = fmt.Sprintf("%s FAILED.", failedCmd)
	}
	j.Finished = time.Now()

	ReportJobStateToServer(j)
}

func (j *Job) AddCmd(cmd *Command) {
	j.Commands = append(j.Commands, cmd)
	ReportCmdStateToServer(j, cmd)
}

func (j *Job) AddCmds(cmds []*Command) {
	for _, cmd := range cmds {
		j.AddCmd(cmd)
	}
}

func ReportJobStateToServer(job *Job) {
	stateLogJob(job)
	result := CreateDatagramNotificationFromJob(job)
	EventServerSendData(result, "", "", "", 1)
}

func ReportCmdStateToServer(job *Job, cmd *Command) {
	stateLogCmd(cmd)
	result := CreateDatagramNotificationFromJob(job)
	EventServerSendData(result, "", "", "", 1)
}

func stateLogJob(data *Job) {
	typeName := "JOB"
	PEND := color.New(color.FgWhite, color.BgBlue).SprintFunc()
	STAR := color.New(color.FgWhite, color.BgYellow).SprintFunc()
	ERRO := color.New(color.FgWhite, color.BgRed).SprintFunc()
	SUCC := color.New(color.FgWhite, color.BgGreen).SprintFunc()
	DEFA := color.New(color.FgWhite, color.BgCyan).SprintFunc()
	LONG := color.New(color.FgRed).SprintFunc()

	// COLOR MILLISECONDS IF >500
	durationMs := data.Finished.Sub(data.Started).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	duration := fmt.Sprintf("%d", durationMs)
	if durationMs > 500 {
		duration = LONG(fmt.Sprintf("%d", durationMs))
	}

	switch data.State {
	case JobStatePending:
		log.Infof("   %s %s %s (%sms)\n", typeName, PEND(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	case JobStateStarted:
		log.Infof("   %s %s %s (%sms)\n", typeName, STAR(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	case JobStateFailed, JobStateTimeout, JobStateCanceled:
		log.Infof("   %s %s %s (%sms)\n", typeName, ERRO(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
		log.Infof("      %s %s %s\n", "", ERRO(punqUtils.FillWith("--> ", 15, " ")), data.Message)
	case JobStateSucceeded:
		log.Infof("   %s %s %s (%sms)\n", typeName, SUCC(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	default:
		log.Infof("   %s %s %s (%sms)\n", typeName, DEFA(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	}
}

func stateLogCmd(data *Command) {
	typeName := "CMD"
	PEND := color.New(color.FgWhite, color.BgBlue).SprintFunc()
	STAR := color.New(color.FgWhite, color.BgYellow).SprintFunc()
	ERRO := color.New(color.FgWhite, color.BgRed).SprintFunc()
	SUCC := color.New(color.FgWhite, color.BgGreen).SprintFunc()
	DEFA := color.New(color.FgWhite, color.BgCyan).SprintFunc()
	LONG := color.New(color.FgRed).SprintFunc()

	// COLOR MILLISECONDS IF >500
	durationMs := data.Finished.Sub(data.Started).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	duration := fmt.Sprintf("%d", durationMs)
	if durationMs > 500 {
		duration = LONG(fmt.Sprintf("%d", durationMs))
	}

	switch data.State {
	case JobStatePending:
		log.Infof("   %s %s %s (%sms)\n", typeName, PEND(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	case JobStateStarted:
		log.Infof("   %s %s %s (%sms)\n", typeName, STAR(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	case JobStateFailed, JobStateTimeout, JobStateCanceled:
		log.Infof("   %s %s %s (%sms)\n", typeName, ERRO(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
		log.Infof("      %s %s %s\n", "", ERRO(punqUtils.FillWith("--> ", 15, " ")), data.Message)
	case JobStateSucceeded:
		log.Infof("   %s %s %s (%sms)\n", typeName, SUCC(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	default:
		log.Infof("   %s %s %s (%sms)\n", typeName, DEFA(punqUtils.FillWith(string(data.State), 15, " ")), punqUtils.FillWith(data.Title, 96, " "), duration)
	}
}

// func StateDebugLog(debugStr string) {
// 	DEBUG := color.New(color.FgWhite, color.BgHiMagenta).SprintFunc()
// 	log.Infof("%-6s %-26s %s\n", "DEBUG", DEBUG(punqUtils.FillWith("DEBUG", 15, " ")), debugStr)
// }
