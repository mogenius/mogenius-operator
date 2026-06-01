package reconciler

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func GetNamespace(name string, valkeyClient *valkeyclient.ValkeyClient, logger *slog.Logger) (v1.Namespace, error) {

	object, err := store.GetResource(*valkeyClient, utils.NamespaceResource.ApiVersion, utils.NamespaceResource.Kind, "", name, logger)
	if err != nil {
		return v1.Namespace{}, fmt.Errorf("failed to get namespace: %w", err)
	}
	var namespace v1.Namespace
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.UnstructuredContent(), &namespace); err != nil {
		return v1.Namespace{}, fmt.Errorf("failed to parse Namespace: %w", err)
	}
	return namespace, nil
}
