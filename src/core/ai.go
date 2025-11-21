package core

import (
	"log/slog"
	"mogenius-k8s-manager/src/ai"
)

type AiApi interface {
	UpdateTaskState(taskID string, newState ai.AiTaskState) error
	GetAiTasksForWorkspace(workspace string) ([]ai.AiTask, error)
	InjectAiPromptConfig(prompt ai.AiPromptConfig)
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
