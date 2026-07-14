package ai

import (
	"fmt"
	"mogenius-operator/src/structs"
	"time"
)

// Cancel markers live in Valkey so a cancel request reaches the replica that
// is actually processing the task; the processing loop checks the marker at
// every LLM turn boundary. The value is the human-readable cancel reason.
const aiTaskCancelPrefix = "ai_task_cancel"
const aiTaskCancelTTL = time.Hour

func taskCancelKey(taskID string) string {
	return aiTaskCancelPrefix + ":" + taskID
}

func canceledByMessage(user structs.User) string {
	if user.Email == "" {
		return "canceled by user"
	}
	return "canceled by " + user.Email
}

// CancelTask aborts a queued or running report so it stops burning tokens.
// Pending tasks flip to ignored immediately; in-progress tasks get a cancel
// marker that the processing loop picks up at the next LLM turn, aborting the
// run and flipping the task to ignored (no new task state — canceled runs are
// just ignored reports with a cancel note).
func (ai *aiManager) CancelTask(taskID string, user structs.User) (*AiTask, error) {
	task, err := ai.getTaskByKey(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
	}

	switch task.State {
	case AI_TASK_STATE_PENDING:
		task.State = AI_TASK_STATE_IGNORED
		task.Error = canceledByMessage(user)
		if err := ai.createOrUpdateAiTask(task, taskID); err != nil {
			return nil, fmt.Errorf("failed to cancel pending task: %w", err)
		}
		ai.notifyTaskChanged(task)
		ai.logger.Info("AI task canceled while pending", "taskID", taskID, "canceledBy", user.Email)
		return task, nil

	case AI_TASK_STATE_IN_PROGRESS:
		if err := ai.valkeyClient.Set(canceledByMessage(user), aiTaskCancelTTL, taskCancelKey(taskID)); err != nil {
			return nil, fmt.Errorf("failed to store cancel request: %w", err)
		}
		ai.logger.Info("AI task cancel requested", "taskID", taskID, "canceledBy", user.Email)
		// State flips to ignored once the processing loop hits the marker;
		// the UI gets the update via the regular task event.
		return task, nil

	default:
		return nil, fmt.Errorf("task %s is in state %q; only pending or in-progress tasks can be canceled", taskID, task.State)
	}
}

// taskCancelReason returns the cancel message when a cancel was requested for
// the task, or "" when none is pending.
func (ai *aiManager) taskCancelReason(taskID string) string {
	reason, err := ai.valkeyClient.Get(taskCancelKey(taskID))
	if err != nil {
		return ""
	}
	return reason
}

func (ai *aiManager) clearTaskCancelRequest(taskID string) {
	if err := ai.valkeyClient.DeleteSingle(taskCancelKey(taskID)); err != nil {
		ai.logger.Warn("Failed to clear task cancel marker", "taskID", taskID, "error", err)
	}
}
