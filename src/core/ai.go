package core

import (
	"log/slog"
	"mogenius-operator/src/ai"
	"mogenius-operator/src/structs"
)

type AiApi interface {
	UpdateTaskState(taskID string, newState ai.AiTaskState) error
	UpdateTaskReadState(taskID string, user *structs.User) error
	GetAiTasksForWorkspace(workspace string) ([]ai.AiTask, error)
	InjectAiPromptConfig(prompt ai.AiPromptConfig)
	GetStatus() ai.AiManagerStatus
	ResetDailyTokenLimit() error
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
