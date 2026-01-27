package ai

import (
	"context"
	"fmt"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/websocket"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/yaml"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/valkey-io/valkey-go"

	"github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"

	"github.com/ollama/ollama/api"
	ollama "github.com/ollama/ollama/api"
)

const (
	DB_AI_BUCKET_TASKS              = "ai_tasks"
	DB_AI_BUCKET_TASKS_LATEST       = "ai_tasks_latest"
	DB_AI_BUCKET_TOKENS             = "ai_tokens"
	DB_AI_LATEST_TASK_KEY           = "latest-task"
	DB_AI_LATEST_NAMESPACE_TASK_KEY = "latest-namespace-task"
)

var ValkeyAiTTL = time.Hour * 24 * 7 // 7 days

type AiTaskState string
type AiTask struct {
	ID                  string                       `json:"id"`
	Prompt              string                       `json:"prompt"`
	Response            *AiResponse                  `json:"response"`
	State               AiTaskState                  `json:"state"` // pending, in-progress, completed, failed, ignored
	Controller          *utils.WorkloadSingleRequest `json:"controller,omitempty"`
	TokensUsed          int64                        `json:"tokensUsed"`
	Model               string                       `json:"model"`
	TimeUsedInMs        int                          `json:"timeUsedInMs"`
	CreatedAt           int64                        `json:"createdAt"`
	UpdatedAt           int64                        `json:"updatedAt"`
	ReferencingResource utils.WorkloadSingleRequest  `json:"referencingResource"` // the resource that triggered this task
	TriggeredBy         AiFilter                     `json:"triggeredBy"`         // e.g., "Failed Pods" filter
	ReadByUsers         []ReadBy                     `json:"readByUsers"`
	Error               string                       `json:"error"`
}

type AiTaskLatest struct {
	Task   *AiTask         `json:"task,omitempty"`
	Status AiManagerStatus `json:"status"`
}

type ReadBy struct {
	User   structs.User `json:"user"`
	ReadAt time.Time    `json:"readAt"`
}

// state enums
const (
	AI_TASK_STATE_PENDING     AiTaskState = "pending"
	AI_TASK_STATE_IN_PROGRESS AiTaskState = "in-progress"
	AI_TASK_STATE_COMPLETED   AiTaskState = "completed"
	AI_TASK_STATE_FAILED      AiTaskState = "failed"
	AI_TASK_STATE_IGNORED     AiTaskState = "ignored"
	AI_TASK_STATE_SOLVED      AiTaskState = "solved"
)

type AiFilter struct {
	Id          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Kind        string            `json:"kind"`
	Contains    map[string]string `json:"contains"` // {"Running": "status.phase"}, {"ImagePullBackOff": "status.phase.ContainerStatuses.state.waiting.reason"}
	Excludes    map[string]string `json:"excludes"` // {"Succeeded": "status.phase"}, {"Completed": "status.phase"}
	Prompt      string            `json:"prompt"`
	For         *time.Duration    `json:"for,omitempty"` // optional duration for which the condition should be met
}

type AiPromptConfig struct {
	Id           string     `json:"id"`
	Name         string     `json:"name"`
	SystemPrompt string     `json:"systemPrompt"`
	Filters      []AiFilter `json:"filters"`
}

type AiManagerStatus struct {
	SdkType                     AiSdkType `json:"sdkType"`
	TokenLimit                  int64     `json:"tokenLimit"`
	TokensUsed                  int64     `json:"tokensUsed"`
	Model                       string    `json:"model"`
	ApiUrl                      string    `json:"apiUrl"`
	IsAiPromptConfigInitialized bool      `json:"isAiPromptConfigInitialized"`
	IsAiModelConfigInitialized  bool      `json:"isAiModelConfigInitialized"`
	TodaysProcessedTasks        int       `json:"todaysProcessedTasks"`
	TotalDbEntries              int       `json:"totalDbEntries"`
	UnprocessedDbEntries        int       `json:"unprocessedDbEntries"`
	IgnoredDbEntries            int       `json:"ignoredDbEntries"`
	NumberOfUnreadTasks         int       `json:"numberOfUnreadTasks,omitempty"`
	Error                       string    `json:"error,omitempty"`
	Warning                     string    `json:"warning,omitempty"`
	NextTokenResetTime          string    `json:"nextTokenResetTime,omitempty"`
}

type AiResponse struct {
	ErrorMessage string   `json:"errorMessage"`
	Analysis     Analysis `json:"analysis"`
}

type Analysis struct {
	ProblemDescription  string                        `json:"problemDescription"`
	PossibleCauses      []string                      `json:"possibleCauses"`
	ProposedSolutions   []Solution                    `json:"proposedSolutions"`
	AdditionalInfo      string                        `json:"additionalInformation"`
	NeedsFollowUp       bool                          `json:"needsFollowUp"`
	FollowUpResources   []utils.WorkloadSingleRequest `json:"followUpResources"`
	CurrentResourceYaml string                        `json:"currentResourceYaml"`
	TargetResourceYaml  string                        `json:"targetResourceYaml"`
	TargetResource      utils.WorkloadSingleRequest   `json:"targetResource,omitempty"`
	ProposedOperation   string                        `json:"proposedOperation,omitempty"` // UpdateResource', 'DeleteResource', 'CreateResource', 'Other'
}

type Solution struct {
	SolutionDescription string   `json:"solutionDescription"`
	Steps               []string `json:"steps"`
}

type UsedToken struct {
	Timestamp    time.Time `json:"timestamp"`
	TokensUsed   int64     `json:"tokensUsed"`
	IsIgnored    bool      `json:"isIgnored"`
	Key          string    `json:"key"`
	Model        string    `json:"model"`
	TimeUsedInMs int       `json:"timeUsedInMs"`
}

type ModelsRequest struct {
	Sdk    string  `json:"SDK,omitempty"`
	ApiKey *string `json:"API_KEY,omitempty"`
	ApiUrl string  `json:"API_URL,omitempty"`
}
type AiManager interface {
	ProcessObject(obj *unstructured.Unstructured, eventType string, resource utils.ResourceDescriptor) // eventType can be "add", "update", "delete"
	Run()
	UpdateTaskState(taskID string, newState AiTaskState) error
	UpdateTaskReadState(taskID string, user *structs.User) error
	GetAllAiTasks() ([]AiTask, error)
	GetAiTasksForWorkspace(workspace string) ([]AiTask, error)
	GetAiTasksForResource(resourceReq utils.WorkloadSingleRequest) ([]AiTask, error)
	GetLatestTask(workspace *string) (*AiTaskLatest, error)
	InjectAiPromptConfig(prompt AiPromptConfig)
	GetStatus(workspace *string) AiManagerStatus
	ResetDailyTokenLimit() error
	DeleteAllAiData() error
	GetAvailableModels(request *ModelsRequest) ([]string, error)
}

type SecretGetter func(namespace, name string) (*coreV1.Secret, error)

type aiManager struct {
	logger            *slog.Logger
	valkeyClient      valkeyclient.ValkeyClient
	config            cfg.ConfigModule
	aiPromptConfig    *AiPromptConfig
	ownerCacheService store.OwnerCacheService
	eventClient       websocket.WebsocketClient
	secretGetter      SecretGetter
	error             string
	warning           string
	pendingTasks      map[string]AiTask
	pendingTasksLock  *sync.RWMutex
}

func NewAiManager(logger *slog.Logger, valkeyClient valkeyclient.ValkeyClient, config cfg.ConfigModule, ownerCacheService store.OwnerCacheService, eventClient websocket.WebsocketClient, secretGetter SecretGetter) AiManager {
	self := &aiManager{}

	self.logger = logger
	self.valkeyClient = valkeyClient
	self.config = config
	self.ownerCacheService = ownerCacheService
	self.eventClient = eventClient
	self.secretGetter = secretGetter
	self.pendingTasks = make(map[string]AiTask)
	self.pendingTasksLock = &sync.RWMutex{}

	return self
}

func (ai *aiManager) ProcessObject(obj *unstructured.Unstructured, eventType string, resource utils.ResourceDescriptor) {
	if obj == nil {
		return
	}

	if eventType == "delete" {
		// On delete, we try to remove any existing AI tasks for this object
		key := ai.getValkeyKey(obj.GetKind(), obj.GetNamespace(), obj.GetName())
		err := ai.valkeyClient.DeleteMultiple(key)
		if err != nil {
			ai.logger.Error("Error deleting AI tasks for deleted object", "objectKind", obj.GetKind(), "objectName", obj.GetName(), "objectNamespace", obj.GetNamespace(), "error", err)
		}
		return
	}

	initialized := ai.isAiPromptConfigInitialized()
	if !initialized {
		return
	}

	filters := ai.getAiFilters()

	for _, filter := range filters {
		if obj.GetKind() == filter.Kind {
			// check contains conditions
			matches, err := filterMatchesForObject(filter, obj)
			if err != nil {
				ai.logger.Error("Error checking AI filter match for object", "filter", filter.Name, "objectKind", obj.GetKind(), "objectName", obj.GetName(), "objectNamespace", obj.GetNamespace(), "error", err)
				continue
			}

			if matches {
				timestamp := time.Now().Unix()
				key := ai.getValkeyKey(obj.GetKind(), obj.GetNamespace(), obj.GetName())
				// create AI task
				task := &AiTask{
					ID:        key,
					Prompt:    buildUserPrompt(filter.Prompt, obj),
					State:     AI_TASK_STATE_PENDING,
					CreatedAt: timestamp,
					UpdatedAt: timestamp,
					ReferencingResource: utils.WorkloadSingleRequest{
						ResourceDescriptor: resource,
						Namespace:          obj.GetNamespace(),
						ResourceName:       obj.GetName(),
					},
					TriggeredBy: filter,
					Error:       "",
				}

				shouldCreate, err := ai.shouldCreateNewTask(key)
				if err != nil {
					ai.logger.Error("Error checking if should create new AI task", "error", err)
					continue
				}

				if shouldCreate {

					if task.TriggeredBy.For != nil {
						ai.pendingTasksLock.Lock()
						// store pending task to check later
						ai.pendingTasks[key] = *task
						ai.logger.Info("AI task pending due to 'For' duration not yet met", "key", key, "filter", filter.Name)
						ai.pendingTasksLock.Unlock()
						continue
					}
					ai.insertNewAiTask(task, obj, eventType, key)
				}
			}
		}
	}
}

func (ai *aiManager) insertNewAiTask(task *AiTask, obj *unstructured.Unstructured, eventType string, key string) {
	controller := ai.ownerCacheService.OwnerFromReference(obj.GetNamespace(), obj.GetOwnerReferences())
	task.Controller = controller
	if controller != nil {
		ctrlOb, err := store.GetResource(ai.valkeyClient, controller.ResourceDescriptor.ApiVersion, controller.ResourceDescriptor.Kind, controller.Namespace, controller.ResourceName, ai.logger)
		if err != nil {
			ai.logger.Error("Error fetching controller object for AI task", "controllerKind", controller.ResourceDescriptor.Kind, "controllerName", controller.ResourceName, "controllerNamespace", controller.Namespace, "error", err)
		} else {
			if ctrlOb != nil {
				controllerYaml, err := store.GetYamlFromUnstructuredResource(ctrlOb)
				if err != nil {
					ai.logger.Error("Error generating controller YAML for AI task prompt", "controllerKind", controller.ResourceDescriptor.Kind, "controllerName", controller.ResourceName, "controllerNamespace", controller.Namespace, "error", err)
				}
				task.Prompt += "\n\nThe controller resource YAML is as follows:\n" + controllerYaml
			}
		}
	}
	err := ai.createOrUpdateAiTask(task, key)
	if err != nil {
		ai.logger.Error("Error creating AI task", "error", err)
	} else {
		ai.logger.Info("AI task created", "taskID", task.ID, "event", eventType, "objectKind", obj.GetKind(), "objectName", obj.GetName(), "objectNamespace", obj.GetNamespace(), "filter", task.TriggeredBy.Name)
	}
}

func filterMatchesForObject(filter AiFilter, obj *unstructured.Unstructured) (bool, error) {
	matched := false
	for path, expectedValue := range filter.Contains {
		value, found, err := getNestedStringWithJSONPath(obj, path, expectedValue)
		if err != nil {
			return false, fmt.Errorf("Error checking AI filter contains: expectedValue=%s, error=%v", expectedValue, err)
		}

		if !found {
			continue
		}

		// For array results (comma-separated), check if expectedValue is in any of the values
		values := strings.Split(value, ", ")
		for _, v := range values {
			if strings.TrimSpace(v) == expectedValue {
				matched = true
				break
			}
		}

		if matched {
			break
		}
	}

	// no need to check excludes if not matched
	if !matched {
		return false, nil
	}

	// check excludes conditions
	for path, expectedValue := range filter.Excludes {
		value, found, err := getNestedStringWithJSONPath(obj, path, expectedValue)
		if err != nil {
			return false, fmt.Errorf("Error checking AI filter excludes: expectedValue=%s, error=%v", expectedValue, err)
		}

		if !found {
			continue
		}

		// For array results (comma-separated), check if expectedValue is in any of the values
		values := strings.Split(value, ", ")
		for _, v := range values {
			if strings.TrimSpace(v) == expectedValue {
				return false, nil
			}
		}
	}

	return true, nil
}

// BACKGROUND PROCESSING
func (ai *aiManager) Run() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			ctx := context.Background()

			initialized := ai.isAiPromptConfigInitialized()
			if !initialized {
				continue
			}

			modelConfigInitialized := ai.isAiModelConfigInitialized()
			if !modelConfigInitialized {
				continue
			}

			if ai.isTokenLimitExceeded() {
				continue
			}

			ai.processPendingTasks()

			ai.error = ""
			ai.processAiTaskQueue(ctx)
		}
	}()
}

func (ai *aiManager) processPendingTasks() {
	ai.pendingTasksLock.Lock()
	defer ai.pendingTasksLock.Unlock()

	stillPending := make(map[string]AiTask)
	for key, task := range ai.pendingTasks {
		shouldCreate, err := ai.shouldCreateNewTask(key)
		if err != nil {
			ai.logger.Error("Error checking if should create new AI task", "error", err)
			continue
		}
		if !shouldCreate {
			continue
		}

		now := time.Now()
		if task.TriggeredBy.For == nil {
			// should never happen due to earlier checks, but just in case
			ai.logger.Error("Pending AI task filter has nil 'For' duration", "key", key, "filter", task.TriggeredBy.Name)
			continue
		}

		createdTime := time.Unix(task.CreatedAt, 0)
		if createdTime.Add(*task.TriggeredBy.For).Before(now) {
			// The duration has elapsed since CreatedAt
			reloadedObject, err := store.GetResource(ai.valkeyClient,
				task.ReferencingResource.ApiVersion,
				task.ReferencingResource.Kind,
				task.ReferencingResource.Namespace,
				task.ReferencingResource.ResourceName,
				ai.logger)
			if err != nil {
				ai.logger.Error("Error reloading object for pending AI task", "key", key, "filter", task.TriggeredBy.Name, "error", err)
				continue
			}
			if reloadedObject == nil {
				ai.logger.Info("Referenced object for pending AI task no longer exists, skipping task creation", "key", key, "filter", task.TriggeredBy.Name)
				continue
			}
			matched, err := filterMatchesForObject(task.TriggeredBy, reloadedObject)
			if err != nil {
				ai.logger.Error("Error checking AI filter match for reloaded object", "filter", task.TriggeredBy.Name, "objectKind", reloadedObject.GetKind(), "objectName", reloadedObject.GetName(), "objectNamespace", reloadedObject.GetNamespace(), "error", err)
				continue
			}
			if matched {
				// create AI task
				ai.insertNewAiTask(&task, reloadedObject, "delayed", key)
			}
		} else {
			stillPending[key] = task
		}
	}
	ai.pendingTasks = stillPending
}

func (ai *aiManager) isTokenLimitExceeded() bool {
	tokenLimit, err := ai.getDailyTokenLimit()
	if err != nil {
		ai.logger.Error("Error getting daily token limit", "error", err)
		ai.error = err.Error()
		return true
	}

	tokensUsed, _, err := ai.getTodayTokenUsage()
	if err != nil {
		ai.logger.Error("Error getting today's token usage", "error", err)
		ai.error = err.Error()
		return true
	}

	if tokensUsed >= tokenLimit {
		ai.logger.Warn("Daily AI token limit reached, skipping AI task processing", "tokensUsed", tokensUsed, "dailyLimit", tokenLimit)
		ai.error = fmt.Errorf("Daily AI token limit reached (%d tokens used of %d). Increase limit or wait 24 hours.", tokensUsed, tokenLimit).Error()
		return true
	} else if tokensUsed >= int64(float64(tokenLimit)*0.8) {
		// warn at 80%
		ai.logger.Warn("Approaching daily AI token limit", "tokensUsed", tokensUsed, "dailyLimit", tokenLimit)
		ai.warning = fmt.Sprintf("Approaching daily AI token limit (%d tokens used of %d).", tokensUsed, tokenLimit)
	} else {
		ai.warning = ""
	}
	return false
}

func (ai *aiManager) getTodayTokenUsage() (todaysTokens int64, todaysProcessedTasks int, err error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	cursor := uint64(0)
	pattern := DB_AI_BUCKET_TOKENS + ":*"
	ctx := context.Background()

	for {
		// Build and execute SCAN command
		scanCmd := ai.valkeyClient.GetValkeyClient().B().Scan().Cursor(cursor).Match(pattern).Count(100).Build()
		scanResult, err := ai.valkeyClient.GetValkeyClient().Do(ctx, scanCmd).AsScanEntry()
		if err != nil {
			return 0, 0, err
		}

		if len(scanResult.Elements) > 0 {
			// Build all GET commands for this batch
			cmds := make([]valkey.Completed, len(scanResult.Elements))
			for i, key := range scanResult.Elements {
				cmds[i] = ai.valkeyClient.GetValkeyClient().B().Get().Key(key).Build()
			}

			// Execute all GETs in a single round trip
			results := ai.valkeyClient.GetValkeyClient().DoMulti(ctx, cmds...)

			// Process results
			for _, result := range results {
				item, err := result.ToString()
				if err != nil {
					// Key might have been deleted or expired, skip it
					continue
				}

				var tokenEntry UsedToken
				if err := json.Unmarshal([]byte(item), &tokenEntry); err != nil {
					// Log error but continue processing
					continue
				}

				if tokenEntry.Timestamp.Unix() >= startOfDay && !tokenEntry.IsIgnored {
					todaysTokens += tokenEntry.TokensUsed
					todaysProcessedTasks++
				}
			}
		}

		cursor = scanResult.Cursor
		if cursor == 0 {
			break // SCAN complete
		}
	}

	return todaysTokens, todaysProcessedTasks, nil
}

func (ai *aiManager) getDbStats(namespace *string) (totalDbEntries int, unprocessedDbEntries int, ignoredDbEntries int, numberOfUnreadTasks int, err error) {
	key := ai.getValkeyKey("*", "*", "*")
	if namespace != nil {
		key = ai.getValkeyKey("*", *namespace, "*")
	}

	cursor := uint64(0)
	ctx := context.Background()

	for {
		// Build and execute SCAN command
		scanCmd := ai.valkeyClient.GetValkeyClient().B().Scan().Cursor(cursor).Match(key).Count(100).Build()
		scanResult, err := ai.valkeyClient.GetValkeyClient().Do(ctx, scanCmd).AsScanEntry()
		if err != nil {
			return 0, 0, 0, 0, err
		}

		if len(scanResult.Elements) > 0 {
			// Build all GET commands for this batch
			cmds := make([]valkey.Completed, len(scanResult.Elements))
			for i, k := range scanResult.Elements {
				cmds[i] = ai.valkeyClient.GetValkeyClient().B().Get().Key(k).Build()
			}

			// Execute all GETs in a single round trip
			results := ai.valkeyClient.GetValkeyClient().DoMulti(ctx, cmds...)

			// Process results
			for _, result := range results {
				item, err := result.ToString()
				if err != nil {
					// Key might have been deleted or expired, skip it
					continue
				}

				var task AiTask
				if err := json.Unmarshal([]byte(item), &task); err != nil {
					// Log error but continue processing
					continue
				}

				totalDbEntries++

				if task.State == AI_TASK_STATE_PENDING || task.State == AI_TASK_STATE_FAILED {
					unprocessedDbEntries++
				}
				if task.State == AI_TASK_STATE_IGNORED {
					ignoredDbEntries++
				}
				if len(task.ReadByUsers) == 0 {
					numberOfUnreadTasks++
				}
			}
		}

		cursor = scanResult.Cursor
		if cursor == 0 {
			break // SCAN complete
		}
	}

	return totalDbEntries, unprocessedDbEntries, ignoredDbEntries, numberOfUnreadTasks, nil
}

func (ai *aiManager) addTokenUsage(tokensUsed int, model string, timeUsedInMs int, entryKey string) error {
	now := time.Now()
	key := fmt.Sprintf("%s:%d", DB_AI_BUCKET_TOKENS, now.Unix())

	usedToken := UsedToken{
		Key:          entryKey,
		Timestamp:    now,
		TokensUsed:   int64(tokensUsed),
		IsIgnored:    false,
		Model:        model,
		TimeUsedInMs: timeUsedInMs,
	}

	err := ai.valkeyClient.SetObject(usedToken, ValkeyAiTTL, key)
	if err != nil {
		return fmt.Errorf("error saving AI token usage: %v", err)
	}

	return nil
}

func (ai *aiManager) resetTodayTokenUsage() error {
	// Calculate the start of today in Unix timestamp
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	keys, err := ai.valkeyClient.Keys(fmt.Sprintf("%s:*", DB_AI_BUCKET_TOKENS))
	if err != nil {
		return err
	}

	var resettedTokens int64 = 0
	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			return err
		}
		var tokenEntry UsedToken
		err = json.Unmarshal([]byte(item), &tokenEntry)
		if err != nil {
			return err
		}
		if tokenEntry.Timestamp.Unix() >= startOfDay {
			resettedTokens += tokenEntry.TokensUsed
			tokenEntry.TokensUsed = 0
			err := ai.valkeyClient.SetObject(tokenEntry, ValkeyAiTTL, key)
			if err != nil {
				return fmt.Errorf("error saving AI token usage: %v", err)
			}

		}
	}
	ai.logger.Info("Reset today's AI token usage", "resettedTokens", resettedTokens)

	ai.resetCache()

	return nil
}

func (ai *aiManager) getAllTaskKeys() ([]string, error) {
	pattern := fmt.Sprintf("%s:*", DB_AI_BUCKET_TASKS)
	cursor := uint64(0)
	ctx := context.Background()
	var allKeys []string

	for {
		scanCmd := ai.valkeyClient.GetValkeyClient().B().Scan().Cursor(cursor).Match(pattern).Count(100).Build()
		scanResult, err := ai.valkeyClient.GetValkeyClient().Do(ctx, scanCmd).AsScanEntry()
		if err != nil {
			return nil, err
		}

		allKeys = append(allKeys, scanResult.Elements...)

		cursor = scanResult.Cursor
		if cursor == 0 {
			break
		}
	}

	return allKeys, nil
}

func (ai *aiManager) processAiTaskQueue(ctx context.Context) {
	keys, err := ai.getAllTaskKeys()
	if err != nil {
		ai.logger.Error("Error listing AI tasks", "error", err)
		return
	}
	// Process items here
	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			ai.logger.Error("Error getting AI task", "key", key, "error", err)
			continue
		}
		var task AiTask
		err = json.Unmarshal([]byte(item), &task)
		if err != nil {
			ai.logger.Error("Error unmarshaling AI task", "key", key, "error", err)
			continue
		}

		// Process only pending tasks or retry failed tasks
		if task.State != AI_TASK_STATE_PENDING && task.State != AI_TASK_STATE_FAILED {
			continue
		}

		if ai.isTokenLimitExceeded() {
			task.State = AI_TASK_STATE_FAILED
			task.Error = "Daily AI token limit exceeded, cannot process further tasks. Increase limit or wait 24 hours."
			err := ai.createOrUpdateAiTask(&task, key)
			if err != nil {
				ai.logger.Error("Error updating AI task", "taskID", task.ID, "error", err)
			}
			continue
		}

		task.State = AI_TASK_STATE_IN_PROGRESS
		task.Error = ""
		err = ai.createOrUpdateAiTask(&task, key)
		if err != nil {
			ai.logger.Error("Failed to set AI task state to in progress", "taskID", task.ID, "error", err)
			continue
		}

		latestTask := &AiTaskLatest{
			Task:   &task,
			Status: ai.GetStatus(nil),
		}

		// send event notification
		ai.sendAiEvent(latestTask)

		response, tokensUsed, timeUsedInMs, modelUsed, err := ai.processPrompt(ctx, task.Prompt)
		if err != nil {
			task.Error = err.Error()
			task.State = AI_TASK_STATE_FAILED
			ai.logger.Error("Error processing AI task", "taskID", task.ID, "error", err)
		} else {
			task.State = AI_TASK_STATE_COMPLETED
			task.Response = response

		}
		task.Model = modelUsed
		task.TimeUsedInMs = timeUsedInMs
		task.TokensUsed = tokensUsed
		err = ai.addTokenUsage(int(tokensUsed), modelUsed, timeUsedInMs, key)
		if err != nil {
			ai.logger.Error("Error recording AI token usage", "taskID", task.ID, "error", err)
		}

		// update status for event
		ai.resetCache()
		latestTask.Status = ai.GetStatus(nil)

		// send event notification
		ai.sendAiEvent(latestTask)

		// Save updated task
		err = ai.createOrUpdateAiTask(&task, key)
		if err != nil {
			ai.logger.Error("Error updating AI task", "taskID", task.ID, "error", err)
			continue
		}
		ai.logger.Info("AI task processed", "taskID", task.ID, "tokensUsed", task.TokensUsed, "state", task.State, "name", task.ReferencingResource.ResourceName, "namespace", task.ReferencingResource.Namespace)
	}
}

// HELPER FUNCTIONS
func getNestedStringWithJSONPath(obj *unstructured.Unstructured, path string, keyword string) (value string, found bool, err error) {
	j := jsonpath.New("parser")
	j.AllowMissingKeys(true)

	// JSONPath expects the path to start with {}, e.g., {.status.conditions[?(@.type=="Ready")].status}
	if !strings.HasPrefix(path, "{") {
		path = "{" + path + "}"
	}

	// JSONPath expects double quotes instead of single quotes
	path = strings.ReplaceAll(path, "'", "\"")

	if err := j.Parse(path); err != nil {
		return "", false, fmt.Errorf("failed to parse JSONPath: %w", err)
	}

	results, err := j.FindResults(obj.Object)
	if err != nil {
		// Handle array out of bounds as "not found" instead of error
		if strings.Contains(err.Error(), "array index out of bounds") {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to find results: %w", err)
	}

	if len(results) == 0 || len(results[0]) == 0 {
		return "", false, nil
	}

	// Handle multiple results (when using [*] or filters that return multiple items)
	var allValues []string
	for _, resultArray := range results {
		for _, result := range resultArray {
			val := result.Interface()

			// Handle string result
			if str, ok := val.(string); ok {
				allValues = append(allValues, str)
				continue
			}

			// Handle map (for keyword search)
			if labelMap, ok := val.(map[string]any); ok {
				var matches []string
				for key, value := range labelMap {
					valueStr := fmt.Sprintf("%v", value)
					if keyword == "" {
						// If no keyword, return all key=value pairs
						matches = append(matches, fmt.Sprintf("%s=%s", key, valueStr))
					} else if strings.Contains(strings.ToLower(key), strings.ToLower(keyword)) ||
						strings.Contains(strings.ToLower(valueStr), strings.ToLower(keyword)) {
						matches = append(matches, fmt.Sprintf("%s=%s", key, valueStr))
					}
				}

				if len(matches) > 0 {
					sort.Strings(matches)
					allValues = append(allValues, strings.Join(matches, ", "))
				}
				continue
			}

			// Handle other types (numbers, booleans, etc.)
			allValues = append(allValues, fmt.Sprintf("%v", val))
		}
	}

	if len(allValues) == 0 {
		return "", false, nil
	}

	// Join all values with a delimiter
	return strings.Join(allValues, ", "), true, nil
}

func (ai *aiManager) createOrUpdateAiTask(task *AiTask, key string) error {
	timestamp := time.Now().Unix()
	task.UpdatedAt = timestamp

	jsonString, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("error marshaling AI task: %v", err)
	}
	err = ai.valkeyClient.Set(string(jsonString), ValkeyAiTTL, key)
	if err != nil {
		return fmt.Errorf("error saving AI task: %v", err)
	}

	// last updated task
	err = ai.valkeyClient.Set(string(jsonString), ValkeyAiTTL, ai.getValkeyLatestTaskKey())
	if err != nil {
		ai.logger.Warn("Error saving AI task", "error", err)
	}
	parts := strings.Split(key, ":")
	if len(parts) > 2 {
		namespace := parts[2]
		err = ai.valkeyClient.Set(string(jsonString), ValkeyAiTTL, ai.getValkeyLatestNamespaceTaskKey(namespace))
		if err != nil {
			ai.logger.Warn("Error saving AI task for namespace", "namespace", namespace, "error", err)
		}
	}

	return nil
}

func (ai *aiManager) shouldCreateNewTask(key string) (bool, error) {
	exists, err := ai.valkeyClient.Exists(key)
	if err != nil {
		return false, fmt.Errorf("error checking if AI task exists: %v", err)
	}
	return !exists, nil
}

func (ai *aiManager) processPrompt(ctx context.Context, prompt string) (response *AiResponse, tokensUsed int64, timeUsedInMs int, modelUser string, err error) {
	startTime := time.Now()
	model, err := ai.getAiModel()
	if err != nil {
		return nil, 0, int(time.Since(startTime).Milliseconds()), model, err
	}
	systemPrompt := ai.getSystemPrompt()

	sdk, err := ai.getSdkType()
	if err != nil {
		return nil, 0, int(time.Since(startTime).Milliseconds()), model, err
	}
	switch sdk {
	case AiSdkTypeOpenAI:
		return ai.processPromptOpenAi(ctx, model, systemPrompt, prompt)
	case AiSdkTypeAnthropic:
		return ai.processPromptAnthropic(ctx, model, systemPrompt, prompt)
	case AiSdkTypeOllama:
		return ai.processPromptOllama(ctx, model, systemPrompt, prompt)
	default:
		return nil, 0, int(time.Since(startTime).Milliseconds()), model, fmt.Errorf("unsupported AI SDK type: %s", sdk)
	}
}

// for nasty AIs which return markdown code blocks or extra text around JSON
func cleanJSONResponse(response string) string {
	// Trim whitespace
	response = strings.TrimSpace(response)

	// Remove markdown code blocks (```json or ``` at start/end)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")

	// Trim again after removing code blocks
	return strings.TrimSpace(response)
}

func extractJSONRobust(text string) (jsonData []byte, removedText string, err error) {
	start := strings.Index(text, "{")
	if start == -1 {
		return nil, "", fmt.Errorf("no JSON object found")
	}

	// Capture the text that was removed (the "bullshit")
	removedText = text[:start]

	braceCount := 0
	inString := false
	escapeNext := false

	for i := start; i < len(text); i++ {
		char := text[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
				if braceCount == 0 {
					return []byte(text[start : i+1]), removedText, nil
				}
			}
		}
	}

	return nil, removedText, fmt.Errorf("unbalanced braces in JSON")
}

func buildUserPrompt(prompt string, obj *unstructured.Unstructured) string {
	objBytes, err := yaml.Marshal(obj.Object)
	if err != nil {
		return fmt.Sprintf("%s\n\nError serializing Kubernetes object: %v", prompt, err)
	}
	return fmt.Sprintf("%s\n\nHere are the related Kubernetes resources in yaml format:\n%s", prompt, string(objBytes))
}

func (ai *aiManager) getOpenAIClient(request *ModelsRequest) (*openai.Client, error) {
	var apiKey string
	if request != nil && request.ApiKey != nil {
		apiKey = *request.ApiKey
	} else {
		var err error
		apiKey, err = ai.getApiKey()
		if err != nil {
			return nil, err
		}
	}
	var baseUrl string
	if request != nil {
		baseUrl = request.ApiUrl
	} else {
		var err error
		baseUrl, err = ai.getBaseUrl()
		if err != nil {
			return nil, err
		}
	}
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseUrl),
	)
	return &client, nil
}

func (ai *aiManager) getAnthropicClient(request *ModelsRequest) (*anthropic.Client, error) {
	var apiKey string
	if request != nil && request.ApiKey != nil {
		apiKey = *request.ApiKey
	} else {
		var err error
		apiKey, err = ai.getApiKey()
		if err != nil {
			return nil, err
		}
	}
	var baseUrl string
	if request != nil {
		baseUrl = request.ApiUrl
	} else {
		var err error
		baseUrl, err = ai.getBaseUrl()
		if err != nil {
			return nil, err
		}
	}

	client := anthropic.NewClient(
		anthropic_option.WithBaseURL(baseUrl),
		anthropic_option.WithAPIKey(apiKey),
	)
	return &client, nil
}

func (ai *aiManager) getOllamaClient(request *ModelsRequest) (*ollama.Client, error) {
	var baseUrl string
	if request != nil {
		baseUrl = request.ApiUrl
	} else {
		var err error
		baseUrl, err = ai.getBaseUrl()
		if err != nil {
			return nil, err
		}
	}
	url, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	client := api.NewClient(url, http.DefaultClient)

	return client, nil
}

func (ai *aiManager) GetAvailableModels(request *ModelsRequest) ([]string, error) {
	var sdk AiSdkType
	if request != nil {
		sdk = AiSdkType(request.Sdk)
	} else {
		var err error
		sdk, err = ai.getSdkType()
		if err != nil {
			return []string{}, err
		}
	}
	switch sdk {
	case AiSdkTypeOpenAI:
		client, err := ai.getOpenAIClient(request)
		if err != nil {
			ai.logger.Error("Error getting OpenAI client for available models", "error", err)
			return []string{}, err
		}

		ctx := context.Background()
		models, err := client.Models.List(ctx)
		if err != nil {
			ai.logger.Error("Error listing available AI models", "error", err)
			return []string{}, err
		}

		var modelNames []string
		for _, model := range models.Data {
			modelNames = append(modelNames, model.ID)
		}
		return modelNames, nil
	case AiSdkTypeAnthropic:
		client, err := ai.getAnthropicClient(request)
		if err != nil {
			ai.logger.Error("Error getting Anthropic client for available models", "error", err)
			return []string{}, err
		}

		ctx := context.Background()
		models, err := client.Models.List(ctx, anthropic.ModelListParams{})
		if err != nil {
			ai.logger.Error("Error listing available AI models", "error", err)
			return []string{}, err
		}

		var modelNames []string
		for _, model := range models.Data {
			modelNames = append(modelNames, model.ID)
		}
		return modelNames, nil
	case AiSdkTypeOllama:
		client, err := ai.getOllamaClient(request)
		if err != nil {
			ai.logger.Error("Error getting Ollama client for available models", "error", err)
			return []string{}, err
		}

		ctx := context.Background()
		listResponse, err := client.List(ctx)
		if err != nil {
			ai.logger.Error("Error listing available AI models", "error", err)
			return []string{}, err
		}

		var modelNames []string
		for _, model := range listResponse.Models {
			modelNames = append(modelNames, model.Name)
		}
		return modelNames, nil
	default:
		return []string{}, fmt.Errorf("unsupported AI SDK type: %s", sdk)
	}
}

func (ai *aiManager) getValkeyKey(kind, namespace, name string) string {
	// controller lookup for pods
	if kind == "Pod" {
		controller := ai.ownerCacheService.ControllerForPod(namespace, name)
		if controller != nil {
			kind = controller.Kind
			name = controller.ResourceName
		}
	}
	return fmt.Sprintf("%s:%s:%s:%s", DB_AI_BUCKET_TASKS, kind, namespace, name)
}

func (ai *aiManager) getValkeyLatestTaskKey() string {
	return fmt.Sprintf("%s:%s", DB_AI_BUCKET_TASKS_LATEST, DB_AI_LATEST_TASK_KEY)
}

func (ai *aiManager) getValkeyLatestNamespaceTaskKey(namespace string) string {
	return fmt.Sprintf("%s:%s:%s", DB_AI_BUCKET_TASKS_LATEST, DB_AI_LATEST_NAMESPACE_TASK_KEY, namespace)
}

func (ai *aiManager) sendAiEvent(task *AiTaskLatest) {
	datagram := structs.Datagram{
		Id:      utils.NanoId(),
		Pattern: "AiProcessEvent",
		Payload: map[string]interface{}{
			"task":   task.Task,
			"status": task.Status,
		},
		CreatedAt: time.Now(),
	}
	structs.ReportEventToServer(ai.eventClient, datagram)
}

func (ai *aiManager) resetCache() {
	cachedStatusTime = time.Time{}
}
