package structs

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

type Job struct {
	Id                      string     `json:"id"`
	NamespaceId             string     `json:"namespaceId"`
	StageId                 *string    `json:"stageId,omitempty"`
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
		ServiceId:   job.ServiceId,
		Title:       job.Title,
		Message:     job.Message,
		State:       job.State,
		DurationMs:  job.DurationMs,
	}
}

func CreateJob(title string, namespaceId string, stageId *string, serviceId *string, c *websocket.Conn) Job {
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
			result := CreateDatagramFromNotification(data, c)
			result.Send()
		} else {
			logger.Log.Error("Serialization failed.")
		}
	} else {
		logger.Log.Error("No connection to server.")
	}
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
	case "PENDING":
		fmt.Printf("   %s %s %s (%sms)\n", typeName, PEND(utils.FillWith(data.State, 15, " ")), utils.FillWith(data.Title, 96, " "), duration)
	case "STARTED":
		fmt.Printf("   %s %s %s (%sms)\n", typeName, STAR(utils.FillWith(data.State, 15, " ")), utils.FillWith(data.Title, 96, " "), duration)
	case "ERROR", "FAILED":
		fmt.Printf("   %s %s %s (%sms)\n", typeName, ERRO(utils.FillWith(data.State, 15, " ")), utils.FillWith(data.Title, 96, " "), duration)
		fmt.Printf("      %s %s %s\n", "", ERRO(utils.FillWith("--> ", 15, " ")), data.Message)
	case "SUCCEEDED":
		fmt.Printf("   %s %s %s (%sms)\n", typeName, SUCC(utils.FillWith(data.State, 15, " ")), utils.FillWith(data.Title, 96, " "), duration)
	default:
		fmt.Printf("   %s %s %s (%sms)\n", typeName, DEFA(utils.FillWith(data.State, 15, " ")), utils.FillWith(data.Title, 96, " "), duration)
	}
}

func StateDebugLog(debugStr string) {
	DEBUG := color.New(color.FgWhite, color.BgHiMagenta).SprintFunc()
	fmt.Printf("%-6s %-26s %s\n", "DEBUG", DEBUG(utils.FillWith("DEBUG", 15, " ")), debugStr)
}
