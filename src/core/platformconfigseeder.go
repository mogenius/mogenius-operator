package core

import (
	"context"
	"log/slog"
	"time"

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

// PLATFORM_CONFIG_BOOTSTRAPPED_AT_ANNOTATION records when the operator created
// the default PlatformConfig, so an object that came from the seeder can be told
// apart from one a user or a GitOps engine put there.
const PLATFORM_CONFIG_BOOTSTRAPPED_AT_ANNOTATION = "mogenius.com/bootstrapped-at"

// EnsureDefaultPlatformConfig creates a default PlatformConfig if none exists,
// so the operator always has an object to publish the detected GitOps status on.
//
// The spec is always empty. An empty spec declares no repository and no engine,
// which is what makes creating it safe: it only carries status. The repository
// is connected through the mogenius UI, and once the GitOps engine syncs the
// real config that file is the authority.
//
// Only on create — re-seeding would fight the synced config on every restart.
//
// Meant to run on the leader — concurrent replicas would only race into
// AlreadyExists errors, which are tolerated anyway. Non-fatal: without the
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

	if _, err := client.Create(context.Background(), defaultPlatformConfig(time.Now()), metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return
		}
		logger.Error("ensure default platform config: failed to create platform config", "error", err)
		return
	}

	logger.Info("created default platform config", "name", DEFAULT_PLATFORM_CONFIG_NAME)
}

// defaultPlatformConfig is the object the seeder creates: an empty spec, and the
// bootstrapped-at annotation saying the operator is where it came from.
func defaultPlatformConfig(createdAt time.Time) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": utils.PlatformConfigResource.ApiVersion,
		"kind":       utils.PlatformConfigResource.Kind,
		"metadata": map[string]any{
			"name": DEFAULT_PLATFORM_CONFIG_NAME,
			"annotations": map[string]any{
				PLATFORM_CONFIG_BOOTSTRAPPED_AT_ANNOTATION: createdAt.UTC().Format(time.RFC3339),
			},
		},
		"spec": map[string]any{},
	}}
}
