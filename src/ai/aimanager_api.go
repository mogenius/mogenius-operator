package ai

import (
	"encoding/json"
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"strings"
	"sync"
	"time"
)

var cachedStatusTime time.Time
var cachedStatus AiManagerStatus
var cachedWorkspaceStatusTime map[string]time.Time = make(map[string]time.Time)
var cachedWorkspaceStatus map[string]AiManagerStatus = make(map[string]AiManagerStatus)
var AiCachedStatusLiveTime time.Duration = time.Minute * 1

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
	valkeyPath := ai.getValkeyKey(resourceReq.Kind, resourceReq.Namespace, resourceReq.ResourceName)
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

func (ai *aiManager) GetAiLatestTasksForWorkspace(workspace string) ([]AiTask, error) {
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
			latestNamespaceTask, err := ai.getLatestNamespaceTask(workspaceResource.Id)
			if err != nil {
				ai.logger.Warn("No latest AI task found for namespace", "namespace", workspaceResource.Id)
				continue
			}
			if latestNamespaceTask != nil {
				tasks = append(tasks, *latestNamespaceTask)
			}
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

func (ai *aiManager) GetLatestTask(workspace *string) (*AiTaskLatest, error) {
	latestTask := &AiTaskLatest{}
	latestTask.Status = ai.GetStatus(workspace)

	if workspace == nil {
		task, err := ai.getLatestTask()
		if err != nil {
			return nil, err
		}

		latestTask.Task = task
		return latestTask, err
	}

	tasks, err := ai.GetAiLatestTasksForWorkspace(*workspace)
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		if task.ReadByUser == nil {
			if latestTask.Task == nil || task.CreatedAt > latestTask.Task.CreatedAt {
				latestTask.Task = &task
			}
		}
	}
	return latestTask, err
}

func (ai *aiManager) getAiTasksForNamespace(namespace string) ([]AiTask, error) {

	key := ai.getValkeyKey("*", namespace, "*")

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

func (ai *aiManager) getLatestTask() (*AiTask, error) {
	key := ai.getValkeyLatestTaskKey()
	item, err := ai.valkeyClient.Get(key)

	if err != nil {
		return nil, err
	}
	var task AiTask
	err = json.Unmarshal([]byte(item), &task)
	if err != nil {
		return nil, err
	}

	return &task, nil
}

func (ai *aiManager) getLatestNamespaceTask(namespace string) (*AiTask, error) {
	key := ai.getValkeyLatestNamespaceTaskKey(namespace)
	item, err := ai.valkeyClient.Get(key)

	if err != nil {
		return nil, err
	}
	var task AiTask
	err = json.Unmarshal([]byte(item), &task)
	if err != nil {
		return nil, err
	}

	return &task, nil
}

func (ai *aiManager) GetStatus(workspace *string) AiManagerStatus {
	mutex := sync.Mutex{}
	if workspace == nil {
		if cachedStatusTime.Add(AiCachedStatusLiveTime).After(time.Now()) {
			return cachedStatus
		}
	} else {
		if lastCachedTime, exists := cachedWorkspaceStatusTime[*workspace]; exists {
			if lastCachedTime.Add(AiCachedStatusLiveTime).After(time.Now()) {
				if status, exists := cachedWorkspaceStatus[*workspace]; exists {
					return status
				}
			}
		}
	}

	sdk, _ := ai.getSdkType()
	limit, _ := ai.getDailyTokenLimit()
	model, _ := ai.getAiModel()
	apiUrl, _ := ai.getBaseUrl()
	tokensUsed, todaysProcessedTasks, _ := ai.getTodayTokenUsage()

	if tokensUsed > limit {
		ai.error = fmt.Sprintf("Daily AI token limit exceeded (%d tokens used of %d).", tokensUsed, limit)
	} else {
		if strings.HasPrefix(ai.error, "Daily AI token limit") {
			ai.error = ""
		}
	}
	var totalDbEntries int = 0
	var unprocessedDbEntries int = 0
	var ignoredDbEntries int = 0
	var numberOfUnreadTasks int = 0
	var err error = nil
	if workspace == nil {
		totalDbEntries, unprocessedDbEntries, ignoredDbEntries, numberOfUnreadTasks, err = ai.getDbStats(nil)
	} else {
		var ownNamespace string
		ownNamespace, err = ai.config.TryGet("MO_OWN_NAMESPACE")
		if err == nil {
			var workspaceObject *v1alpha1.Workspace
			workspaceObject, err = store.GetWorkspace(ownNamespace, *workspace)
			if err == nil && workspaceObject != nil {
				for _, workspaceResource := range workspaceObject.Spec.Resources {

					switch workspaceResource.Type {
					case "namespace":
						var totalDbEntriesForNs int = 0
						var unprocessedDbEntriesForNs int = 0
						var ignoredDbEntriesForNs int = 0
						var numberOfUnreadTasksForNs int = 0
						err = nil
						totalDbEntriesForNs, unprocessedDbEntriesForNs, ignoredDbEntriesForNs, numberOfUnreadTasksForNs, err = ai.getDbStats(&workspaceResource.Id)
						if err != nil {
							ai.logger.Warn("Failed to get DB stats for workspace namespace", "workspace", workspace, "namespace", workspaceResource.Id, "error", err)
							continue
						}
						totalDbEntries += totalDbEntriesForNs
						unprocessedDbEntries += unprocessedDbEntriesForNs
						ignoredDbEntries += ignoredDbEntriesForNs
						numberOfUnreadTasks += numberOfUnreadTasksForNs
					case "helm":
						ai.logger.Warn("Retrieving AI Tasks for this workspace is not possible", "workspace", workspace, "type", workspaceResource.Type)
					case "argocd":
						ai.logger.Warn("Retrieving AI Tasks for this workspace is not possible", "workspace", workspace, "type", workspaceResource.Type)
					default:
						ai.logger.Warn("Retrieving AI Tasks for this workspace is not possible", "workspace", workspace, "type", workspaceResource.Type)
					}
				}
			}
		}
	}

	if err != nil {
		ai.error = fmt.Sprintf("Failed to get DB stats: %v", err)
	}

	// 0 oclock next day
	nextReset := time.Now().Add(24 * time.Hour)
	nextReset = time.Date(nextReset.Year(), nextReset.Month(), nextReset.Day(), 0, 0, 0, 0, nextReset.Location())

	status := AiManagerStatus{
		SdkType:                     sdk,
		TokenLimit:                  limit,
		TokensUsed:                  tokensUsed,
		ApiUrl:                      apiUrl,
		Model:                       model,
		IsAiPromptConfigInitialized: ai.isAiPromptConfigInitialized(),
		IsAiModelConfigInitialized:  ai.isAiModelConfigInitialized(),
		TodaysProcessedTasks:        todaysProcessedTasks,
		TotalDbEntries:              totalDbEntries,
		UnprocessedDbEntries:        unprocessedDbEntries,
		IgnoredDbEntries:            ignoredDbEntries,
		Error:                       ai.error,
		Warning:                     ai.warning,
		NumberOfUnreadTasks:         numberOfUnreadTasks,
		NextTokenResetTime:          nextReset.Format(time.RFC3339),
	}
	mutex.Lock()
	if workspace != nil {
		cachedWorkspaceStatusTime[*workspace] = time.Now()
		cachedWorkspaceStatus[*workspace] = status
	} else {
		cachedStatusTime = time.Now()
		cachedStatus = status
	}
	mutex.Unlock()
	return status
}

func (ai *aiManager) ResetDailyTokenLimit() error {
	return ai.resetTodayTokenUsage()
}

func (ai *aiManager) DeleteAllAiData() error {
	prefixes := []string{
		DB_AI_BUCKET_TASKS + ":*",
		DB_AI_BUCKET_TOKENS + ":*",
	}
	ai.resetCache()
	err := ai.valkeyClient.DeleteMultiple(prefixes...)
	return err
}
