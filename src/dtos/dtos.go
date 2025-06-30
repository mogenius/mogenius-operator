package dtos

import (
	"log/slog"
	"mogenius-k8s-manager/src/logging"
)

var dtosLogger *slog.Logger

func Setup(logManagerModule logging.SlogManager) {
	dtosLogger = logManagerModule.CreateLogger("dtos")
}

// TODO: Bene remove this when project is removed
func NewK8sController(kind string, name string, namespace string) K8sController {
	return K8sController{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}
}

// TODO: Bene remove this when project is removed
type K8sController struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}
