package structs

import (
	"fmt"
	"mogenius-k8s-manager/utils"
	"os/exec"
	"sync"
	"time"

	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
)

type Command struct {
	Id       string       `json:"id"`
	Title    string       `json:"title"`
	Message  string       `json:"message,omitempty"`
	State    JobStateEnum `json:"state"`
	Started  time.Time    `json:"started"`
	Finished time.Time    `json:"finished"`
}

func CreateCommand(title string, job *Job) *Command {
	cmd := &Command{
		Id:      punqUtils.NanoId(),
		Title:   title,
		Message: "",
		State:   JobStatePending,
		Started: time.Now(),
	}
	job.AddCmd(cmd)
	return cmd
}

// XXX NOT USED ANYMORE?
func CreateShellCommand(title string, job *Job, shellCmd string, wg *sync.WaitGroup) {
	wg.Add(1)
	cmd := CreateCommand(title, job)
	go func() {
		defer wg.Done()
		cmd.Start(job, title)

		output, err := exec.Command("sh", "-c", shellCmd).Output()
		log.Info(string(shellCmd))
		log.Info(string(output))

		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			errorMsg := string(exitErr.Stderr)
			log.Error(shellCmd)
			log.Errorf("%d: %s", exitCode, errorMsg)
			cmd.Fail(job, fmt.Sprintf("'%s' ERROR: %s", title, errorMsg))
		} else if err != nil {
			log.Errorf("exec.Command: %s", err.Error())
		} else {
			cmd.Success(job, title)
		}
	}()
}

func CreateShellCommandGoRoutine(title string, shellCmd string, successFunc func(), failFunc func(output string, err error)) {
	go func() {
		output, err := exec.Command("sh", "-c", shellCmd).Output()
		log.Info(string(shellCmd))
		log.Info(string(output))
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			errorMsg := string(exitErr.Stderr)
			log.Error(shellCmd)
			log.Errorf("%d: %s", exitCode, errorMsg)
			if failFunc != nil {
				failFunc(string(output), exitErr)
			}
		} else if err != nil {
			log.Errorf("exec.Command: %s", err.Error())
			if failFunc != nil {
				failFunc(string(output), err)
			}
		} else {
			log.Infof("SUCCESS: %s", shellCmd)
			if successFunc != nil {
				successFunc()
			}
		}
	}()
}

func (cmd *Command) Start(job *Job, msg string) {
	cmd.State = JobStateStarted
	cmd.Message = msg
	ReportCmdStateToServer(job, cmd)
}

func (cmd *Command) Fail(job *Job, error string) {
	cmd.State = JobStateFailed
	cmd.Message = error
	cmd.Finished = time.Now()
	if utils.CONFIG.Misc.Debug {
		log.Errorf("Command '%s' failed: %s", cmd.Title, error)
	}
	ReportCmdStateToServer(job, cmd)
}

func (cmd *Command) Success(job *Job, msg string) {
	cmd.State = JobStateSucceeded
	cmd.Message = msg
	cmd.Finished = time.Now()
	ReportCmdStateToServer(job, cmd)
}
