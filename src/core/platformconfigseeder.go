package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/utils"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// DEFAULT_PLATFORM_CONFIG_NAME is the cluster-scoped PlatformConfig the platform
// reads the GitOps status from. The name is a convention shared with the API.
const DEFAULT_PLATFORM_CONFIG_NAME = "platform"

// PLATFORM_CONFIG_BOOTSTRAPPED_AT_ANNOTATION records when the operator created
// the default PlatformConfig, so an object that came from the seeder can be told
// apart from one a user or a GitOps engine put there.
const PLATFORM_CONFIG_BOOTSTRAPPED_AT_ANNOTATION = "mogenius.com/bootstrapped-at"

// platformConfigSeedRetrySchedule is the wait before each attempt after the
// first. The PlatformConfig CRD is applied moments earlier in the same startup,
// and the API server serves a freshly created CRD only once it is Established —
// a Get or Create in that window fails with NotFound. A one-shot seeder that
// lost this race left the cluster without a PlatformConfig until the next
// leadership change, and the platform's onboarding cannot proceed without the
// object. The schedule rides out CRD establishment and short API server blips;
// anything longer is a real problem the next leadership change retries.
var platformConfigSeedRetrySchedule = []time.Duration{
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	15 * time.Second,
	30 * time.Second,
	30 * time.Second,
}

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
// object the operator simply reports no status. Failed attempts are retried
// with backoff (see platformConfigSeedRetrySchedule) before giving up until
// the next leadership change.
func EnsureDefaultPlatformConfig(logger *slog.Logger, clientProvider k8sclient.K8sClientProvider) {
	client := clientProvider.DynamicClient().Resource(
		kubernetes.CreateGroupVersionResource(utils.PlatformConfigResource.ApiVersion, utils.PlatformConfigResource.Plural),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shutdown.Add(cancel)

	err := seedDefaultPlatformConfig(ctx, logger, client)
	for attempt := 0; err != nil && attempt < len(platformConfigSeedRetrySchedule); attempt++ {
		logger.Warn("ensure default platform config: attempt failed, retrying",
			"attempt", attempt+1, "backoff", platformConfigSeedRetrySchedule[attempt].String(), "error", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(platformConfigSeedRetrySchedule[attempt]):
		}

		err = seedDefaultPlatformConfig(ctx, logger, client)
	}
	if err != nil {
		logger.Error("ensure default platform config: giving up until the next leadership change", "error", err)
	}
}

// seedDefaultPlatformConfig is one attempt: create the default object when none
// exists. Nil when the object exists afterwards, an error when the attempt is
// worth retrying — a NotFound from Create means the CRD is not served yet, and
// any other failure is an API server hiccup the next attempt may not see.
func seedDefaultPlatformConfig(ctx context.Context, logger *slog.Logger, client dynamic.NamespaceableResourceInterface) error {
	_, err := client.Get(ctx, DEFAULT_PLATFORM_CONFIG_NAME, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	// NotFound covers both the missing object and the not-yet-served CRD; the
	// Create below tells them apart by succeeding or failing.
	if !errors.IsNotFound(err) {
		return fmt.Errorf("read platform config: %w", err)
	}

	if _, err := client.Create(ctx, defaultPlatformConfig(time.Now()), metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create platform config: %w", err)
	}

	logger.Info("created default platform config", "name", DEFAULT_PLATFORM_CONFIG_NAME)
	return nil
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
