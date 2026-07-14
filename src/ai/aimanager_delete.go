package ai

import (
	"fmt"
	"mogenius-operator/src/structs"
)

// DeleteTask permanently removes a single task. Running tasks must be
// canceled first: the processing loop persists its task struct when it
// finishes and would silently resurrect the deleted key.
func (ai *aiManager) DeleteTask(taskID string, user structs.User) (*AiTask, error) {
	task, err := ai.getTaskByKey(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
	}

	switch task.State {
	case AI_TASK_STATE_IN_PROGRESS, AI_TASK_STATE_EXECUTING:
		return nil, fmt.Errorf("task %s is in state %q; cancel it before deleting", taskID, task.State)
	}

	if err := ai.valkeyClient.DeleteSingle(taskID); err != nil {
		return nil, fmt.Errorf("failed to delete task %s: %w", taskID, err)
	}
	ai.resetCache()
	ai.sendAiDeleteEvent(taskID)
	ai.logger.Info("AI task deleted", "taskID", taskID, "deletedBy", user.Email)
	return task, nil
}
