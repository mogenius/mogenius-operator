// Package openbaomanager runs the out-of-band lifecycle control loop for an
// operator-managed OpenBao instance. The OpenBao workload itself is installed
// declaratively through the GitOps engine (see reconciler.reconcileOpenBao);
// this manager takes over once the pods come up and drives OpenBao to the
// desired state: initialize once, auto-unseal on every restart, configure a KV
// v2 engine plus Kubernetes auth, and publish a ClusterSecretStore so the
// External Secrets Operator can read secrets from it.
//
// Every step is idempotent so the loop is safe to run on a short interval.
package openbaomanager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"mogenius-operator/src/config"
	"mogenius-operator/src/crds/v1alpha1"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/utils"

	bao "github.com/openbao/openbao/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// tickInterval is how often the loop drives OpenBao towards the desired
	// state. It must be short enough to re-unseal quickly after a pod restart.
	tickInterval = 20 * time.Second

	defaultNamespace       = "openbao"
	defaultRelease         = "openbao"
	defaultKVMount         = "secret"
	defaultSecretStoreName = "openbao"

	// keysSecretName is the Secret (in MO_OWN_NAMESPACE) holding the root token
	// and unseal keys generated at init time.
	keysSecretName = "openbao-keys"

	kubernetesAuthPath = "kubernetes"
	esoRoleName        = "eso"
	esoPolicyName      = "eso-read"

	// esoServiceAccountName / esoNamespace identify the External Secrets
	// Operator controller ServiceAccount that the Kubernetes auth role is bound
	// to. They match the release/namespace used by reconcileExternalSecretsOperator.
	esoServiceAccountName = "external-secrets-operator"
	esoNamespace          = "external-secrets-operator"

	// secretShares / secretThreshold control Shamir key splitting at init.
	secretShares    = 5
	secretThreshold = 3
)

// Module is the OpenBao lifecycle manager. Start/Stop are leader-gated by the
// caller so only one replica drives init/unseal at a time. The data operations
// (Status and the KV helpers) are safe to call from any request handler.
type Module interface {
	Start()
	Stop()

	Status(ctx context.Context) (string, error)
	ListKVMounts(ctx context.Context) ([]KVMount, error)
	ListSecrets(ctx context.Context, mount, path string) ([]string, error)
	GetSecret(ctx context.Context, mount, path string) (map[string]any, error)
	PutSecret(ctx context.Context, mount, path string, data map[string]any) error
	DeleteSecret(ctx context.Context, mount, path string) error
}

type openBaoManager struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider

	running atomic.Bool
	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewOpenBaoManager builds the manager. It does not start the control loop.
func NewOpenBaoManager(logger *slog.Logger, configModule config.ConfigModule, clientProvider k8sclient.K8sClientProvider) Module {
	return &openBaoManager{
		logger:         logger,
		config:         configModule,
		clientProvider: clientProvider,
	}
}

// Start launches the control loop. It is a no-op if already running or if the
// PlatformConfig feature gate is off (dev builds only, matching the reconciler).
func (m *openBaoManager) Start() {
	if !utils.IsDevBuild() {
		m.logger.Debug("OpenBao manager disabled outside dev builds")
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running.Swap(true) {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	m.wg.Go(func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		m.logger.Info("OpenBao manager started")
		for {
			select {
			case <-ctx.Done():
				m.logger.Info("OpenBao manager stopped")
				return
			case <-ticker.C:
				if err := m.reconcile(ctx); err != nil {
					m.logger.Warn("OpenBao reconcile failed", "error", err)
				}
			}
		}
	})
}

// Stop cancels the control loop and waits for it to exit.
func (m *openBaoManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running.Swap(false) {
		return
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	m.cancel = nil
}

// target holds the resolved coordinates of the OpenBao instance and the ESO
// integration derived from a PlatformConfig's OpenBaoConfig.
type target struct {
	namespace string
	release   string
	kvMount   string
	storeName string
	serverURL string
	// ownerRef is the parent PlatformConfig, set on every object the manager
	// creates so they are garbage-collected with it.
	ownerRef metav1.OwnerReference
}

// reconcile runs one pass: find an enabled OpenBao config, then init → unseal →
// configure → ensure the ClusterSecretStore. Unreachable OpenBao (pods not up
// yet) is not an error.
func (m *openBaoManager) reconcile(ctx context.Context) error {
	cfg, owner, err := m.enabledOpenBaoConfig(ctx)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil // OpenBao not enabled; nothing to do.
	}

	tgt := resolveTarget(cfg)
	tgt.ownerRef = owner

	client, err := newBaoClient(tgt.serverURL)
	if err != nil {
		return fmt.Errorf("create OpenBao client: %w", err)
	}

	status, err := client.Sys().SealStatusWithContext(ctx)
	if err != nil {
		// OpenBao is not reachable yet (chart still deploying / pod starting).
		m.logger.Debug("OpenBao not reachable yet", "server", tgt.serverURL, "error", err)
		return nil
	}

	if !status.Initialized {
		if err := m.initialize(ctx, client, tgt.ownerRef); err != nil {
			return fmt.Errorf("initialize OpenBao: %w", err)
		}
		// Re-read status so we unseal in the same pass.
		status, err = client.Sys().SealStatusWithContext(ctx)
		if err != nil {
			return fmt.Errorf("seal status after init: %w", err)
		}
	}

	rootToken, unsealKeys, err := m.readKeysSecret(ctx)
	if err != nil {
		return fmt.Errorf("read keys secret: %w", err)
	}
	if rootToken == "" {
		return fmt.Errorf("OpenBao is initialized but no keys secret %q found; cannot manage it", keysSecretName)
	}

	if status.Sealed {
		if err := m.unseal(ctx, client, unsealKeys); err != nil {
			return fmt.Errorf("unseal OpenBao: %w", err)
		}
	}

	client.SetToken(rootToken)

	if err := m.configure(ctx, client, tgt); err != nil {
		return fmt.Errorf("configure OpenBao: %w", err)
	}

	if err := m.ensureClusterSecretStore(ctx, tgt); err != nil {
		return fmt.Errorf("ensure ClusterSecretStore: %w", err)
	}

	return nil
}

// initialize runs sys/init and persists the resulting keys. It is only reached
// when OpenBao reports uninitialized, so any pre-existing keys secret refers to
// wiped storage and is intentionally overwritten.
func (m *openBaoManager) initialize(ctx context.Context, client *bao.Client, ownerRef metav1.OwnerReference) error {
	m.logger.Info("initializing OpenBao")
	resp, err := client.Sys().InitWithContext(ctx, &bao.InitRequest{
		SecretShares:    secretShares,
		SecretThreshold: secretThreshold,
	})
	if err != nil {
		return err
	}
	if err := m.writeKeysSecret(ctx, resp.RootToken, resp.KeysB64, ownerRef); err != nil {
		return fmt.Errorf("persist keys: %w", err)
	}
	m.logger.Info("OpenBao initialized and keys stored", "secret", keysSecretName)
	return nil
}

// unseal submits unseal shares until OpenBao reports unsealed.
func (m *openBaoManager) unseal(ctx context.Context, client *bao.Client, keys []string) error {
	if len(keys) == 0 {
		return fmt.Errorf("no unseal keys available")
	}
	m.logger.Info("unsealing OpenBao")
	for _, key := range keys {
		status, err := client.Sys().UnsealWithContext(ctx, key)
		if err != nil {
			return err
		}
		if !status.Sealed {
			m.logger.Info("OpenBao unsealed")
			return nil
		}
	}
	return fmt.Errorf("submitted %d unseal keys but OpenBao is still sealed", len(keys))
}

func resolveTarget(cfg *v1alpha1.OpenBaoConfig) target {
	namespace := defaultNamespace
	release := defaultRelease
	if cfg.Chart != nil {
		if cfg.Chart.Namespace != "" {
			namespace = cfg.Chart.Namespace
		}
		if cfg.Chart.Name != "" {
			release = cfg.Chart.Name
		}
	}
	kvMount := defaultKVMount
	if cfg.KVMount != "" {
		kvMount = cfg.KVMount
	}
	storeName := defaultSecretStoreName
	if cfg.SecretStoreName != "" {
		storeName = cfg.SecretStoreName
	}
	return target{
		namespace: namespace,
		release:   release,
		kvMount:   kvMount,
		storeName: storeName,
		serverURL: fmt.Sprintf("http://%s.%s.svc:8200", release, namespace),
	}
}

func newBaoClient(serverURL string) (*bao.Client, error) {
	config := bao.DefaultConfig()
	config.Address = serverURL
	return bao.NewClient(config)
}

// enabledOpenBaoConfig lists PlatformConfigs and returns the first OpenBaoConfig
// that is enabled together with an OwnerReference to its parent PlatformConfig,
// or a nil config when none is enabled. Absence of the CRD (non-dev clusters)
// is treated as "not enabled".
func (m *openBaoManager) enabledOpenBaoConfig(ctx context.Context) (*v1alpha1.OpenBaoConfig, metav1.OwnerReference, error) {
	gvr := gvrFor(utils.PlatformConfigResource)
	list, err := m.clientProvider.DynamicClient().Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		// CRD not installed or not permitted; nothing to manage.
		m.logger.Debug("list PlatformConfigs failed", "error", err)
		return nil, metav1.OwnerReference{}, nil
	}
	for i := range list.Items {
		var pc v1alpha1.PlatformConfig
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[i].Object, &pc); err != nil {
			m.logger.Warn("parse PlatformConfig failed", "name", list.Items[i].GetName(), "error", err)
			continue
		}
		if pc.Spec.OpenBao != nil && pc.Spec.OpenBao.Enabled {
			owner := metav1.OwnerReference{
				APIVersion: utils.PlatformConfigResource.ApiVersion,
				Kind:       utils.PlatformConfigResource.Kind,
				Name:       list.Items[i].GetName(),
				UID:        list.Items[i].GetUID(),
			}
			return pc.Spec.OpenBao, owner, nil
		}
	}
	return nil, metav1.OwnerReference{}, nil
}

// gvrFor derives a GroupVersionResource from a ResourceDescriptor whose
// ApiVersion is either "group/version" or a core "version".
func gvrFor(rd utils.ResourceDescriptor) schema.GroupVersionResource {
	gv, err := schema.ParseGroupVersion(rd.ApiVersion)
	if err != nil {
		gv = schema.GroupVersion{Version: rd.ApiVersion}
	}
	return gv.WithResource(rd.Plural)
}
