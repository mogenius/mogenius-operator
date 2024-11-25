package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeProvider struct {
	ClientSet    *kubernetes.Clientset
	ClientConfig rest.Config
}

func NewKubeProvider() (*KubeProvider, error) {
	provider, err := newKubeProviderInCluster()
	if err == nil {
		return provider, nil
	} else {
		provider, err = newKubeProviderLocal()
	}

	if err != nil {
		k8sLogger.Error("NewKubeProvider", "error", err.Error())
	}
	return provider, err
}

func newKubeProviderLocal() (*KubeProvider, error) {
	config, err := ContextConfigLoader()
	if err != nil {
		return nil, err
	}

	clientSet, errClientSet := kubernetes.NewForConfig(config)
	if errClientSet != nil {
		return nil, errClientSet
	}

	return &KubeProvider{
		ClientSet:    clientSet,
		ClientConfig: *config,
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

	return &KubeProvider{
		ClientSet:    clientset,
		ClientConfig: *config,
	}, nil
}

func ContextConfigLoader() (*rest.Config, error) {
	var kubeconfigs []string = GetDefaultKubeConfig()
	var config *rest.Config
	var err error
	for _, kubeconfig := range kubeconfigs {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err == nil {
			return config, nil
		}
	}
	return nil, err
}
