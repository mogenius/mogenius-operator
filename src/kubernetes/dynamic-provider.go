package kubernetes

import (
	punq "github.com/mogenius/punq/kubernetes"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type DynamicKubeProvider struct {
	ClientSet    *dynamic.DynamicClient
	ClientConfig rest.Config
}

func NewDynamicKubeProvider(contextId *string) (*DynamicKubeProvider, error) {
	provider, err := newDynamicKubeProviderInCluster(contextId)
	if err == nil {
		return provider, nil
	} else {
		provider, err = newDynamicKubeProviderLocal(contextId)
	}

	if err != nil {
		k8sLogger.Error("failed to create dynamic kube provider", "error", err)
	}
	return provider, err
}

func RunsInCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}

func newDynamicKubeProviderLocal(contextId *string) (*DynamicKubeProvider, error) {
	config, err := punq.ContextSwitcher(contextId)
	if err != nil {
		return nil, err
	}

	clientSet, errClientSet := dynamic.NewForConfig(config)
	if errClientSet != nil {
		return nil, errClientSet
	}

	return &DynamicKubeProvider{
		ClientSet:    clientSet,
		ClientConfig: *config,
	}, nil
}

func newDynamicKubeProviderInCluster(contextId *string) (*DynamicKubeProvider, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	if contextId != nil {
		config, err = punq.ContextSwitcher(contextId)
		if err != nil {
			return nil, err
		}
	}

	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &DynamicKubeProvider{
		ClientSet:    clientset,
		ClientConfig: *config,
	}, nil
}
