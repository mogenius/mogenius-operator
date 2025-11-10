package k8sclient

import (
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	mocrds "mogenius-operator/src/crds"
	"mogenius-operator/src/shutdown"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type ExecutionContext int8

const (
	execution_context_cluster ExecutionContext = iota
	execution_context_local
)

type K8sClientProvider interface {
	K8sClientSet() *kubernetes.Clientset
	MetricsClientSet() *metricsv.Clientset
	DynamicClient() *dynamic.DynamicClient
	MogeniusClientSet() *mocrds.MogeniusClientSet
	RunsInCluster() bool
	WithImpersonate(subject rbacv1.Subject) (K8sClientProvider, error)
	ClientConfig() *rest.Config
}

type k8sClientProvider struct {
	clientConfig     *rest.Config
	executionContext ExecutionContext
	config           config.ConfigModule
}

func NewK8sClientProvider(logger *slog.Logger, configModule config.ConfigModule) K8sClientProvider {
	assert.Assert(logger != nil)
	assert.Assert(configModule != nil)

	provider := new(k8sClientProvider)
	provider.config = configModule

	config, err := provider.detectAndGetKubeConfig(logger)
	if err != nil {
		logger.Error("failed to detect kubeconfig", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	_, err = kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error("invalid kubeconfig - cant create `*kubernetes.Clientset`", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	_, err = metricsv.NewForConfig(config)
	if err != nil {
		logger.Error("invalid kubeconfig - cant create `*metricsv.Clientset`", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	_, err = dynamic.NewForConfig(config)
	if err != nil {
		logger.Error("invalid kubeconfig - cant create `*dynamic.DynamicClient`", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	provider.clientConfig = config

	return provider
}

func (self *k8sClientProvider) WithImpersonate(subject rbacv1.Subject) (K8sClientProvider, error) {
	other := &k8sClientProvider{}
	other.executionContext = self.executionContext
	other.config = self.config
	other.clientConfig = self.ClientConfig()

	switch subject.Kind {
	case "User":
		if subject.Name == "" {
			return nil, fmt.Errorf(`User Name should be set`)
		}
		if subject.Namespace != "" {
			return nil, fmt.Errorf(`User Namespace should be empty`)
		}
		if subject.APIGroup != "rbac.authorization.k8s.io" {
			return nil, fmt.Errorf(`User APIGroup should be "rbac.authorization.k8s.io"`)
		}
		other.clientConfig.Impersonate = rest.ImpersonationConfig{
			UserName: subject.Name,
		}
	case "Group":
		if subject.Name == "" {
			return nil, fmt.Errorf(`Group Name should be set`)
		}
		if subject.Namespace != "" {
			return nil, fmt.Errorf(`Group Namespace should be empty`)
		}
		if subject.APIGroup != "rbac.authorization.k8s.io" {
			return nil, fmt.Errorf(`Group APIGroup should be "rbac.authorization.k8s.io"`)
		}
		other.clientConfig.Impersonate = rest.ImpersonationConfig{
			Groups: []string{subject.Name},
		}
	case "ServiceAccount":
		if subject.Name == "" {
			return nil, fmt.Errorf(`ServiceAccount Name should be set`)
		}
		if subject.Namespace == "" {
			return nil, fmt.Errorf(`ServiceAccount Namespace should be set`)
		}
		if subject.APIGroup != "" {
			return nil, fmt.Errorf(`ServiceAccount APIGroup should be empty`)
		}
		other.clientConfig.Impersonate = rest.ImpersonationConfig{
			UserName: "system:serviceaccount:" + subject.Namespace + ":" + subject.Name + "",
		}
	default:
		assert.Assert(false, "Unknown subject.Kind for impersonate config", subject)
	}

	return other, nil
}

func (self *k8sClientProvider) ClientConfig() *rest.Config {
	return rest.CopyConfig(self.clientConfig)
}

func (self *k8sClientProvider) K8sClientSet() *kubernetes.Clientset {
	clientSet, err := kubernetes.NewForConfig(self.clientConfig)
	assert.Assert(err == nil, "creating a client should not fail as it is tested when provider is created", err)
	assert.Assert(clientSet != nil)
	return clientSet
}

func (self *k8sClientProvider) MetricsClientSet() *metricsv.Clientset {
	clientSet, err := metricsv.NewForConfig(self.clientConfig)
	assert.Assert(err == nil, "creating a client should not fail as it is tested when provider is created", err)
	assert.Assert(clientSet != nil)
	return clientSet
}

func (self *k8sClientProvider) DynamicClient() *dynamic.DynamicClient {
	client, err := dynamic.NewForConfig(self.clientConfig)
	assert.Assert(err == nil, "creating a client should not fail as it is tested when provider is created", err)
	assert.Assert(client != nil)
	return client
}

func (self *k8sClientProvider) MogeniusClientSet() *mocrds.MogeniusClientSet {
	clientSet, err := mocrds.NewMogeniusClientSet(*self.clientConfig)
	assert.Assert(err == nil, "creating a client should not fail as it is tested when provider is created", err)
	assert.Assert(clientSet != nil)
	return clientSet
}

type loggingRoundTripper struct {
	rt     http.RoundTripper
	logger *slog.Logger
}

func (l *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	l.logger.Info("K8s API request", "method", req.Method, "url", req.URL.String()) // debug.Stack() if you want to see the stack trace
	return l.rt.RoundTrip(req)
}

func (self *k8sClientProvider) detectAndGetKubeConfig(logger *slog.Logger) (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		config.UserAgent = "mogenius-operator"
		if self.config.Get("KUBERNETES_DEBUG") == "true" {
			config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
				return &loggingRoundTripper{rt: rt, logger: logger}
			}
		}
		self.executionContext = execution_context_cluster
		return config, nil
	}
	logger.Debug("failed to get rest.InClusterConfig", "error", err)

	config, err = self.contextConfigLoader(logger)
	if err == nil {
		config.UserAgent = "mogenius-operator"
		if self.config.Get("KUBERNETES_DEBUG") == "true" {
			config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
				return &loggingRoundTripper{rt: rt, logger: logger}
			}
		}
		self.executionContext = execution_context_local
		return config, nil
	}
	logger.Debug("failed to get local kubeconfig", "error", err)

	return nil, fmt.Errorf("failed to initialize kubeconfig for k8s client")
}

func (self *k8sClientProvider) contextConfigLoader(logger *slog.Logger) (*rest.Config, error) {
	kubeconfigs, err := getDefaultKubeConfig(logger)
	if err != nil {
		return nil, err
	}

	var config *rest.Config
	for _, kubeconfig := range kubeconfigs {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err == nil {
			return config, nil
		}
	}

	return nil, err
}

func getDefaultKubeConfig(logger *slog.Logger) ([]string, error) {
	var kubeconfig string = os.Getenv("KUBECONFIG")
	var kubeconfigs []string

	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
			kubeconfigs = []string{kubeconfig}
		}
	} else {
		kubeconfigs = strings.Split(kubeconfig, ":")
	}
	if len(kubeconfigs) == 0 {
		return []string{}, fmt.Errorf("failed to determine any valid kubeconfig in either $KUBECONFIG or standard paths")
	}

	// Check that at least one kubeconfig file exists and is readable
	validConfigs := []string{}
	for _, singleConfig := range kubeconfigs {
		_, err := os.Stat(singleConfig)
		if os.IsNotExist(err) {
			logger.Debug("kubeconfig file does not exist", "kubeConfig", singleConfig, "error", err)
			continue
		}
		if os.IsPermission(err) {
			logger.Debug("no permission to read kubeconfig file", "kubeConfig", singleConfig, "error", err)
			continue
		}
		validConfigs = append(validConfigs, singleConfig)
	}

	if len(validConfigs) == 0 {
		return []string{}, fmt.Errorf("could not read any kubeconfig")
	}

	return validConfigs, nil
}

func (self *k8sClientProvider) RunsInCluster() bool {
	switch self.executionContext {
	case execution_context_cluster:
		return true
	case execution_context_local:
		return false
	default:
		panic(fmt.Errorf("unreachable: unhandled execution context"))
	}
}
