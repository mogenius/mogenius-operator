package kubernetes

import (
	"fmt"
	utils "mogenius-k8s-manager/utils"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type KubeProvider struct {
	ClientSet     *kubernetes.Clientset
	DynamicClient *dynamic.DynamicClient
	ClientConfig  rest.Config
}

func NewKubeProvider() (*KubeProvider, error) {
	var provider *KubeProvider
	var err error
	if utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = newKubeProviderInCluster()
	} else {
		provider, err = newKubeProviderLocal()
	}

	if err != nil {
		k8sLogger.Error("failed to create kube provider", "error", err)
	}
	return provider, err
}

func newKubeProviderLocal() (*KubeProvider, error) {
	config, err := GetRestConfig()
	if err != nil {
		return nil, err
	}

	clientSet, errClientSet := kubernetes.NewForConfig(config)
	if errClientSet != nil {
		return nil, errClientSet
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeProvider{
		ClientSet:     clientSet,
		DynamicClient: dynamicClient,
		ClientConfig:  *config,
	}, nil
}

func newKubeProviderInCluster() (*KubeProvider, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeProvider{
		ClientSet:     clientset,
		DynamicClient: dynamicClient,
		ClientConfig:  *config,
	}, nil
}

func GetRestConfig() (*rest.Config, error) {
	var kubeconfigs []string = GetDefaultKubeConfig()
	var config *rest.Config
	var err error
	for _, kubeconfig := range kubeconfigs {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err == nil {
			return config, nil
		}
	}

	return nil, fmt.Errorf("Error loading kubeconfig: %s", err.Error())
}

func GetDefaultKubeConfig() []string {
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

	// Check that at least one kubeconfig file exists
	validConfigs := []string{}
	for _, singleConfig := range kubeconfigs {
		if _, err := os.Stat(singleConfig); os.IsNotExist(err) {
			k8sLogger.Warn("kubeconfig file does not exist", "kubeConfig", singleConfig)
		} else {
			validConfigs = append(validConfigs, singleConfig)
		}
	}

	if len(validConfigs) == 0 {
		k8sLogger.Error("Error: No valid kubeconfig file found. Ensure that the $KUBECONFIG environment variable or the default kubeconfig is set correctly. For more info, see https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/.")
		panic(1)
	}

	return validConfigs
}
