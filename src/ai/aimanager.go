package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/jsonpath"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	DB_AI_BUCKET_TASKS = "ai_tasks"
)

type AiTaskState string
type AiTask struct {
	ID                  string                      `json:"id"`
	Prompt              string                      `json:"prompt"`
	Response            *AiResponse                 `json:"response"`
	State               AiTaskState                 `json:"state"` // pending, in-progress, completed, failed, ignored
	TokensUsed          int64                       `json:"tokensUsed"`
	CreatedAt           int64                       `json:"createdAt"`
	UpdatedAt           int64                       `json:"updatedAt"`
	ReferencingResource utils.WorkloadSingleRequest `json:"referencingResource"` // the resource that triggered this task
	TriggeredBy         AiFilter                    `json:"triggeredBy"`         // e.g., "Failed Pods" filter
	ReadByUser          *ReadBy                     `json:"readByUser,omitempty"`
	Error               error                       `json:"error,omitempty"`
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
	Name     string            `json:"name"`
	Kind     string            `json:"kind"`
	Contains map[string]string `json:"contains"` // {"Running": "status.phase"}, {"ImagePullBackOff": "status.phase.ContainerStatuses.state.waiting.reason"}
	Excludes map[string]string `json:"excludes"` // {"Succeeded": "status.phase"}, {"Completed": "status.phase"}
	Prompt   string            `json:"prompt"`
}

type AiPromptConfig struct {
	Name         string     `json:"name"`
	SystemPrompt string     `json:"systemPrompt"`
	Filters      []AiFilter `json:"filters"`
}

type AiManagerStatus struct {
	TokenLimit                  int64  `json:"tokenLimit"`
	TokensUsed                  int64  `json:"tokensUsed"`
	Model                       string `json:"model"`
	ApiUrl                      string `json:"apiUrl"`
	IsAiPromptConfigInitialized bool   `json:"isAiPromptConfigInitialized"`
	IsAiModelConfigInitialized  bool   `json:"isAiModelConfigInitialized"`
	DbEntries                   int    `json:"dbEntries"`
	Error                       string `json:"error,omitempty"`
	Warning                     string `json:"warning,omitempty"`
}

type AiResponse struct {
	ErrorMessage string   `json:"errorMessage"`
	Analysis     Analysis `json:"analysis"`
}

type Analysis struct {
	ProblemDescription  string             `json:"problemDescription"`
	PossibleCauses      []string           `json:"possibleCauses"`
	ProposedSolutions   []Solution         `json:"proposedSolutions"`
	AdditionalInfo      string             `json:"additionalInformation"`
	NeedsFollowUp       bool               `json:"needsFollowUp"`
	FollowUpResources   []FollowUpResource `json:"followUpResources"`
	KubectlApplyFixYaml string             `json:"kubectlApplyFixYaml"`
	DiffYaml            string             `json:"diffYaml"`
}

type Solution struct {
	SolutionDescription string   `json:"solutionDescription"`
	Steps               []string `json:"steps"`
}

type FollowUpResource struct {
	Kind       string `json:"kind"`
	Plural     string `json:"plural"`
	APIVersion string `json:"apiVersion"`
	Namespaced bool   `json:"namespaced"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
}

var AiFilters = []AiFilter{
	{
		Name: "Failed Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"Failed": ".status.phase",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod failed and suggest possible solutions.",
	},
	{
		Name: "CrashLoopBackOff Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"CrashLoopBackOff": ".status.containerStatuses[*].state.waiting.reason",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is in CrashLoopBackOff and suggest possible solutions.",
	},
	{
		Name: "ImagePullBackOff Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"ImagePullBackOff": ".status.containerStatuses[*].state.waiting.reason",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is in ImagePullBackOff and suggest possible solutions.",
	},
	{
		Name: "ErrImagePull Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"ErrImagePull": ".status.containerStatuses[*].state.waiting.reason",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod cannot pull its image and suggest possible solutions.",
	},
	{
		Name: "CreateContainerConfigError Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"CreateContainerConfigError": ".status.containerStatuses[*].state.waiting.reason",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod has a CreateContainerConfigError (likely ConfigMap or Secret issue) and suggest possible solutions.",
	},
	{
		Name: "InvalidImageName Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"InvalidImageName": ".status.containerStatuses[*].state.waiting.reason",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod has an invalid image name and suggest possible solutions.",
	},
	{
		Name: "ReplicaSet with unavailable replicas",
		Kind: "ReplicaSet",
		Contains: map[string]string{
			"0": ".status.replicas",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this ReplicaSet has unavailable replicas and suggest possible solutions.",
	},
	{
		Name: "PodNotReady",
		Kind: "Pod",
		Contains: map[string]string{
			"False": ".status.conditions[?(@.type=='Ready')].status",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is NotReady and suggest possible solutions.",
	},
	{
		Name: "Pending Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"Pending": ".status.phase",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is stuck in Pending state (likely scheduling issues, resource constraints, or PVC problems) and suggest possible solutions.",
	},
	{
		Name: "OOMKilled Containers",
		Kind: "Pod",
		Contains: map[string]string{
			"OOMKilled": ".status.containerStatuses[*].lastState.terminated.reason",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod's container was OOMKilled (Out of Memory) and suggest possible solutions including memory limit adjustments.",
	},
	{
		Name: "Deployment with unavailable replicas",
		Kind: "Deployment",
		Contains: map[string]string{
			"False": ".status.conditions[?(@.type=='Available')].status",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Deployment has unavailable replicas and suggest possible solutions.",
	},
	{
		Name: "StatefulSet with failed replicas",
		Kind: "StatefulSet",
		Contains: map[string]string{
			"False": ".status.conditions[?(@.type=='Ready')].status",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this StatefulSet has failed replicas and suggest possible solutions.",
	},
	{
		Name: "PVC Pending",
		Kind: "PersistentVolumeClaim",
		Contains: map[string]string{
			"Pending": ".status.phase",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this PersistentVolumeClaim is Pending (likely no matching PV or StorageClass issues) and suggest possible solutions.",
	},
	{
		Name:     "Service with no endpoints",
		Kind:     "Service",
		Contains: map[string]string{},
		Excludes: map[string]string{},
		Prompt:   "Check if this Service has endpoints. If not, provide a detailed analysis of why (likely selector mismatch or no ready Pods) and suggest possible solutions.",
	},
	{
		Name: "Unschedulable Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"Unschedulable": ".status.conditions[?(@.type=='PodScheduled')].reason",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is unschedulable (likely resource constraints, node affinity, or taints) and suggest possible solutions.",
	},
	{
		Name: "Jobs that failed",
		Kind: "Job",
		Contains: map[string]string{
			"False": ".status.conditions[?(@.type=='Complete')].status",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Job failed to complete and suggest possible solutions.",
	},
	{
		Name: "HPA unable to scale",
		Kind: "HorizontalPodAutoscaler",
		Contains: map[string]string{
			"False": ".status.conditions[?(@.type=='AbleToScale')].status",
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
}

type aiManager struct {
	logger         *slog.Logger
	valkeyClient   valkeyclient.ValkeyClient
	config         cfg.ConfigModule
	aiPromptConfig *AiPromptConfig
	error          string
	warning        string
}

func NewAiManager(logger *slog.Logger, valkeyClient valkeyclient.ValkeyClient, config cfg.ConfigModule) AiManager {
	self := &aiManager{}

	self.logger = logger
	self.valkeyClient = valkeyClient
	self.config = config

	return self
}

func (ai *aiManager) ProcessObject(obj *unstructured.Unstructured, eventType string, resource utils.ResourceDescriptor) {
	if obj == nil {
		return
	}

	if eventType == "delete" {
		// On delete, we try to remove any existing AI tasks for this object
		key := getValkeyKey(obj.GetKind(), obj.GetNamespace(), obj.GetName(), "*")
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
			for expectedValue, path := range filter.Contains {
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
			for expectedValue, path := range filter.Excludes {
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
					Error:       nil,
				}

				key := getValkeyKey(obj.GetKind(), obj.GetNamespace(), obj.GetName(), filter.Name)
				shouldCreate, err := ai.shouldCreateNewTask(key)
				if err != nil {
					ai.logger.Error("Error checking if should create new AI task", "error", err)
					continue
				}

				if shouldCreate {
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

			tokenLimit, err := ai.getDailyTokenLimit()
			if err != nil {
				ai.logger.Error("Error getting daily token limit", "error", err)
				ai.error = err.Error()
				continue
			}

			tokensUsed, _, err := ai.getTodayTokenUsage()
			if err != nil {
				ai.logger.Error("Error getting today's token usage", "error", err)
				ai.error = err.Error()
				continue
			}

			if tokensUsed >= tokenLimit {
				ai.logger.Warn("Daily AI token limit reached, skipping AI task processing", "tokensUsed", tokensUsed, "dailyLimit", tokenLimit)
				ai.error = fmt.Errorf("daily AI token limit reached (%d tokens used of %d)", tokensUsed, tokenLimit).Error()
				continue
			} else if tokensUsed >= int64(float64(tokenLimit)*0.8) {
				// warn at 80%
				ai.logger.Warn("Approaching daily AI token limit", "tokensUsed", tokensUsed, "dailyLimit", tokenLimit)
				ai.warning = fmt.Sprintf("approaching daily AI token limit (%d tokens used of %d)", tokensUsed, tokenLimit)
			} else {
				ai.warning = ""
			}

			ai.error = ""
			ai.processAiTaskQueue(ctx)
		}
	}()
}

func (ai *aiManager) getTodayTokenUsage() (todaysTokens int64, aiTaskDbEntries int, err error) {
	// Calculate the start of today in Unix timestamp
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	keys, err := ai.valkeyClient.Keys(DB_AI_BUCKET_TASKS + ":*")
	if err != nil {
		return 0, 0, err
	}

	var totalTokens int64 = 0
	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			return -1, -1, err
		}
		var task AiTask
		err = json.Unmarshal([]byte(item), &task)
		if err != nil {
			return -1, -1, err
		}

		if task.UpdatedAt >= startOfDay {
			totalTokens += task.TokensUsed
		}
	}

	return totalTokens, len(keys), nil
}

func (ai *aiManager) resetTodayTokenUsage() error {
	// Calculate the start of today in Unix timestamp
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	keys, err := ai.valkeyClient.Keys(DB_AI_BUCKET_TASKS + ":*")
	if err != nil {
		return err
	}

	var resettedTokens int64 = 0
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
		if task.UpdatedAt >= startOfDay {
			resettedTokens += task.TokensUsed
			task.TokensUsed = 0
			err = ai.createOrUpdateAiTask(&task, key)
			if err != nil {
				return err
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

		response, tokensUsed, err := ai.processPrompt(ctx, task.Prompt)
		if err != nil {
			task.Error = err
			task.State = AI_TASK_STATE_FAILED
			task.TokensUsed = tokensUsed
			ai.logger.Error("Error processing AI task", "taskID", task.ID, "error", err)
		} else {
			task.State = AI_TASK_STATE_COMPLETED
			task.Response = response
			task.TokensUsed = tokensUsed
		}

		// Save updated task
		err = ai.createOrUpdateAiTask(&task, key)
		if err != nil {
			ai.logger.Error("Error updating AI task", "taskID", task.ID, "error", err)
			continue
		}
		ai.logger.Info("AI task processed", "taskID", task.ID, "tokensUsed", task.TokensUsed)
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
	client, err := ai.getOpenAIClient()
	if err != nil {
		return nil, 0, err
	}

	model, err := ai.getAiModel()
	if err != nil {
		return nil, 0, err
	}
	systemPrompt := ai.getSystemPrompt()

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
		return nil, tokensUsed, fmt.Errorf("error unmarshaling AI response: %v", err)
	}

	// also return tokens used
	return &aiResponse, tokensUsed, nil
}

func buildUserPrompt(prompt string, obj *unstructured.Unstructured) string {
	objJsonBytes, err := json.MarshalIndent(obj.Object, "", "  ")
	if err != nil {
		return fmt.Sprintf("%s\n\nError serializing Kubernetes object: %v", prompt, err)
	}
	objJson := string(objJsonBytes)

	return fmt.Sprintf("%s\n\nHere is the Kubernetes object in JSON format:\n%s", prompt, objJson)
}

func (ai *aiManager) getOpenAIClient() (*openai.Client, error) {

	apiKey, err := ai.getApiKey()
	if err != nil {
		return nil, err
	}
	baseUrl, err := ai.getBaseUrl()
	if err != nil {
		return nil, err
	}
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseUrl),
	)
	return &client, nil
}

func getValkeyKey(kind, namespace, name, filter string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", DB_AI_BUCKET_TASKS, kind, namespace, name, filter)
}
