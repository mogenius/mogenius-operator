package ai

import (
	"encoding/json"
	"fmt"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"strings"
	"time"
)

func (ai *aiManager) UpdateTaskState(taskID string, newState AiTaskState) error {
	keys, err := ai.valkeyClient.Keys(DB_AI_BUCKET_TASKS + ":*")
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

func (ai *aiManager) UpdateTaskReadState(taskID string, user *structs.User) error {
	if user.Email == "" {
		return fmt.Errorf("user cannot be nil")
	}

	keys, err := ai.valkeyClient.Keys(DB_AI_BUCKET_TASKS + ":*")
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
			// toggle
			if task.ReadByUser == nil {
				task.ReadByUser = &ReadBy{
					User:   *user,
					ReadAt: time.Now(),
				}
				return ai.createOrUpdateAiTask(&task, key)
			} else {
				task.ReadByUser = nil
				return ai.createOrUpdateAiTask(&task, key)
			}
		}
	}

	return fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
}

func (ai *aiManager) GetAiTasksForResource(resourceReq utils.WorkloadSingleRequest) ([]AiTask, error) {
	tasks := []AiTask{}
	valkeyPath := ai.getValkeyKey(resourceReq.Kind, resourceReq.Namespace, resourceReq.ResourceName, "*")
	keys, err := ai.valkeyClient.Keys(valkeyPath)
	if err != nil {
		return tasks, err
	}

	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			return tasks, err
		}
		var task AiTask
		err = json.Unmarshal([]byte(item), &task)
		if err != nil {
			return tasks, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
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

func (ai *aiManager) GetLatestTask(workspace string) (*AiTaskLatest, error) {
	tasks, err := ai.GetAiTasksForWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	latestTask := &AiTaskLatest{}

	for _, task := range tasks {
		if task.ReadByUser == nil {
			latestTask.NumberOfUnreadTasks++
			if latestTask.Task == nil || task.CreatedAt > latestTask.Task.CreatedAt {
				latestTask.Task = &task
			}
		}
	}
	return latestTask, err
}

func (ai *aiManager) getAiTasksForNamespace(namespace string) ([]AiTask, error) {

	key := ai.getValkeyKey("*", namespace, "*", "*")

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

func (ai *aiManager) GetStatus() AiManagerStatus {
	limit, _ := ai.getDailyTokenLimit()
	model, _ := ai.getAiModel()
	apiUrl, _ := ai.getBaseUrl()
	tokensUsed, dbEntries, _ := ai.getTodayTokenUsage()

	if tokensUsed > limit {
		ai.error = fmt.Sprintf("Daily AI token limit exceeded (%d tokens used of %d).", tokensUsed, limit)
	} else {
		if strings.HasPrefix(ai.error, "Daily AI token limit") {
			ai.error = ""
		}
	}

	return AiManagerStatus{
		TokenLimit:                  limit,
		TokensUsed:                  tokensUsed,
		ApiUrl:                      apiUrl,
		Model:                       model,
		IsAiPromptConfigInitialized: ai.isAiPromptConfigInitialized(),
		IsAiModelConfigInitialized:  ai.isAiModelConfigInitialized(),
		DbEntries:                   dbEntries,
		Error:                       ai.error,
		Warning:                     ai.warning,
	}
}

func (ai *aiManager) ResetDailyTokenLimit() error {
	return ai.resetTodayTokenUsage()
}
