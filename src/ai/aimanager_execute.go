package ai

import (
	"fmt"
	"mogenius-operator/src/structs"
	"time"
)

// ApproveTask records the user's approval, signals the waiting agent goroutine
// to proceed, and returns. The actual tool execution happens in the agent's own
// tool loop; the task state is updated to EXECUTED by ApproveTask since the
// call is handed off to the model's execution path.
func (ai *aiManager) ApproveTask(taskID string, user structs.User) (*AiTask, error) {
	task, err := ai.getTaskByKey(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
	}
	if task.State != AI_TASK_STATE_PROPOSED {
		return nil, fmt.Errorf("task %s is in state %q; only proposed tasks can be approved", taskID, task.State)
	}
	if user.Email == "" {
		return nil, fmt.Errorf("approving a task requires an attributable user")
	}

	task.State = AI_TASK_STATE_EXECUTED
	task.Approval = &ApprovalRecord{User: user, At: time.Now()}
	if err := ai.createOrUpdateAiTask(task, taskID); err != nil {
		return nil, fmt.Errorf("failed to persist approval: %w", err)
	}
	ai.notifyTaskChanged(task)
	ai.signalApproval(taskID, user, nil)

	return task, nil
}

// RejectTask marks a proposed task as rejected by the given user.
func (ai *aiManager) RejectTask(taskID string, user structs.User, reason string) (*AiTask, error) {
	task, err := ai.getTaskByKey(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
	}
	if task.State != AI_TASK_STATE_PROPOSED {
		return nil, fmt.Errorf("task %s is in state %q; only proposed tasks can be rejected", taskID, task.State)
	}
	if user.Email == "" {
		return nil, fmt.Errorf("rejecting a task requires an attributable user")
	}

	task.State = AI_TASK_STATE_REJECTED
	task.Approval = &ApprovalRecord{User: user, At: time.Now(), Rejected: true, Reason: reason}
	if err := ai.createOrUpdateAiTask(task, taskID); err != nil {
		return nil, fmt.Errorf("failed to persist rejection: %w", err)
	}
	ai.notifyTaskChanged(task)

	rejectionMsg := fmt.Sprintf("rejected by %s", user.Email)
	if reason != "" {
		rejectionMsg += ": " + reason
	}
	ai.signalApproval(taskID, user, fmt.Errorf("%s", rejectionMsg))

	return task, nil
}

// signalApproval wakes the agent run blocking on the given task's approval.
// If no run is waiting (e.g. cross-replica or already timed out) the call is a no-op.
func (ai *aiManager) signalApproval(taskID string, approver structs.User, err error) {
	ai.pendingApprovalsMu.Lock()
	ch, ok := ai.pendingApprovals[taskID]
	ai.pendingApprovalsMu.Unlock()
	if ok {
		ch <- approvalResult{approver: approver, err: err}
	}
}


// notifyTaskChanged pushes the task's new state to the UI and refreshes the
// cached status counters.
func (ai *aiManager) notifyTaskChanged(task *AiTask) {
	ai.resetCache()
	ai.sendAiEvent(&AiTaskLatest{
		Task:   task,
		Status: ai.GetStatus(nil),
	})
}
