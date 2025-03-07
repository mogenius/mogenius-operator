package k8sclient

import (
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	mocrds "mogenius-k8s-manager/src/crds"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/utils"
	"os"
	"path/filepath"
	"strings"

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
	DynamicClient() *dynamic.DynamicClient
	MogeniusClientSet() *mocrds.MogeniusClientSet
	RunsInCluster() bool
	ClientConfig() *rest.Config
}

type k8sClientProvider struct {
	clientConfig     *rest.Config
	executionContext ExecutionContext
}

func NewK8sClientProvider(logger *slog.Logger) K8sClientProvider {
	assert.Assert(logger != nil)

	provider := new(k8sClientProvider)

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

	_, err = dynamic.NewForConfig(config)
	if err != nil {
		logger.Error("invalid kubeconfig - cant create `*dynamic.DynamicClient`", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}

	provider.clientConfig = config

	return provider
}

func (self *k8sClientProvider) ClientConfig() *rest.Config {
	return utils.ShallowCopy(self.clientConfig)
}

func (self *k8sClientProvider) K8sClientSet() *kubernetes.Clientset {
	clientSet, err := kubernetes.NewForConfig(self.clientConfig)
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

func (self *k8sClientProvider) detectAndGetKubeConfig(logger *slog.Logger) (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		self.executionContext = execution_context_cluster
		return config, nil
	}
	logger.Debug("failed to get rest.InClusterConfig", "error", err)

	config, err = self.contextConfigLoader(logger)
	if err == nil {
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
