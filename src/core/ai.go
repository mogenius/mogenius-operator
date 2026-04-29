package core

import (
	"log/slog"
	"mogenius-operator/src/ai"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type AiApi interface {
	UpdateTaskState(taskID string, newState ai.AiTaskState) error
	UpdateTaskReadState(taskID string, user *structs.User) error
	GetAiTasksForWorkspace(workspace string) ([]ai.AiTask, error)
	GetAllAiTasks() ([]ai.AiTask, error)
	GetAiTasksForResource(resourceReq utils.WorkloadSingleRequest) ([]ai.AiTask, error)
	GetLatestTask(workspace *string) (*ai.AiTaskLatest, error)
	InjectAiPromptConfig(prompt ai.AiPromptConfig, aiPrompts *ai.AiPrompts)
	GetStatus(workspace *string) ai.AiManagerStatus
	ResetDailyTokenLimit() error
	DeleteAllAiData() error
	GetAvailableModels(request *ai.ModelsRequest) ([]string, error)
	GetPromptConfig() (*ai.AiPromptConfig, error)
	// HandleConfigMapChange reloads AI prompt filters from the given ConfigMap object.
	HandleConfigMapChange(obj *unstructured.Unstructured)
	// HandleConfigMapDelete clears the in-memory prompt config and purges
	// AI tasks/tokens from Valkey when the AI filters ConfigMap is deleted.
	HandleConfigMapDelete(obj *unstructured.Unstructured)
}
type aiApi struct {
	logger    *slog.Logger
	aiManager ai.AiManager
}

func NewAiApi(logger *slog.Logger, aiManager ai.AiManager) AiApi {
	self := &aiApi{}

	self.logger = logger
	self.aiManager = aiManager

	return self
}

func (ai *aiApi) GetAllAiTasks() ([]ai.AiTask, error) {
	return ai.aiManager.GetAllAiTasks()
}

func (ai *aiApi) GetAiTasksForWorkspace(workspace string) ([]ai.AiTask, error) {
	return ai.aiManager.GetAiTasksForWorkspace(workspace)
}
func (ai *aiApi) GetAiTasksForResource(resourceReq utils.WorkloadSingleRequest) ([]ai.AiTask, error) {
	return ai.aiManager.GetAiTasksForResource(resourceReq)
}

func (ai *aiApi) GetLatestTask(workspace *string) (*ai.AiTaskLatest, error) {
	return ai.aiManager.GetLatestTask(workspace)
}

func (ai *aiApi) InjectAiPromptConfig(prompt ai.AiPromptConfig, aiPrompts *ai.AiPrompts) {
	ai.aiManager.InjectAiPromptConfig(prompt, aiPrompts)
}

func (ai *aiApi) UpdateTaskState(taskID string, newState ai.AiTaskState) error {
	return ai.aiManager.UpdateTaskState(taskID, newState)
}

func (ai *aiApi) UpdateTaskReadState(taskID string, user *structs.User) error {
	return ai.aiManager.UpdateTaskReadState(taskID, user)
}

func (ai *aiApi) GetStatus(workspace *string) ai.AiManagerStatus {
	return ai.aiManager.GetStatus(workspace)
}

func (ai *aiApi) ResetDailyTokenLimit() error {
	return ai.aiManager.ResetDailyTokenLimit()
}

func (ai *aiApi) DeleteAllAiData() error {
	return ai.aiManager.DeleteAllAiData()
}

func (ai *aiApi) GetAvailableModels(request *ai.ModelsRequest) ([]string, error) {
	return ai.aiManager.GetAvailableModels(request)
}

func (ai *aiApi) GetPromptConfig() (*ai.AiPromptConfig, error) {
	return ai.aiManager.GetPromptConfig()
}

// HandleConfigMapChange reloads AI prompt filters from a ConfigMap's data field.
// Uses "self" as receiver to avoid shadowing the "ai" package import.
func (self *aiApi) HandleConfigMapChange(obj *unstructured.Unstructured) {
	self.logger.Info("AI filters ConfigMap changed, updating prompt config")

	data, found, err := unstructured.NestedStringMap(obj.Object, "data")
	if err != nil || !found {
		self.logger.Error("failed to read ConfigMap data", "error", err)
		return
	}

	var filters []ai.AiFilter
	if filtersYaml, ok := data["filters"]; ok {
		if err := yaml.Unmarshal([]byte(filtersYaml), &filters); err != nil {
			self.logger.Error("failed to unmarshal filters from ConfigMap", "error", err)
			return
		}
	}

	var userFilters []ai.AiFilter
	if userFiltersYaml, ok := data["userFilters"]; ok {
		if err := yaml.Unmarshal([]byte(userFiltersYaml), &userFilters); err != nil {
			self.logger.Error("failed to unmarshal userFilters from ConfigMap", "error", err)
			return
		}
	}

	existingConfig, err := self.aiManager.GetPromptConfig()
	var updatedConfig ai.AiPromptConfig
	if err == nil && existingConfig != nil {
		updatedConfig = *existingConfig
		updatedConfig.Filters = filters
		updatedConfig.UserFilters = userFilters
	} else {
		updatedConfig = ai.AiPromptConfig{
			Filters:     filters,
			UserFilters: userFilters,
		}
	}

	self.aiManager.InjectAiPromptConfig(updatedConfig, nil)
}

// HandleConfigMapDelete clears the in-memory AI prompt config and purges all
// AI-related Valkey entries (tasks, tokens, latest tasks). Triggered when the
// AI filters ConfigMap is deleted, which represents a feature deactivation.
// Without this cleanup, stale AiTasks would linger until their 7-day TTL.
func (self *aiApi) HandleConfigMapDelete(_ *unstructured.Unstructured) {
	self.logger.Info("AI filters ConfigMap deleted — clearing prompt config and purging stored AI data")
	self.aiManager.InjectAiPromptConfig(ai.AiPromptConfig{}, nil)
	if err := self.aiManager.DeleteAllAiData(); err != nil {
		self.logger.Error("failed to delete AI data after ConfigMap deletion", "error", err)
	}
}
