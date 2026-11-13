package nodemetrics

import (
	"context"
	"fmt"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/bootstrap"
	"mogenius-operator/src/config"
	"mogenius-operator/src/containerenumerator"
	"mogenius-operator/src/cpumonitor"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/networkmonitor"
	"mogenius-operator/src/rammonitor"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/statsdb"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/valkeyclient"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// valkeyConnectMaxWait bounds how long node-metrics waits for Valkey at
// startup before it exits.
const valkeyConnectMaxWait = 3 * time.Minute

// Run starts node-metrics and blocks until shutdown. It only wires the
// packages node-metrics needs, so the slim mogenius-nodemetrics binary does
// not link the operator's cluster-mode code (Helm, AI, Argo CD, socket API).
//
// Unlike the operator it neither creates nor updates the cluster secret
// (and therefore never runs its data model migration): node-metrics only
// needs the Valkey connection, whose password the chart passes as
// MO_VALKEY_PASSWORD.
func Run(configModule *config.Config, logs bootstrap.Logging, metricsRate uint64) {
	go func() {
		for range logs.RecordChannel {
			// node-metrics does not ship its logs to Valkey
		}
	}()
	go func() {
		defer shutdown.SendShutdownSignal(true)
		configModule.Validate()

		if err := run(configModule, logs, metricsRate); err != nil {
			logs.CmdLogger.Error("node-metrics failed", "error", err)
			return
		}
		select {}
	}()
	shutdown.Listen()
}

func run(configModule *config.Config, logs bootstrap.Logging, metricsRate uint64) error {
	logManager := logs.Manager

	clientProvider := k8sclient.NewK8sClientProvider(logManager.CreateLogger("client-provider"), configModule)
	if !clientProvider.RunsInCluster() && configModule.Get("MO_SKIP_IMPERSONATION") != "true" {
		impersonated, err := clientProvider.WithImpersonate(rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      "mogenius-operator-service-account-app",
			Namespace: configModule.Get("MO_OWN_NAMESPACE"),
		})
		if err != nil {
			return fmt.Errorf("impersonate operator service account: %w", err)
		}
		clientProvider = impersonated
	}

	valkeyLogger := logManager.CreateLogger("valkey")
	valkeyClient := valkeyclient.NewValkeyClient(valkeyLogger, configModule)
	auditLogLimit, err := configModule.TryGetInt("MO_AUDIT_LOG_LIMIT")
	assert.Assert(err == nil, err)
	if err := store.Setup(logManager, valkeyClient, auditLogLimit, configModule.Get("MO_AUDIT_LOG_TTL")); err != nil {
		return fmt.Errorf("set up store: %w", err)
	}
	utils.Setup(logManager, configModule)

	if !configModule.IsSet("MO_VALKEY_PASSWORD") {
		password, err := valkeyPasswordFromSecret(clientProvider, configModule.Get("MO_OWN_NAMESPACE"))
		if err != nil {
			logs.CmdLogger.Error("failed to get valkey password", "error", err)
		} else {
			configModule.Set("MO_VALKEY_PASSWORD", password)
		}
	}
	if err := valkeyclient.ConnectWithRetry(valkeyClient, valkeyLogger, valkeyConnectMaxWait); err != nil {
		return err
	}
	// Flush pending pipelined writes on shutdown instead of dropping them.
	shutdown.Add(valkeyClient.Close)

	containerEnumerator := containerenumerator.NewContainerEnumerator(logManager.CreateLogger("container-enumerator"), configModule, clientProvider)
	cpuMonitor := cpumonitor.NewCpuMonitor(logManager.CreateLogger("cpu-monitor"), configModule, clientProvider, containerEnumerator)
	ramMonitor := rammonitor.NewRamMonitor(logManager.CreateLogger("ram-monitor"), configModule, clientProvider, containerEnumerator)
	networkMonitor := networkmonitor.NewNetworkMonitor(logManager.CreateLogger("network-monitor"), configModule, containerEnumerator, configModule.Get("MO_HOST_PROC_PATH"))
	networkMonitor.Snoopy().SetArgs(networkmonitor.SnoopyArgs{MetricsRate: metricsRate})

	ownerCacheService := store.NewOwnerCacheService(logManager.CreateLogger("owner-cache"), configModule)
	statsDb := statsdb.NewValkeyStatsModule(logManager.CreateLogger("db-stats"), configModule, valkeyClient, ownerCacheService)

	NewCollector(
		logManager.CreateLogger("traffic-collector"),
		configModule.Get("OWN_NODE_NAME"),
		statsDb,
		cpuMonitor,
		ramMonitor,
		networkMonitor,
	).Run()

	return nil
}

// valkeyPasswordFromSecret reads the password of the bundled Valkey from
// its secret, for runs where MO_VALKEY_PASSWORD is not set (local
// development).
func valkeyPasswordFromSecret(clientProvider k8sclient.K8sClientProvider, namespace string) (string, error) {
	secret, err := clientProvider.K8sClientSet().CoreV1().Secrets(namespace).Get(context.Background(), "mogenius-operator-valkey", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get valkey secret: %w", err)
	}
	password := string(secret.Data["valkey-password"])
	if password == "" {
		return "", fmt.Errorf("valkey secret has no valkey-password")
	}
	return password, nil
}
