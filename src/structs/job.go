package structs

import (
	"fmt"
	"time"

	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/shell"
	"mogenius-k8s-manager/src/utils"
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
	ContainerName  string       `json:"containerName,omitempty"`
}

func CreateJob(title string, projectId string, namespace string, controllerName string) *Job {
	job := &Job{
		Id:             utils.NanoId(),
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
	stateLogCmd(cmd, job.NamespaceName, job.ControllerName)
	result := CreateDatagramNotificationFromJob(job)
	EventServerSendData(result, "", "", "", 1)
}

func stateLogJob(data *Job) {
	typeName := "JOB"
	// COLOR MILLISECONDS IF >500
	durationMs := data.Finished.Sub(data.Started).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	duration := fmt.Sprintf("%d", durationMs)
	if durationMs > 500 {
		duration = shell.Colorize(fmt.Sprintf("%d", durationMs), shell.Red)
	}

	serviceLogger, err := logManager.GetLogger("services")
	if err != nil {
		assert.Assert(serviceLogger != nil, "serviceLogger has to be initialized for stateLogJob")
	}

	var message string
	switch data.State {
	case JobStatePending:
		message = fmt.Sprintf(
			"   %s %s %s (%sms)\n",
			typeName,
			shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgBlue),
			utils.FillWith(data.Title, 96, " "),
			duration,
		)
	case JobStateStarted:
		message = fmt.Sprintf(
			"   %s %s %s (%sms)\n",
			typeName,
			shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgYellow),
			utils.FillWith(data.Title, 96, " "),
			duration,
		)
	case JobStateFailed, JobStateTimeout, JobStateCanceled:
		message = fmt.Sprintf(
			"   %s %s %s (%sms)\n",
			typeName,
			shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgRed),
			utils.FillWith(data.Title, 96, " "),
			duration,
		)
	case JobStateSucceeded:
		message = fmt.Sprintf(
			"   %s %s %s (%sms)\n",
			typeName,
			shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgGreen),
			utils.FillWith(data.Title, 96, " "),
			duration,
		)
	default:
		message = fmt.Sprintf(
			"   %s %s %s (%sms)\n",
			typeName,
			shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgCyan),
			utils.FillWith(data.Title, 96, " "),
			duration,
		)
	}
	serviceLogger.Info(message, "namespace", data.NamespaceName, "controllerName", data.ControllerName)
}

func stateLogCmd(data *Command, ns string, controllerName string) {
	typeName := "CMD"

	// COLOR MILLISECONDS IF >500
	durationMs := data.Finished.Sub(data.Started).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	duration := fmt.Sprintf("%d", durationMs)
	if durationMs > 500 {
		duration = shell.Colorize(fmt.Sprintf("%d", durationMs), shell.Red)
	}

	serviceLogger, err := logManager.GetLogger("services")
	if err != nil {
		assert.Assert(serviceLogger != nil, "serviceLogger has to be initialized for stateLogCmd")
	}

	var message string
	switch data.State {
	case JobStatePending:
		message = fmt.Sprintf(
			"   %s %s %s (%sms)\n",
			typeName, shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgYellow),
			utils.FillWith(data.Title, 96, " "),
			duration,
		)
	case JobStateStarted:
		message = fmt.Sprintf("   %s %s %s (%sms)\n",
			typeName,
			shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgYellow),
			utils.FillWith(data.Title, 96, " "),
			duration,
		)
	case JobStateFailed, JobStateTimeout, JobStateCanceled:
		message = fmt.Sprintf(
			"   %s %s %s (%sms)%s",
			typeName,
			shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgRed),
			utils.FillWith(data.Title, 96, " "),
			duration,
			"\n"+data.Message,
		)
	case JobStateSucceeded:
		message = fmt.Sprintf(
			"   %s %s %s (%sms)\n",
			typeName,
			shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgGreen),
			utils.FillWith(data.Title, 96, " "),
			duration,
		)
	default:
		message = fmt.Sprintf(
			"   %s %s %s (%sms)\n",
			typeName,
			shell.Colorize(utils.FillWith(string(data.State), 15, " "), shell.White, shell.BgCyan),
			utils.FillWith(data.Title, 96, " "),
			duration,
		)
	}
	serviceLogger.Info(message, "namespace", ns, "controllerName", controllerName)
}
