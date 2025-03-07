package kubernetes

import (
	cmclientset "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	"k8s.io/client-go/rest"
)

type KubeProviderCertManager struct {
	ClientSet    *cmclientset.Clientset
	ClientConfig rest.Config
}

func NewKubeProviderCertManager() (*KubeProviderCertManager, error) {
	provider, err := newKubeProviderCertManagerInCluster()
	if err == nil {
		return provider, nil
	} else {
		provider, err = newKubeProviderCertManagerLocal()
	}

	if err != nil {
		k8sLogger.Error("NewKubeProviderCertManager", "error", err.Error())
	}
	return provider, err
}

func newKubeProviderCertManagerLocal() (*KubeProviderCertManager, error) {
	config := clientProvider.ClientConfig()
	cmClientset, err := cmclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeProviderCertManager{
		ClientSet:    cmClientset,
		ClientConfig: *config,
	}, nil
}

func newKubeProviderCertManagerInCluster() (*KubeProviderCertManager, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := cmclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeProviderCertManager{
		ClientSet:    clientset,
		ClientConfig: *config,
	}, nil
}
