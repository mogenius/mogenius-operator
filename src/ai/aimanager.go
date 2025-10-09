package ai

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/valkeyclient"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	DB_AI_BUCKET_TASKS   = "ai_tasks"
	AI_DAILY_TOKEN_LIMIT = 1000000
)

type AiTask struct {
	ID         string `json:"id"`
	Prompt     string `json:"prompt"`
	Response   string `json:"response"`
	State      string `json:"state"` // pending, in-progress, completed, failed, ignored
	TokensUsed int    `json:"tokensUsed"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

// state enums
const (
	AI_TASK_STATE_PENDING     = "pending"
	AI_TASK_STATE_IN_PROGRESS = "in-progress"
	AI_TASK_STATE_COMPLETED   = "completed"
	AI_TASK_STATE_FAILED      = "failed"
	AI_TASK_STATE_IGNORED     = "ignored"
)

type AiFilter struct {
	Name     string            `json:"name"`
	Kind     string            `json:"kind"`
	Contains map[string]string `json:"contains"` // {"Running": "status.phase"}, {"ImagePullBackOff": "status.phase.ContainerStatuses.state.waiting.reason"}
	Excludes map[string]string `json:"excludes"` // {"Succeeded": "status.phase"}, {"Completed": "status.phase"}
	Prompt   string            `json:"prompt"`
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
}

type aiManager struct {
	logger       *slog.Logger
	valkeyClient valkeyclient.ValkeyClient
}

func NewAiManager(logger *slog.Logger, valkeyClient valkeyclient.ValkeyClient) AiManager {
	self := &aiManager{}

	self.logger = logger
	self.valkeyClient = valkeyClient

	return self
}

func (self *aiManager) ProcessObject(obj *unstructured.Unstructured, eventType string) {
	if obj == nil {
		return
	}

	for _, filter := range AiFilters {
		if obj.GetKind() == filter.Kind {
			matches := false
			// check contains conditions
			for expectedValue, path := range filter.Contains {
				value, found, err := getNestedStringWithArrays(obj, path, expectedValue)
				if err != nil {
					self.logger.Error("Error checking AI filter contains", "expectedValue", expectedValue, "error", err)
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
					self.logger.Error("Error checking AI filter excludes", "expectedValue", expectedValue, "error", err)
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
					Prompt:    filter.Prompt,
					State:     "pending",
					CreatedAt: timestamp,
					UpdatedAt: timestamp,
				}
				err := self.valkeyClient.StoreSortedListEntry(task, timestamp, DB_AI_BUCKET_TASKS, obj.GetKind(), obj.GetNamespace(), obj.GetName())
				if err != nil {
					self.logger.Error("Error creating AI task", "error", err)
				} else {
					self.logger.Info("AI task created", "taskID", task.ID, "objectKind", obj.GetKind(), "objectName", obj.GetName(), "objectNamespace", obj.GetNamespace())
				}
			}
		}
	}
}

// HELPER FUNCTIONS
func getNestedStringWithArrays(obj *unstructured.Unstructured, path string, keyword string) (string, bool, error) {
	parts := splitPath(path)

	var currentObj interface{} = obj.Object

	for i := 0; i < len(parts); i++ {
		part := parts[i]

		// Check if this part is an array index
		if idx, err := strconv.Atoi(part); err == nil {
			// Current object should be a slice
			slice, ok := currentObj.([]interface{})
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
		m, ok := currentObj.(map[string]interface{})
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
	if labelMap, ok := currentObj.(map[string]interface{}); ok {
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
