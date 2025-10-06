package ai

import (
	"encoding/json"
	"fmt"
	"mogenius-operator/src/store"
)

func (ai *aiManager) UpdateTaskState(taskID string, newState AiTaskState) error {
	keys, err := ai.valkeyClient.Keys(DB_AI_BUCKET_TASKS)
	if err != nil {
		return err
	}

	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			return err
		}
		var task AiTask
		err = json.Unmarshal([]byte(item), &task)
		if err != nil {
			return err
		}
		if task.ID == taskID {
			task.State = newState
			err = ai.createOrUpdateAiTask(&task, key)
			if err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
}

func (ai *aiManager) GetAiTasksForWorkspace(workspace string) ([]AiTask, error) {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve own namespace: %v", err)
	}

	workspaceObject, err := store.GetWorkspace(ownNamespace, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %v", err)
	}
	if workspaceObject == nil {
		return nil, fmt.Errorf("workspace not found: %s", workspace)
	}

	var tasks []AiTask
	for _, workspaceResource := range workspaceObject.Spec.Resources {

		switch workspaceResource.Type {
		case "namespace":
			tasksForNamespace, err := ai.getAiTasksForNamespace(workspaceResource.Id)
			if err != nil {
				return nil, fmt.Errorf("failed to get AI tasks for namespace '%s': %v", workspaceResource.Id, err)
			}
			tasks = append(tasks, tasksForNamespace...)
		case "helm":
			ai.logger.Error("Retrieving AI Tasks for this workspace is not possible", "workspace", workspace, "type", workspaceResource.Type)
		case "argocd":
			ai.logger.Error("Retrieving AI Tasks for this workspace is not possible", "workspace", workspace, "type", workspaceResource.Type)
		default:
			ai.logger.Error("Retrieving AI Tasks for this workspace is not possible", "workspace", workspace, "type", workspaceResource.Type)
		}
	}

	return tasks, nil
}

func (ai *aiManager) getAiTasksForNamespace(namespace string) ([]AiTask, error) {

	key := getValkeyKey("*", namespace, "*", "*")

	items, err := ai.valkeyClient.List(100, key)
	if err != nil {
		return nil, err
	}

	var tasks []AiTask
	for _, item := range items {
		var task AiTask
		err = json.Unmarshal([]byte(item), &task)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}
