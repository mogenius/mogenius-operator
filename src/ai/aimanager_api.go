package ai

import (
	"fmt"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var cachedStatusTime time.Time
var cachedStatus AiManagerStatus
var cachedWorkspaceStatusTime map[string]time.Time = make(map[string]time.Time)
var cachedWorkspaceStatus map[string]AiManagerStatus = make(map[string]AiManagerStatus)
var AiCachedStatusLiveTime time.Duration = time.Minute * 1

func (ai *aiManager) UpdateTaskState(taskID string, newState AiTaskState) error {

	item, err := ai.valkeyClient.Get(taskID)
	if err != nil {
		return err
	}
	if item == "" {
		return fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
	}
	var task AiTask
	err = json.Unmarshal([]byte(item), &task)
	if err != nil {
		return err
	}
	task.State = newState
	err = ai.createOrUpdateAiTask(&task, taskID)
	if err != nil {
		return err
	}

	return nil
}

func (ai *aiManager) UpdateTaskReadState(taskID string, user *structs.User) error {
	if user.Email == "" {
		return fmt.Errorf("user cannot be nil")
	}

	item, err := ai.valkeyClient.Get(taskID)
	if err != nil {
		return err
	}
	if item == "" {
		return fmt.Errorf("no ai task with the specified id has been found: %s", taskID)
	}

	var task AiTask
	err = json.Unmarshal([]byte(item), &task)
	if err != nil {
		return err
	}
	if userSeesTaskForTheFirstTime(user, task.ReadByUsers) {
		task.ReadByUsers = append(task.ReadByUsers, ReadBy{
			User:   *user,
			ReadAt: time.Now(),
		})
		return ai.createOrUpdateAiTask(&task, taskID)

	}

	return nil
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
		case "helm", "argocd":
			ai.logger.Error("Retrieving AI Tasks for this workspace type will be possible in the future", "workspace", workspace, "type", workspaceResource.Type)
		default:
			ai.logger.Error("Retrieving AI Tasks for unknown workspace type is not possible", "workspace", workspace, "type", workspaceResource.Type)
		}
	}

	return tasks, nil
}

func (ai *aiManager) GetAllAiTasks() ([]AiTask, error) {
	key := ai.getValkeyKey("*", "*", "*")

	items, err := ai.valkeyClient.List(1000, key)
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
		skipDuplicate := false
		for _, existingTask := range tasks {
			if existingTask.ID == task.ID {
				skipDuplicate = true
				break
			}
		}
		if skipDuplicate {
			continue
		}
		tasks = append(tasks, task)
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
		case "helm", "argocd":
			ai.logger.Error("Retrieving AI Tasks for this workspace type will be possible in the future", "workspace", workspace, "type", workspaceResource.Type)
		default:
			ai.logger.Error("Retrieving AI Tasks for unknown workspace type is not possible", "workspace", workspace, "type", workspaceResource.Type)
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
		if latestTask.Task == nil || task.CreatedAt > latestTask.Task.CreatedAt {
			latestTask.Task = &task
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
	maxToolCalls, _ := ai.getAiMaxToolCalls()
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
	var err error

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
						var totalDbEntriesForNs int
						var unprocessedDbEntriesForNs int
						var ignoredDbEntriesForNs int
						var numberOfUnreadTasksForNs int
						var err error
						totalDbEntriesForNs, unprocessedDbEntriesForNs, ignoredDbEntriesForNs, numberOfUnreadTasksForNs, err = ai.getDbStats(&workspaceResource.Id)
						if err != nil {
							ai.logger.Warn("Failed to get DB stats for workspace namespace", "workspace", workspace, "namespace", workspaceResource.Id, "error", err)
							continue
						}
						totalDbEntries += totalDbEntriesForNs
						unprocessedDbEntries += unprocessedDbEntriesForNs
						ignoredDbEntries += ignoredDbEntriesForNs
						numberOfUnreadTasks += numberOfUnreadTasksForNs
					case "helm", "argocd":
						ai.logger.Error("Retrieving AI Tasks for this workspace type will be possible in the future", "workspace", workspace, "type", workspaceResource.Type)
					default:
						ai.logger.Error("Retrieving AI Tasks for unknown workspace type is not possible", "workspace", workspace, "type", workspaceResource.Type)
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
		MaxToolCalls:                maxToolCalls,
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
		DB_AI_BUCKET_TASKS_LATEST + ":*",
	}
	ai.resetCache()
	err := ai.valkeyClient.DeleteMultiple(prefixes...)
	return err
}

func userSeesTaskForTheFirstTime(readBy *structs.User, users []ReadBy) bool {
	if readBy == nil {
		return false
	}
	for _, u := range users {
		if u.User.Email == readBy.Email {
			return false
		}
	}
	return true
}

// ResolveWorkspaceContext looks up the WorkspaceSpec and the user's GrantSpec
// for a given workspace name and user email. It returns nil for values that
// could not be resolved (e.g. workspace not found, no grant for the user).
func (ai *aiManager) ResolveWorkspaceContext(userEmail string, workspaceName string) (*v1alpha1.WorkspaceSpec, *v1alpha1.GrantSpec) {
	ownNamespace, err := ai.config.TryGet("MO_OWN_NAMESPACE")
	if err != nil {
		ai.logger.Warn("ResolveWorkspaceContext: failed to get own namespace", "error", err)
		return nil, nil
	}

	// Resolve workspace
	var workspaceSpec *v1alpha1.WorkspaceSpec
	workspace, err := store.GetWorkspace(ownNamespace, workspaceName)
	if err != nil {
		ai.logger.Warn("ResolveWorkspaceContext: failed to get workspace", "workspace", workspaceName, "error", err)
	} else if workspace != nil {
		workspaceSpec = &workspace.Spec
	}

	// Resolve user's grant for this workspace
	var grantSpec *v1alpha1.GrantSpec
	if userEmail != "" {
		// Find user CRD by email to get the user's metadata.name (used as grantee)
		userResources := store.GetResourceByKindAndNamespace(ai.valkeyClient, "mogenius.com/v1alpha1", "User", ownNamespace, ai.logger)
		var userName string
		for _, u := range userResources {
			email, _, _ := unstructured.NestedString(u.Object, "spec", "email")
			if email == userEmail {
				userName = u.GetName()
				break
			}
		}

		if userName != "" {
			// Find grant for this user and workspace
			grantResources := store.GetResourceByKindAndNamespace(ai.valkeyClient, "mogenius.com/v1alpha1", "Grant", ownNamespace, ai.logger)
			for _, g := range grantResources {
				grantee, _, _ := unstructured.NestedString(g.Object, "spec", "grantee")
				targetName, _, _ := unstructured.NestedString(g.Object, "spec", "targetName")
				if grantee == userName && targetName == workspaceName {
					role, _, _ := unstructured.NestedString(g.Object, "spec", "role")
					targetType, _, _ := unstructured.NestedString(g.Object, "spec", "targetType")
					grantSpec = &v1alpha1.GrantSpec{
						Grantee:    grantee,
						TargetType: targetType,
						TargetName: targetName,
						Role:       role,
					}
					break
				}
			}
		}
	}

	return workspaceSpec, grantSpec
}
