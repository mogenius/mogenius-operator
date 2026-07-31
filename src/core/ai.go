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
	GetAllAiTasks() ([]ai.AiTask, error)
	GetAiTasksForResource(resourceReq utils.WorkloadSingleRequest) ([]ai.AiTask, error)
	GetLatestTask(workspace *string) (*ai.AiTaskLatest, error)
	GetRun(runID string) (*ai.AiRun, error)
	InjectAiPromptConfig(prompt ai.AiPromptConfig, aiPrompts *ai.AiPrompts)
	GetStatus(workspace *string) ai.AiManagerStatus
	DeleteAllAiData() error
	GetAvailableModels(request *ai.ModelsRequest) ([]string, error)
	TestAiModel(name string) (*ai.AiModelTestResult, error)
	GetPromptConfig() (*ai.AiPromptConfig, error)

	ApproveTask(taskID string, user structs.User, workspace string) (*ai.AiTask, error)
	RejectTask(taskID string, user structs.User, reason string) (*ai.AiTask, error)
	CancelTask(taskID string, user structs.User) (*ai.AiTask, error)
	DeleteTask(taskID string, user structs.User) (*ai.AiTask, error)
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

func (self *aiApi) GetRun(runID string) (*ai.AiRun, error) {
	return self.aiManager.GetRun(runID)
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

func (ai *aiApi) DeleteAllAiData() error {
	return ai.aiManager.DeleteAllAiData()
}

func (ai *aiApi) GetAvailableModels(request *ai.ModelsRequest) ([]string, error) {
	return ai.aiManager.GetAvailableModels(request)
}

func (self *aiApi) TestAiModel(name string) (*ai.AiModelTestResult, error) {
	return self.aiManager.TestAiModel(name)
}

func (ai *aiApi) GetPromptConfig() (*ai.AiPromptConfig, error) {
	return ai.aiManager.GetPromptConfig()
}

func (self *aiApi) ApproveTask(taskID string, user structs.User, workspace string) (*ai.AiTask, error) {
	return self.aiManager.ApproveTask(taskID, user, workspace)
}

func (self *aiApi) RejectTask(taskID string, user structs.User, reason string) (*ai.AiTask, error) {
	return self.aiManager.RejectTask(taskID, user, reason)
}

func (self *aiApi) CancelTask(taskID string, user structs.User) (*ai.AiTask, error) {
	return self.aiManager.CancelTask(taskID, user)
}

func (self *aiApi) DeleteTask(taskID string, user structs.User) (*ai.AiTask, error) {
	return self.aiManager.DeleteTask(taskID, user)
}

