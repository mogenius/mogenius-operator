package reconciler

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/store"
	"mogenius-operator/src/valkeyclient"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func GetNamespace(name string, valkeyClient *valkeyclient.ValkeyClient, logger *slog.Logger) (v1.Namespace, error) {

	objects := store.GetResourceByKindAndNamespace(*valkeyClient, "", "namespace", "", logger)
	if len(objects) == 0 {
		return v1.Namespace{}, fmt.Errorf("namespace not found: %s", name)
	}
	var namespace v1.Namespace
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(objects[0].Object, &namespace); err != nil {
		return v1.Namespace{}, fmt.Errorf("failed to parse Namespace: %w", err)
	}
	return namespace, nil
}
