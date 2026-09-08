package core

import (
	"context"
	"log/slog"

	"mogenius-operator/src/gitops"
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

// PlatformBootstrap is the repository the operator was installed with, from
// MO_PLATFORM_BOOTSTRAP_*.
//
// It exists to break one circle: the GitOps engine has to be pointed at the
// repository before the PlatformConfig can arrive from it, and the config
// cannot declare its own location before anything has read it. Passing the
// location at install time seeds it into the resource the operator creates
// anyway, and reconciling that entry is what pulls the real config in — which
// then declares the same repository and takes over.
type PlatformBootstrap struct {
	RepositoryURL string
	Branch        string
	Path          string
	Engine        string
}

func (b PlatformBootstrap) configured() bool {
	return b.RepositoryURL != "" && b.Path != ""
}

// gitOpsSpec is the seed's spec.gitOps: the engine declared but not enabled,
// and the repository it should sync.
//
// Not enabled, because Helm installed the engine — enabling it would have the
// operator install a second one next to that release. Declaring it is still
// what tells the platform which engine this cluster runs.
func (b PlatformBootstrap) gitOpsSpec() map[string]any {
	engineBlock := map[string]any{"enabled": false}

	gitOps := map[string]any{}
	if b.Engine == gitops.EngineFlux {
		gitOps["fluxcd"] = engineBlock
	} else {
		gitOps["argocd"] = engineBlock
	}

	gitOps["repositories"] = []any{
		map[string]any{
			"name": DEFAULT_PLATFORM_CONFIG_NAME,
			"url":  b.RepositoryURL,
			// The CRD calls it revision; it is the branch the source tracks.
			"revision": b.Branch,
			"path":     b.Path,
		},
	}

	return gitOps
}

// EnsureDefaultPlatformConfig creates a default PlatformConfig if none exists,
// so the operator always has an object to publish the detected GitOps status
// on, and — when the operator was installed with a bootstrap repository — to
// carry that repository until the synced config replaces it.
//
// Only on create. Once the GitOps engine syncs the real config, that is the
// authority, and re-seeding would fight it on every restart.
//
// Meant to run on the leader — concurrent replicas would only race into
// AlreadyExists errors, which are tolerated anyway. Non-fatal: without the
// object the operator simply reports no status.
func EnsureDefaultPlatformConfig(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider, bootstrap PlatformBootstrap) {
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

	spec := map[string]any{}
	if bootstrap.configured() {
		spec["gitOps"] = bootstrap.gitOpsSpec()
	}

	platformConfig := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": utils.PlatformConfigResource.ApiVersion,
		"kind":       utils.PlatformConfigResource.Kind,
		"metadata":   map[string]any{"name": DEFAULT_PLATFORM_CONFIG_NAME},
		"spec":       spec,
	}}

	if _, err := client.Create(context.Background(), platformConfig, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return
		}
		logger.Error("ensure default platform config: failed to create platform config", "error", err)
		return
	}

	logger.Info("created default platform config", "name", DEFAULT_PLATFORM_CONFIG_NAME,
		"bootstrapRepository", bootstrap.RepositoryURL, "bootstrapPath", bootstrap.Path)
}
