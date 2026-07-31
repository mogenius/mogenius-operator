package ai

import (
	"context"
	"fmt"
	"mogenius-operator/src/structs"
	"time"
)

// Cancel markers live in Valkey so a cancel request reaches the replica that
// is actually processing the task; the processing loop checks the marker at
// every LLM turn boundary. The value is the human-readable cancel reason.
// On the replica that runs the task the marker is not the trigger — CancelTask
// aborts the run's context directly via runCancels.
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
// Pending tasks flip to canceled immediately; in-progress tasks flip to
// cancelling right away (so the UI reflects the request instantly), the run's
// context is aborted, and the processing loop flips them to canceled once it
// has unwound.
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
		task.State = AI_TASK_STATE_CANCELED
		task.Error = canceledByMessage(user)
		if err := ai.createOrUpdateAiTask(task, taskID); err != nil {
			return nil, fmt.Errorf("failed to cancel pending task: %w", err)
		}
		ai.notifyTaskChanged(task)
		ai.logger.Info("AI task canceled while pending", "taskID", taskID, "canceledBy", user.Email)
		return task, nil

	case AI_TASK_STATE_IN_PROGRESS:
		// The marker must exist BEFORE the context is canceled: the loop
		// distinguishes a user cancel from an operator shutdown by its
		// presence, and it is the only signal that reaches a run on another
		// replica.
		if err := ai.valkeyClient.Set(canceledByMessage(user), aiTaskCancelTTL, taskCancelKey(taskID)); err != nil {
			return nil, fmt.Errorf("failed to store cancel request: %w", err)
		}
		task.State = AI_TASK_STATE_CANCELLING
		task.Error = canceledByMessage(user)
		if err := ai.createOrUpdateAiTask(task, taskID); err != nil {
			return nil, fmt.Errorf("failed to mark task as cancelling: %w", err)
		}
		ai.notifyTaskChanged(task)
		ai.cancelLocalRun(taskID)
		ai.logger.Info("AI task cancel requested", "taskID", taskID, "canceledBy", user.Email)
		return task, nil

	case AI_TASK_STATE_CANCELLING:
		// Repeated cancel clicks are a no-op; the run is already unwinding.
		return task, nil

	default:
		return nil, fmt.Errorf("task %s is in state %q; only pending or in-progress tasks can be canceled", taskID, task.State)
	}
}

// registerRunCancel makes a run abortable from CancelTask while it executes on
// this replica.
func (ai *aiManager) registerRunCancel(taskID string, cancel context.CancelFunc) {
	ai.runCancelMu.Lock()
	defer ai.runCancelMu.Unlock()
	ai.runCancels[taskID] = cancel
}

func (ai *aiManager) unregisterRunCancel(taskID string) {
	ai.runCancelMu.Lock()
	defer ai.runCancelMu.Unlock()
	delete(ai.runCancels, taskID)
}

// cancelLocalRun aborts the run's context when the task executes on this
// replica; on any other replica it is a no-op and the Valkey marker takes
// over at the next turn boundary.
func (ai *aiManager) cancelLocalRun(taskID string) {
	ai.runCancelMu.Lock()
	cancel := ai.runCancels[taskID]
	ai.runCancelMu.Unlock()
	if cancel != nil {
		cancel()
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
