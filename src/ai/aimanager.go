package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"mogenius-operator/src/websocket"
	"net/url"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/anthropics/anthropic-sdk-go"
	anthropic_option "github.com/anthropics/anthropic-sdk-go/option"

	"github.com/ollama/ollama/api"
	ollama "github.com/ollama/ollama/api"
)

const (
	DB_AI_BUCKET_TASKS  = "ai_tasks"
	DB_AI_BUCKET_TOKENS = "ai_tokens"
)

type AiTaskState string
type AiTask struct {
	ID                  string                       `json:"id"`
	Prompt              string                       `json:"prompt"`
	Response            *AiResponse                  `json:"response"`
	State               AiTaskState                  `json:"state"` // pending, in-progress, completed, failed, ignored
	Controller          *utils.WorkloadSingleRequest `json:"controller,omitempty"`
	TokensUsed          int64                        `json:"tokensUsed"`
	CreatedAt           int64                        `json:"createdAt"`
	UpdatedAt           int64                        `json:"updatedAt"`
	ReferencingResource utils.WorkloadSingleRequest  `json:"referencingResource"` // the resource that triggered this task
	TriggeredBy         AiFilter                     `json:"triggeredBy"`         // e.g., "Failed Pods" filter
	ReadByUser          *ReadBy                      `json:"readByUser,omitempty"`
	Error               string                       `json:"error"`
}

type AiTaskLatest struct {
	Task                *AiTask `json:"task,omitempty"`
	NumberOfUnreadTasks int     `json:"numberOfUnreadTasks"`
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
)

type AiFilter struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Kind        string            `json:"kind"`
	Contains    map[string]string `json:"contains"` // {"Running": "status.phase"}, {"ImagePullBackOff": "status.phase.ContainerStatuses.state.waiting.reason"}
	Excludes    map[string]string `json:"excludes"` // {"Succeeded": "status.phase"}, {"Completed": "status.phase"}
	Prompt      string            `json:"prompt"`
}

type AiPromptConfig struct {
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
	Timestamp  time.Time `json:"timestamp"`
	TokensUsed int64     `json:"tokensUsed"`
	IsIgnored  bool      `json:"isIgnored"`
	Key        string    `json:"key"`
}

type ModelsRequest struct {
	Sdk    string  `json:"SDK,omitempty"`
	ApiKey *string `json:"API_KEY,omitempty"`
	ApiUrl string  `json:"API_URL,omitempty"`
}

var AiFilters = []AiFilter{
	{
		Name:        "Failed Pods",
		Description: "The pod is in a Failed state due to various reasons such as CrashLoopBackOff, ImagePullBackOff, etc.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.phase": "Failed",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod failed and suggest possible solutions.",
	},
	{
		Name:        "CrashLoopBackOff Pods",
		Description: "The pod is in CrashLoopBackOff state due to container crashes.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.containerStatuses[*].state.waiting.reason": "CrashLoopBackOff",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is in CrashLoopBackOff and suggest possible solutions.",
	},
	{
		Name:        "ImagePullBackOff Pods",
		Description: "The pod is in ImagePullBackOff state due to image pull errors.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.containerStatuses[*].state.waiting.reason": "ImagePullBackOff",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is in ImagePullBackOff and suggest possible solutions.",
	},
	{
		Name:        "ErrImagePull Pods",
		Description: "The pod is in ErrImagePull state due to image pull errors.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.containerStatuses[*].state.waiting.reason": "ErrImagePull",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod cannot pull its image and suggest possible solutions.",
	},
	{
		Name:        "CreateContainerConfigError Pods",
		Description: "The pod is in CreateContainerConfigError state likely due to ConfigMap or Secret issues.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.containerStatuses[*].state.waiting.reason": "CreateContainerConfigError",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod has a CreateContainerConfigError (likely ConfigMap or Secret issue) and suggest possible solutions.",
	},
	{
		Name:        "InvalidImageName Pods",
		Description: "The pod is in InvalidImageName state due to an invalid image name.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.containerStatuses[*].state.waiting.reason": "InvalidImageName",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod has an invalid image name and suggest possible solutions.",
	},
	{
		Name:        "Unused ReplicaSet",
		Description: "The ReplicaSet has zero replicas, indicating it is not currently in use.",
		Kind:        "ReplicaSet",
		Contains: map[string]string{
			".spec.replicas":   "0",
			".status.replicas": "0",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this ReplicaSet has zero replicas and suggest possible solutions. It might be unused and can most potentially be deleted.",
	},
	{
		Name:        "PodNotReady",
		Description: "The pod is NotReady, indicating it is not yet ready to serve traffic.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.conditions[?(@.type=='Ready')].status": "False",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is NotReady and suggest possible solutions.",
	},
	{
		Name:        "Pending Pods",
		Description: "The pod is in Pending state, likely due to scheduling issues, resource constraints, or PVC problems.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.phase": "Pending",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is stuck in Pending state (likely scheduling issues, resource constraints, or PVC problems) and suggest possible solutions.",
	},
	{
		Name:        "OOMKilled Containers",
		Description: "The pod's container was OOMKilled (Out of Memory), likely due to memory limits being exceeded.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.containerStatuses[*].lastState.terminated.reason": "OOMKilled",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod's container was OOMKilled (Out of Memory) and suggest possible solutions including memory limit adjustments.",
	},
	{
		Name:        "Deployment with unavailable replicas",
		Description: "The Deployment has unavailable replicas, possibly due to pod failures or insufficient resources.",
		Kind:        "Deployment",
		Contains: map[string]string{
			".status.conditions[?(@.type=='Available')].status": "False",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Deployment has unavailable replicas and suggest possible solutions.",
	},
	{
		Name:        "StatefulSet with failed replicas",
		Kind:        "StatefulSet",
		Description: "The StatefulSet has failed replicas, possibly due to pod failures or insufficient resources.",
		Contains: map[string]string{
			".status.conditions[?(@.type=='Ready')].status": "False",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this StatefulSet has failed replicas and suggest possible solutions.",
	},
	{
		Name:        "PVC Pending",
		Description: "The PersistentVolumeClaim is Pending, likely due to no matching PersistentVolume or StorageClass issues.",
		Kind:        "PersistentVolumeClaim",
		Contains: map[string]string{
			".status.phase": "Pending",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this PersistentVolumeClaim is Pending (likely no matching PV or StorageClass issues) and suggest possible solutions.",
	},
	{
		Name:        "Service with no endpoints",
		Description: "The Service has no endpoints, likely due to selector mismatch or no ready Pods.",
		Kind:        "Service",
		Contains:    map[string]string{},
		Excludes:    map[string]string{},
		Prompt:      "Check if this Service has endpoints. If not, provide a detailed analysis of why (likely selector mismatch or no ready Pods) and suggest possible solutions.",
	},
	{
		Name:        "Unschedulable Pods",
		Description: "The pod is Unschedulable, likely due to resource constraints, node affinity, or taints.",
		Kind:        "Pod",
		Contains: map[string]string{
			".status.conditions[?(@.type=='PodScheduled')].reason": "Unschedulable",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is unschedulable (likely resource constraints, node affinity, or taints) and suggest possible solutions.",
	},
	{
		Name:        "Jobs that failed",
		Description: "The Job has failed to complete, possibly due to pod failures or misconfiguration.",
		Kind:        "Job",
		Contains: map[string]string{
			".status.conditions[?(@.type=='Complete')].status": "False",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Job failed to complete and suggest possible solutions.",
	},
	{
		Name:        "HPA unable to scale",
		Description: "The HorizontalPodAutoscaler is unable to scale, likely due to metrics-server issues or invalid configuration.",
		Kind:        "HorizontalPodAutoscaler",
		Contains: map[string]string{
			".status.conditions[?(@.type=='AbleToScale')].status": "False",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this HorizontalPodAutoscaler cannot scale (likely metrics-server issues or invalid configuration) and suggest possible solutions.",
	},
}

type AiManager interface {
	ProcessObject(obj *unstructured.Unstructured, eventType string, resource utils.ResourceDescriptor) // eventType can be "add", "update", "delete"
	Run()
	UpdateTaskState(taskID string, newState AiTaskState) error
	UpdateTaskReadState(taskID string, user *structs.User) error
	GetAiTasksForWorkspace(workspace string) ([]AiTask, error)
	GetAiTasksForResource(resourceReq utils.WorkloadSingleRequest) ([]AiTask, error)
	GetLatestTask(workspace string) (*AiTaskLatest, error)
	InjectAiPromptConfig(prompt AiPromptConfig)
	GetStatus() AiManagerStatus
	ResetDailyTokenLimit() error
	DeleteAllAiData() error
	GetAvailableModels(request *ModelsRequest) ([]string, error)
}

type aiManager struct {
	logger            *slog.Logger
	valkeyClient      valkeyclient.ValkeyClient
	config            cfg.ConfigModule
	aiPromptConfig    *AiPromptConfig
	ownerCacheService store.OwnerCacheService
	eventClient       websocket.WebsocketClient
	error             string
	warning           string
}

func NewAiManager(logger *slog.Logger, valkeyClient valkeyclient.ValkeyClient, config cfg.ConfigModule, ownerCacheService store.OwnerCacheService, eventClient websocket.WebsocketClient) AiManager {
	self := &aiManager{}

	self.logger = logger
	self.valkeyClient = valkeyClient
	self.config = config
	self.ownerCacheService = ownerCacheService
	self.eventClient = eventClient

	return self
}

func (ai *aiManager) ProcessObject(obj *unstructured.Unstructured, eventType string, resource utils.ResourceDescriptor) {
	if obj == nil {
		return
	}

	if eventType == "delete" {
		// On delete, we try to remove any existing AI tasks for this object
		key := ai.getValkeyKey(obj.GetKind(), obj.GetNamespace(), obj.GetName(), "*")
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
			var matchedFilter *AiFilter = nil
			// check contains conditions
			for path, expectedValue := range filter.Contains {
				value, found, err := getNestedStringWithJSONPath(obj, path, expectedValue)
				if err != nil {
					ai.logger.Error("Error checking AI filter contains", "expectedValue", expectedValue, "error", err)
					continue
				}
				if found && value == expectedValue {
					matchedFilter = &filter
					break
				}
			}
			// check excludes conditions
			for path, expectedValue := range filter.Excludes {
				value, found, err := getNestedStringWithJSONPath(obj, path, expectedValue)
				if err != nil {
					ai.logger.Error("Error checking AI filter excludes", "expectedValue", expectedValue, "error", err)
					continue
				}
				if found && value == expectedValue {
					matchedFilter = nil
					break
				}
			}
			if matchedFilter != nil {
				timestamp := time.Now().Unix()
				// create AI task
				task := &AiTask{
					ID:        utils.NanoIdSmallLowerCase(),
					Prompt:    buildUserPrompt(filter.Prompt, obj),
					State:     AI_TASK_STATE_PENDING,
					CreatedAt: timestamp,
					UpdatedAt: timestamp,
					ReferencingResource: utils.WorkloadSingleRequest{
						ResourceDescriptor: resource,
						Namespace:          obj.GetNamespace(),
						ResourceName:       obj.GetName(),
					},
					TriggeredBy: *matchedFilter,
					Error:       "",
				}

				key := ai.getValkeyKey(obj.GetKind(), obj.GetNamespace(), obj.GetName(), filter.Name)
				shouldCreate, err := ai.shouldCreateNewTask(key)
				if err != nil {
					ai.logger.Error("Error checking if should create new AI task", "error", err)
					continue
				}

				if shouldCreate {
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
					err = ai.createOrUpdateAiTask(task, key)
					if err != nil {
						ai.logger.Error("Error creating AI task", "error", err)
					} else {
						ai.logger.Info("AI task created", "taskID", task.ID, "event", eventType, "objectKind", obj.GetKind(), "objectName", obj.GetName(), "objectNamespace", obj.GetNamespace(), "filter", filter.Name)
					}
				}
			}
		}
	}
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

			ai.error = ""
			ai.processAiTaskQueue(ctx)
		}
	}()
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
	// Calculate the start of today in Unix timestamp
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	keys, err := ai.valkeyClient.Keys(DB_AI_BUCKET_TOKENS + ":*")
	if err != nil {
		return 0, 0, err
	}

	var totalTokens int64 = 0
	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			return -1, -1, err
		}
		var tokenEntry UsedToken
		err = json.Unmarshal([]byte(item), &tokenEntry)
		if err != nil {
			return -1, -1, err
		}

		if tokenEntry.Timestamp.Unix() >= startOfDay && !tokenEntry.IsIgnored {
			totalTokens += tokenEntry.TokensUsed
		}
	}

	return totalTokens, len(keys), nil
}

func (ai *aiManager) getDbStats() (totalDbEntries int, unprocessedDbEntries int, ignoredDbEntries int, err error) {
	keys, err := ai.valkeyClient.Keys(DB_AI_BUCKET_TASKS + ":*")
	if err != nil {
		return 0, 0, 0, err
	}

	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			return 0, 0, 0, err
		}
		var task AiTask
		err = json.Unmarshal([]byte(item), &task)
		if err != nil {
			return 0, 0, 0, err
		}

		if task.State == AI_TASK_STATE_PENDING || task.State == AI_TASK_STATE_FAILED {
			unprocessedDbEntries++
		}
		if task.State == AI_TASK_STATE_IGNORED {
			ignoredDbEntries++
		}
	}

	return len(keys), unprocessedDbEntries, ignoredDbEntries, nil
}

func (ai *aiManager) addTokenUsage(tokensUsed int, entryKey string) error {
	now := time.Now()
	key := fmt.Sprintf("%s:%d", DB_AI_BUCKET_TOKENS, now.Unix())

	usedToken := UsedToken{
		Key:        entryKey,
		Timestamp:  now,
		TokensUsed: int64(tokensUsed),
		IsIgnored:  false,
	}

	err := ai.valkeyClient.SetObject(usedToken, time.Hour*24*7, key)
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
			err := ai.valkeyClient.SetObject(tokenEntry, time.Hour*24*7, key)
			if err != nil {
				return fmt.Errorf("error saving AI token usage: %v", err)
			}

		}
	}
	ai.logger.Info("Reset today's AI token usage", "resettedTokens", resettedTokens)

	return nil
}

func (ai *aiManager) processAiTaskQueue(ctx context.Context) {
	keys, err := ai.valkeyClient.Keys(fmt.Sprintf("%s:*", DB_AI_BUCKET_TASKS))
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

		if ai.isTokenLimitExceeded() {
			task.State = AI_TASK_STATE_FAILED
			task.Error = "Daily AI token limit exceeded, cannot process further tasks. Increase limit or wait 24 hours."
			err := ai.createOrUpdateAiTask(&task, key)
			if err != nil {
				ai.logger.Error("Error updating AI task", "taskID", task.ID, "error", err)
			}
			continue
		} else {
			task.Error = ""
			task.State = AI_TASK_STATE_PENDING
		}

		// Process only pending tasks
		if task.State != AI_TASK_STATE_PENDING {
			continue
		}

		task.State = AI_TASK_STATE_IN_PROGRESS
		err = ai.createOrUpdateAiTask(&task, key)
		if err != nil {
			ai.logger.Error("Failed to set AI task state to in progress", "taskID", task.ID, "error", err)
			continue
		}

		// send event notification
		ai.sendAiEvent(&task)

		response, tokensUsed, err := ai.processPrompt(ctx, task.Prompt)
		if err != nil {
			task.Error = err.Error()
			task.State = AI_TASK_STATE_FAILED
			task.TokensUsed = tokensUsed
			ai.logger.Error("Error processing AI task", "taskID", task.ID, "error", err)
		} else {
			task.State = AI_TASK_STATE_COMPLETED
			task.Response = response
			task.TokensUsed = tokensUsed
		}
		err = ai.addTokenUsage(int(tokensUsed), key)
		if err != nil {
			ai.logger.Error("Error recording AI token usage", "taskID", task.ID, "error", err)
		}

		// send event notification
		ai.sendAiEvent(&task)

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
	err = ai.valkeyClient.Set(string(jsonString), time.Hour*24*7, key)
	if err != nil {
		return fmt.Errorf("error saving AI task: %v", err)
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

func (ai *aiManager) processPrompt(ctx context.Context, prompt string) (*AiResponse, int64, error) {
	model, err := ai.getAiModel()
	if err != nil {
		return nil, 0, err
	}
	systemPrompt := ai.getSystemPrompt()

	sdk, err := ai.getSdkType()
	if err != nil {
		return nil, 0, err
	}
	switch sdk {
	case AiSdkTypeOpenAI:
		client, err := ai.getOpenAIClient(nil)
		if err != nil {
			return nil, 0, err
		}

		chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(prompt),
				openai.SystemMessage(systemPrompt),
			},
			Model: model,
		})

		var tokensUsed int64 = 0
		if chatCompletion != nil {
			tokensUsed = chatCompletion.Usage.TotalTokens
		}

		if err != nil {
			return nil, tokensUsed, err
		}
		if len(chatCompletion.Choices) == 0 {
			return nil, tokensUsed, fmt.Errorf("no choices returned from AI model")
		}

		var aiResponse AiResponse
		err = json.Unmarshal([]byte(chatCompletion.Choices[0].Message.Content), &aiResponse)
		if err != nil {
			return nil, tokensUsed, fmt.Errorf("error unmarshaling AI response: %v\n%s", err, chatCompletion.Choices[0].Message.Content)
		}

		// also return tokens used
		return &aiResponse, tokensUsed, nil
	case AiSdkTypeAnthropic:
		client, err := ai.getAnthropicClient(nil)
		if err != nil {
			return nil, 0, err
		}

		message, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: int64(10000),
			System: []anthropic.TextBlockParam{
				{
					Type: "text",
					Text: systemPrompt,
				},
			},
			Messages: []anthropic.MessageParam{
				{
					Role: anthropic.MessageParamRoleUser,
					Content: []anthropic.ContentBlockParamUnion{
						anthropic.NewTextBlock(prompt),
					},
				},
			},
		})

		var tokensUsed int64 = 0
		if message != nil {
			tokensUsed = message.Usage.InputTokens + message.Usage.OutputTokens
		}

		if err != nil {
			return nil, tokensUsed, err
		}

		if len(message.Content) == 0 {
			return nil, tokensUsed, fmt.Errorf("no content returned from AI model")
		}

		// Extract text from content blocks
		var responseText string
		for _, block := range message.Content {
			responseText += block.Text
		}
		responseText = cleanJSONResponse(responseText)

		var aiResponse AiResponse
		err = json.Unmarshal([]byte(responseText), &aiResponse)
		if err != nil {
			return nil, tokensUsed, fmt.Errorf("error unmarshaling AI response: %v\n%s", err, responseText)
		}

		return &aiResponse, tokensUsed, nil
	case AiSdkTypeOllama:
		client, err := ai.getOllamaClient(nil)
		if err != nil {
			return nil, 0, err
		}

		req := &api.ChatRequest{
			Model: model,
			Messages: []api.Message{
				{
					Role:    "system",
					Content: systemPrompt,
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			Stream: new(bool), // false - we want a single response
		}

		var responseText string
		var promptEvalCount int
		var evalCount int

		err = client.Chat(ctx, req, func(resp api.ChatResponse) error {
			responseText += resp.Message.Content
			if resp.Done {
				promptEvalCount = resp.PromptEvalCount
				evalCount = resp.EvalCount
			}
			return nil
		})

		var tokensUsed int64 = 0
		if err == nil {
			tokensUsed = int64(promptEvalCount + evalCount)
		}

		if err != nil {
			return nil, tokensUsed, err
		}

		if responseText == "" {
			return nil, tokensUsed, fmt.Errorf("no content returned from AI model")
		}

		responseText = cleanJSONResponse(responseText)

		var aiResponse AiResponse
		err = json.Unmarshal([]byte(responseText), &aiResponse)
		if err != nil {
			return nil, tokensUsed, fmt.Errorf("error unmarshaling AI response: %v\n%s", err, responseText)
		}

		return &aiResponse, tokensUsed, nil
	default:
		return nil, 0, fmt.Errorf("unsupported AI SDK type: %s", sdk)
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

func buildUserPrompt(prompt string, obj *unstructured.Unstructured) string {
	objJsonBytes, err := json.MarshalIndent(obj.Object, "", "  ")
	if err != nil {
		return fmt.Sprintf("%s\n\nError serializing Kubernetes object: %v", prompt, err)
	}
	objJson := string(objJsonBytes)

	return fmt.Sprintf("%s\n\nHere is the Kubernetes object in JSON format:\n%s", prompt, objJson)
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

	client := api.NewClient(url, nil)

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

func (ai *aiManager) getValkeyKey(kind, namespace, name, filter string) string {
	// controller lookup for pods
	if kind == "Pod" {
		controller := ai.ownerCacheService.ControllerForPod(namespace, name)
		if controller != nil {
			kind = controller.Kind
			name = controller.ResourceName
		}
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s", DB_AI_BUCKET_TASKS, kind, namespace, name, filter)
}

func (ai *aiManager) sendAiEvent(task *AiTask) {
	datagram := structs.Datagram{
		Id:      utils.NanoId(),
		Pattern: "AiProcessEvent",
		Payload: map[string]interface{}{
			"task": task,
		},
		CreatedAt: time.Now(),
	}
	structs.ReportEventToServer(ai.eventClient, datagram)
}
