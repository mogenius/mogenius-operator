package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	DB_AI_BUCKET_TASKS = "ai_tasks"
)

type AiTaskState string
type AiTask struct {
	ID         string      `json:"id"`
	Prompt     string      `json:"prompt"`
	Response   string      `json:"response"`
	State      AiTaskState `json:"state"` // pending, in-progress, completed, failed, ignored
	TokensUsed int64       `json:"tokensUsed"`
	CreatedAt  int64       `json:"createdAt"`
	UpdatedAt  int64       `json:"updatedAt"`
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

var AiFilters = []AiFilter{
	{
		Name: "Failed Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"Failed": "status.phase",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod failed and suggest possible solutions.",
	},
	{
		Name: "CrashLoopBackOff Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"CrashLoopBackOff": "status.containerStatuses.0.state.waiting.reason",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is in CrashLoopBackOff and suggest possible solutions.",
	},
	{
		Name: "ImagePullBackOff Pods",
		Kind: "Pod",
		Contains: map[string]string{
			"ImagePullBackOff": "status.containerStatuses.0.state.waiting.reason",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this Pod is in ImagePullBackOff and suggest possible solutions.",
	},
	{
		Name: "ReplicaSet with unavailable replicas",
		Kind: "ReplicaSet",
		Contains: map[string]string{
			"0": "status.Replicas",
		},
		Excludes: map[string]string{},
		Prompt:   "Provide a detailed analysis of why this ReplicaSet has unavailable replicas and suggest possible solutions.",
	},
	{
		Name: "Furztest",
		Kind: "Deployment",
		Contains: map[string]string{
			"lala": "metadata.Labels",
		},
		Excludes: map[string]string{},
		Prompt:   "XXX",
	},
}

type AiManager interface {
	ProcessObject(obj *unstructured.Unstructured, eventType string) // eventType can be "add", "update", "delete"
	Run()
	UpdateTaskState(taskID string, newState AiTaskState) error
	GetAiTasksForWorkspace(workspace string) ([]AiTask, error)
	InjectAiPromptConfig(prompt AiPromptConfig)
}

type aiManager struct {
	logger         *slog.Logger
	valkeyClient   valkeyclient.ValkeyClient
	config         cfg.ConfigModule
	aiPromptConfig *AiPromptConfig
}

func NewAiManager(logger *slog.Logger, valkeyClient valkeyclient.ValkeyClient, config cfg.ConfigModule) AiManager {
	self := &aiManager{}

	self.logger = logger
	self.valkeyClient = valkeyClient
	self.config = config

	return self
}

func (ai *aiManager) ProcessObject(obj *unstructured.Unstructured, eventType string) {
	if obj == nil {
		return
	}

	initialized := ai.IsAiPromptConfigInitialized()
	if !initialized {
		return
	}

	filters := ai.getAiFilters()

	for _, filter := range filters {
		if obj.GetKind() == filter.Kind {
			matches := false
			// check contains conditions
			for expectedValue, path := range filter.Contains {
				value, found, err := getNestedStringWithArrays(obj, path, expectedValue)
				if err != nil {
					ai.logger.Error("Error checking AI filter contains", "expectedValue", expectedValue, "error", err)
					continue
				}
				if found && value == expectedValue {
					matches = true
					break
				}
			}
			// check excludes conditions
			for expectedValue, path := range filter.Excludes {
				value, found, err := getNestedStringWithArrays(obj, path, expectedValue)
				if err != nil {
					ai.logger.Error("Error checking AI filter excludes", "expectedValue", expectedValue, "error", err)
					continue
				}
				if found && value == expectedValue {
					matches = false
					break
				}
			}
			if matches {
				timestamp := time.Now().Unix()
				// create AI task
				task := &AiTask{
					ID:        utils.NanoIdSmallLowerCase(),
					Prompt:    buildUserPrompt(filter.Prompt, obj),
					State:     AI_TASK_STATE_PENDING,
					CreatedAt: timestamp,
					UpdatedAt: timestamp,
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
						ai.logger.Info("AI task created", "taskID", task.ID, "objectKind", obj.GetKind(), "objectName", obj.GetName(), "objectNamespace", obj.GetNamespace())
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

			initialized := ai.IsAiPromptConfigInitialized()
			if !initialized {
				continue
			}

			tokenLimit, err := ai.getDailyTokenLimit()
			if err != nil {
				ai.logger.Error("Error getting daily token limit", "error", err)
				continue
			}

			tokens, err := ai.getTodayTokenUsage()
			if err != nil {
				ai.logger.Error("Error getting today's token usage", "error", err)
				continue
			}

			if tokens >= tokenLimit {
				ai.logger.Warn("Daily AI token limit reached, skipping AI task processing", "tokensUsed", tokens, "dailyLimit", tokenLimit)
				continue
			} else if tokens >= int64(float64(tokenLimit)*0.8) {
				// warn at 80%
				// TOD: i guess we should not log this
				ai.logger.Warn("Approaching daily AI token limit", "tokensUsed", tokens, "dailyLimit", tokenLimit)
			}

			ai.processAiTaskQueue(ctx)
		}
	}()
}

func (ai *aiManager) getTodayTokenUsage() (int64, error) {
	// Calculate the start of today in Unix timestamp
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	keys, err := ai.valkeyClient.Keys(DB_AI_BUCKET_TASKS)
	if err != nil {
		return 0, err
	}

	var totalTokens int64 = 0
	for _, key := range keys {
		item, err := ai.valkeyClient.Get(key)
		if err != nil {
			return -1, err
		}
		var task AiTask
		err = json.Unmarshal([]byte(item), &task)
		if err != nil {
			return -1, err
		}

		if task.UpdatedAt >= startOfDay {
			totalTokens += task.TokensUsed
		}
	}

	return totalTokens, nil
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
			task.State = AI_TASK_STATE_FAILED
			task.Response = fmt.Sprintf("Error processing AI task: %v", err)
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
func getNestedStringWithArrays(obj *unstructured.Unstructured, path string, keyword string) (string, bool, error) {
	parts := splitPath(path)

	var currentObj any = obj.Object

	for i := range parts {
		part := parts[i]

		// Check if this part is an array index
		if idx, err := strconv.Atoi(part); err == nil {
			// Current object should be a slice
			slice, ok := currentObj.([]any)
			if !ok {
				return "", false, fmt.Errorf("expected array but got %T", currentObj)
			}

			if idx >= len(slice) || idx < 0 {
				return "", false, nil
			}

			currentObj = slice[idx]
			continue
		}

		// Current object should be a map
		m, ok := currentObj.(map[string]any)
		if !ok {
			return "", false, fmt.Errorf("expected object but got %T at part: %s", currentObj, part)
		}

		val, found := m[part]
		if !found {
			return "", false, nil
		}

		currentObj = val
	}

	// Check if final value is a map (like labels) and we need to search it
	if labelMap, ok := currentObj.(map[string]any); ok {
		// Search through the map for keyword matches
		var matches []string
		for key, value := range labelMap {
			valueStr := fmt.Sprintf("%v", value)
			if strings.Contains(strings.ToLower(key), strings.ToLower(keyword)) ||
				strings.Contains(strings.ToLower(valueStr), strings.ToLower(keyword)) {
				matches = append(matches, fmt.Sprintf("%s=%s", key, valueStr))
			}
		}

		if len(matches) > 0 {
			// Sort for consistent output
			sort.Strings(matches)
			return strings.Join(matches, ", "), true, nil
		}
		return "", false, nil
	}

	// Final value should be a string (original functionality)
	str, ok := currentObj.(string)
	if !ok {
		return "", false, fmt.Errorf("final value is not a string or map, got %T", currentObj)
	}

	return str, true, nil
}

func splitPath(path string) []string {
	var result []string
	current := ""
	for _, char := range path {
		if char == '.' {
			result = append(result, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
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
	if !exists {
		return true, nil
	}

	// Task does already exists, check its state
	existingItem, err := ai.valkeyClient.Get(key)
	if err != nil {
		return false, fmt.Errorf("error getting existing AI task: %v", err)
	}
	var existingTask AiTask
	err = json.Unmarshal([]byte(existingItem), &existingTask)
	if err != nil {
		return false, fmt.Errorf("error unmarshaling existing AI task: %v", err)
	}
	// If existing task is not completed, skip creating a new one
	if existingTask.State == AI_TASK_STATE_PENDING || existingTask.State == AI_TASK_STATE_IN_PROGRESS {
		return false, nil
	}

	err = ai.valkeyClient.DeleteSingle(key)
	if err != nil {
		return false, fmt.Errorf("error deleting existing AI task: %v", err)
	}
	return true, nil
}

func (ai *aiManager) processPrompt(ctx context.Context, prompt string) (string, int64, error) {
	client, err := ai.getOpenAIClient()
	if err != nil {
		return "", 0, err
	}

	model, err := ai.getAiModel()
	if err != nil {
		return "", 0, err
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
		return "", tokensUsed, err
	}
	if len(chatCompletion.Choices) == 0 {
		return "", tokensUsed, fmt.Errorf("no choices returned from AI model")
	}

	// also return tokens used
	return chatCompletion.Choices[0].Message.Content, tokensUsed, nil
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
