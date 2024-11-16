package kubernetes

import (
	snapClientset "github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned"
	"k8s.io/client-go/rest"
)

type KubeProviderSnapshot struct {
	ClientSet    *snapClientset.Clientset
	ClientConfig rest.Config
}

func NewKubeProviderSnapshot() (*KubeProviderSnapshot, error) {
	provider, err := newKubeProviderCsiInCluster()
	if err == nil {
		return provider, nil
	} else {
		provider, err = newKubeProviderCsiLocal()
	}

	if err != nil {
		k8sLogger.Error("NewKubeProviderSnapshot", "error", err.Error())
	}
	return provider, err
}

func newKubeProviderCsiLocal() (*KubeProviderSnapshot, error) {
	config, err := ContextConfigLoader()
	if err != nil {
		return nil, err
	}

	clientSet, errClientSet := snapClientset.NewForConfig(config)
	if errClientSet != nil {
		return nil, errClientSet
	}

	return &KubeProviderSnapshot{
		ClientSet:    clientSet,
		ClientConfig: *config,
	}, nil
}

func newKubeProviderCsiInCluster() (*KubeProviderSnapshot, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := snapClientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeProviderSnapshot{
		ClientSet:    clientset,
		ClientConfig: *config,
	}, nil
}
