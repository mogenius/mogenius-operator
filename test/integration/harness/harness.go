// Package harness provides a test harness that starts the full operator stack
// against a fake platform API server, an in-memory Valkey, and an in-process
// Kubernetes API server (via controller-runtime envtest). No external cluster
// or Docker is required.
//
// envtest requires the kube-apiserver and etcd binaries. Point KUBEBUILDER_ASSETS
// at their directory before running tests (see the test-e2e Justfile target which
// does this automatically via setup-envtest).
//
// Tests must not run in parallel: the global shutdown registry and package-level
// module state in helm/kubernetes/store are shared across operator instances.
package harness

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"mogenius-operator/src/cmd"
	"mogenius-operator/src/config"
	"mogenius-operator/src/logging"

	miniredis "github.com/alicebob/miniredis/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const operatorNamespace = "mogenius"

// TestHarness holds the running test infrastructure.
// Use APIServer.Send to send commands to the operator.
// Use K8s to assert Kubernetes state changes.
type TestHarness struct {
	// APIServer is the fake platform WebSocket server (MO_API_SERVER).
	APIServer *FakeAPIServer
	// EventServer is the fake event WebSocket server (MO_EVENT_SERVER).
	EventServer *FakeAPIServer
	// K8s is a client pointed at the in-process envtest API server.
	K8s kubernetes.Interface

	cancel context.CancelFunc
	done   chan struct{}
}

// New starts the full operator against in-process infrastructure and returns when
// the operator has established its WebSocket connection and is ready for commands.
// All resources are cleaned up via t.Cleanup. Do not call t.Parallel() in tests using this.
func New(t *testing.T) *TestHarness {
	t.Helper()

	// Start in-process kube-apiserver + etcd.
	// KUBEBUILDER_ASSETS must point at the envtest binaries directory.
	env := &envtest.Environment{}
	restCfg, err := env.Start()
	if err != nil {
		t.Fatalf("harness: start envtest: %v\n(hint: run 'just setup-envtest' or set KUBEBUILDER_ASSETS)", err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Logf("harness: stop envtest: %v", err)
		}
	})

	k8s, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		t.Fatalf("harness: create k8s client: %v", err)
	}

	// The operator uses this namespace for its Secret, ConfigMap, and leader election.
	_, err = k8s.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: operatorNamespace},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("harness: create namespace %q: %v", operatorNamespace, err)
	}

	apiServer := NewFakeAPIServer()
	eventServer := NewFakeAPIServer()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("harness: start miniredis: %v", err)
	}

	// Write the envtest rest.Config as a KUBECONFIG file so the operator picks it up.
	// Blank the in-cluster env vars: when tests run inside a Kubernetes pod (e.g. ARC
	// runners) these are set and cause rest.InClusterConfig() to win over KUBECONFIG.
	setEnv(t, "KUBERNETES_SERVICE_HOST", "")
	setEnv(t, "KUBERNETES_SERVICE_PORT", "")
	setEnv(t, "KUBECONFIG", writeKubeconfig(t, restCfg))

	setEnv(t, "MO_API_SERVER", apiServer.URL)
	setEnv(t, "MO_EVENT_SERVER", eventServer.URL)
	setEnv(t, "MO_VALKEY_ADDR", mr.Addr())
	setEnv(t, "MO_VALKEY_PASSWORD", "")
	setEnv(t, "MO_API_KEY", "test-api-key")
	setEnv(t, "MO_CLUSTER_NAME", "test-cluster")
	setEnv(t, "MO_CLUSTER_MFA_ID", "test-mfa-id")
	setEnv(t, "OWN_NODE_NAME", "test-node")
	setEnv(t, "MO_OWN_NAMESPACE", operatorNamespace)
	setEnv(t, "MO_HTTP_ADDR", ":0")
	setEnv(t, "MO_SKIP_IMPERSONATION", "true")
	setEnv(t, "MO_HELM_DATA_PATH", "/tmp/operator")
	setEnv(t, "HELM_REGISTRY_CONFIG", "/tmp/operator/helm/config.json")
	setEnv(t, "HELM_REPOSITORY_CACHE", "/tmp/operator/helm/cache")
	setEnv(t, "HELM_REPOSITORY_CONFIG", "/tmp/operator/helm/repositories.yaml")
	// snoopy (eBPF) is not available in test environments; use the proc-based reader instead.
	setEnv(t, "MO_SNOOPY_IMPLEMENTATION", "procdev")

	cfgModule := config.NewConfig()
	cmd.LoadConfigDeclarations(cfgModule)
	cfgModule.LoadEnvs()

	logManager := logging.NewSlogManager(slog.LevelError, []slog.Handler{slog.NewJSONHandler(os.Stderr, nil)})
	cmdLogger := logManager.CreateLogger("cmd")
	valkeyLogCh := make(chan logging.LogLine, 100)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		if err := cmd.RunClusterWithContext(ctx, logManager, cfgModule, cmdLogger, valkeyLogCh); err != nil {
			t.Errorf("harness: operator error: %v", err)
		}
	}()

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer connectCancel()
	if err := apiServer.WaitConnected(connectCtx); err != nil {
		cancel()
		<-done
		t.Fatalf("harness: %v", err)
	}

	h := &TestHarness{
		APIServer:   apiServer,
		EventServer: eventServer,
		K8s:         k8s,
		cancel:      cancel,
		done:        done,
	}

	t.Cleanup(func() {
		cancel()
		apiServer.Close()
		eventServer.Close()
		mr.Close()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Log("harness: operator did not stop within 15s")
		}
	})

	return h
}

// writeKubeconfig serialises restCfg into a temp kubeconfig file and returns its path.
func writeKubeconfig(t *testing.T, restCfg *rest.Config) string {
	t.Helper()

	apiCfg := clientcmdapi.NewConfig()
	apiCfg.Clusters["test"] = &clientcmdapi.Cluster{
		Server:                   restCfg.Host,
		CertificateAuthorityData: restCfg.CAData,
	}
	apiCfg.AuthInfos["test"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: restCfg.CertData,
		ClientKeyData:         restCfg.KeyData,
	}
	apiCfg.Contexts["test"] = &clientcmdapi.Context{
		Cluster:  "test",
		AuthInfo: "test",
	}
	apiCfg.CurrentContext = "test"

	data, err := clientcmd.Write(*apiCfg)
	if err != nil {
		t.Fatalf("harness: marshal kubeconfig: %v", err)
	}

	f, err := os.CreateTemp("", "harness-kubeconfig-*.yaml")
	if err != nil {
		t.Fatalf("harness: create kubeconfig temp file: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		t.Fatalf("harness: write kubeconfig: %v", err)
	}

	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// setEnv sets key=value for the test duration and restores the original in t.Cleanup.
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	orig, existed := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("harness: setenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, orig)
		} else {
			os.Unsetenv(key)
		}
	})
}
