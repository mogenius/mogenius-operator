package structs

import (
	"mogenius-operator/src/utils"
	"mogenius-operator/src/websocket"
	"time"
)

type Command struct {
	Id       string       `json:"id"`
	Command  string       `json:"command"`
	Title    string       `json:"title"`
	Message  string       `json:"message,omitempty"`
	State    JobStateEnum `json:"state"`
	Started  time.Time    `json:"started"`
	Finished time.Time    `json:"finished"`
}

func CreateCommand(eventClient websocket.WebsocketClient, command string, title string, job *Job) *Command {
	cmd := &Command{
		Id:      utils.NanoId(),
		Command: command,
		Title:   title,
		Message: "",
		State:   JobStatePending,
		Started: time.Now(),
	}
	job.AddCmd(eventClient, cmd)
	return cmd
}

func (cmd *Command) Start(eventClient websocket.WebsocketClient, job *Job, msg string) {
	cmd.State = JobStateStarted
	cmd.Message = msg
	ReportCmdStateToServer(eventClient, job, cmd)
}

func (cmd *Command) Fail(eventClient websocket.WebsocketClient, job *Job, err string) {
	cmd.State = JobStateFailed
	cmd.Message = err
	cmd.Finished = time.Now()
	ReportCmdStateToServer(eventClient, job, cmd)
}

func (cmd *Command) Success(eventClient websocket.WebsocketClient, job *Job, msg string) {
	cmd.State = JobStateSucceeded
	cmd.Message = msg
	cmd.Finished = time.Now()
	ReportCmdStateToServer(eventClient, job, cmd)
}
