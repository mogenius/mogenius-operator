package ai

import (
	"log/slog"
	"mogenius-k8s-manager/src/valkeyclient"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	DB_AI_BUCKET_TASKS   = "ai_tasks"
	AI_DAILY_TOKEN_LIMIT = 1000000
)

type AiTask struct {
	ID         string `json:"id"`
	Prompt     string `json:"prompt"`
	Response   string `json:"response"`
	State      string `json:"state"` // "pending", "in-progress", "completed", "failed"
	TokensUsed int    `json:"tokensUsed"`
	CreatedAt  int64  `json:"createdAt"`
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
	if obj.GetKind() == "Pod" {
		// make Pod from unstructured
		var pod v1.Pod
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pod)
		if err != nil {
			self.logger.Error("Error cannot cast from unstructured", "error", err)
			return
		}
		// only log if Pod is in Failed or Succeeded state
		if pod.Status.Phase == v1.PodFailed || pod.Status.Phase == v1.PodSucceeded {
			self.logger.Info("Pod event received", "name", obj.GetName(), "namespace", obj.GetNamespace(), "eventType", eventType, "phase", pod.Status.Phase)
		}
	}
	if obj.GetKind() == "Event" {
		self.logger.Info("Event received", "name", obj.GetName(), "namespace", obj.GetNamespace(), "eventType", eventType)
	}
}
