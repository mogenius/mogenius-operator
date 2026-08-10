package core

import (
	"context"
	"log/slog"

	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/utils"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DEFAULT_PLATFORM_CONFIG_NAME is the cluster-scoped PlatformConfig the platform
// reads the GitOps status from. The name is a convention shared with the API.
const DEFAULT_PLATFORM_CONFIG_NAME = "platform"

// EnsureDefaultPlatformConfig creates an empty default PlatformConfig if none
// exists, so the operator always has an object to publish the detected GitOps
// status on. Meant to run on the leader — concurrent replicas would only race
// into AlreadyExists errors, which are tolerated anyway. Non-fatal: without the
// object the operator simply reports no status.
func EnsureDefaultPlatformConfig(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider) {
	client := clientProvider.DynamicClient().Resource(
		kubernetes.CreateGroupVersionResource(utils.PlatformConfigResource.ApiVersion, utils.PlatformConfigResource.Plural),
	)

	_, err := client.Get(context.Background(), DEFAULT_PLATFORM_CONFIG_NAME, metav1.GetOptions{})
	if err == nil {
		return
	}
	if !errors.IsNotFound(err) {
		logger.Error("ensure default platform config: failed to read platform config", "error", err)
		return
	}

	platformConfig := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": utils.PlatformConfigResource.ApiVersion,
		"kind":       utils.PlatformConfigResource.Kind,
		"metadata":   map[string]any{"name": DEFAULT_PLATFORM_CONFIG_NAME},
		"spec":       map[string]any{},
	}}

	if _, err := client.Create(context.Background(), platformConfig, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return
		}
		logger.Error("ensure default platform config: failed to create platform config", "error", err)
		return
	}

	logger.Info("created default platform config", "name", DEFAULT_PLATFORM_CONFIG_NAME)
}
