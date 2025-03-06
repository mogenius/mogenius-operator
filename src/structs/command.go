package structs

import (
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/websocket"
	"os/exec"
	"strconv"
	"sync"
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

func CreateShellCommand(eventClient websocket.WebsocketClient, command string, title string, job *Job, shellCmd string, wg *sync.WaitGroup) {
	wg.Add(1)
	cmd := CreateCommand(eventClient, command, title, job)
	go func() {
		defer wg.Done()
		cmd.Start(eventClient, job, title)

		output, err := exec.Command("sh", "-c", shellCmd).Output()
		structsLogger.Debug("executed command", "cmd", string(shellCmd), "output", string(output))

		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			errorMsg := string(exitErr.Stderr)
			structsLogger.Error("command failed", "cmd", shellCmd, "exitCode", exitCode, "errorMsg", errorMsg)
			cmd.Fail(eventClient, job, fmt.Sprintf("'%s' ERROR: %s", title, errorMsg))
		} else if err != nil {
			structsLogger.Error("exec.Command", "cmd", shellCmd, "error", err)
		} else {
			cmd.Success(eventClient, job, title)
		}
	}()
}

func CreateShellCommandGoRoutine(title string, shellCmd string, successFunc func(), failFunc func(output string, err error)) {
	go func() {
		output, err := exec.Command("sh", "-c", shellCmd).Output()
		structsLogger.Debug("executed command", "cmd", string(shellCmd), "output", string(output))

		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			errorMsg := string(exitErr.Stderr)
			structsLogger.Error("command failed", "cmd", shellCmd, "exitCode", exitCode, "errorMsg", errorMsg)
			if failFunc != nil {
				failFunc(string(output), exitErr)
			}
		} else if err != nil {
			structsLogger.Error("exec.Command", "error", err)
			if failFunc != nil {
				failFunc(string(output), err)
			}
		} else {
			structsLogger.Debug("SUCCESS", "shellCmd", shellCmd)
			if successFunc != nil {
				successFunc()
			}
		}
	}()
}

func (cmd *Command) Start(eventClient websocket.WebsocketClient, job *Job, msg string) {
	cmd.State = JobStateStarted
	cmd.Message = msg
	ReportCmdStateToServer(eventClient, job, cmd)
}

func (cmd *Command) Fail(eventClient websocket.WebsocketClient, job *Job, err string) {
	moDebug, erro := strconv.ParseBool(config.Get("MO_DEBUG"))
	assert.Assert(erro == nil)

	cmd.State = JobStateFailed
	cmd.Message = err
	cmd.Finished = time.Now()
	if moDebug {
		structsLogger.Error("Command failed", "title", cmd.Title, "message", cmd.Message, "error", err, "namespace", job.NamespaceName, "controller", job.ControllerName)
	}
	ReportCmdStateToServer(eventClient, job, cmd)
}

func (cmd *Command) Success(eventClient websocket.WebsocketClient, job *Job, msg string) {
	cmd.State = JobStateSucceeded
	cmd.Message = msg
	cmd.Finished = time.Now()
	ReportCmdStateToServer(eventClient, job, cmd)
}
