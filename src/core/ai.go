package core

import (
	"log/slog"
	"mogenius-operator/src/ai"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
)

type AiApi interface {
	UpdateTaskState(taskID string, newState ai.AiTaskState) error
	UpdateTaskReadState(taskID string, user *structs.User) error
	GetAiTasksForWorkspace(workspace string) ([]ai.AiTask, error)
	GetAiTasksForResource(resourceReq utils.WorkloadSingleRequest) ([]ai.AiTask, error)
	GetLatestTask(workspace string) (*ai.AiTaskLatest, error)
	InjectAiPromptConfig(prompt ai.AiPromptConfig)
	GetStatus() ai.AiManagerStatus
	ResetDailyTokenLimit() error
	DeleteAllAiData() error
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

func (ai *aiApi) GetAiTasksForWorkspace(workspace string) ([]ai.AiTask, error) {
	return ai.aiManager.GetAiTasksForWorkspace(workspace)
}
func (ai *aiApi) GetAiTasksForResource(resourceReq utils.WorkloadSingleRequest) ([]ai.AiTask, error) {
	return ai.aiManager.GetAiTasksForResource(resourceReq)
}

func (ai *aiApi) GetLatestTask(workspace string) (*ai.AiTaskLatest, error) {
	return ai.aiManager.GetLatestTask(workspace)
}

func (ai *aiApi) InjectAiPromptConfig(prompt ai.AiPromptConfig) {
	ai.aiManager.InjectAiPromptConfig(prompt)
}

func (ai *aiApi) UpdateTaskState(taskID string, newState ai.AiTaskState) error {
	return ai.aiManager.UpdateTaskState(taskID, newState)
}

func (ai *aiApi) UpdateTaskReadState(taskID string, user *structs.User) error {
	return ai.aiManager.UpdateTaskReadState(taskID, user)
}

func (ai *aiApi) GetStatus() ai.AiManagerStatus {
	return ai.aiManager.GetStatus()
}

func (ai *aiApi) ResetDailyTokenLimit() error {
	return ai.aiManager.ResetDailyTokenLimit()
}

func (ai *aiApi) DeleteAllAiData() error {
	return ai.aiManager.DeleteAllAiData()
}
