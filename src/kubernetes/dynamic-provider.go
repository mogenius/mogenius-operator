package kubernetes

import (
	"fmt"
	"sync"

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

var executionContext ExecutionContext = unknown
var executionContextLock sync.Mutex = sync.Mutex{}

type ExecutionContext int8

const (
	unknown ExecutionContext = iota
	runs_in_cluster
	runs_local
)

func RunsInCluster() bool {
	executionContextLock.Lock()
	defer executionContextLock.Unlock()

	if executionContext == unknown {
		_, err := rest.InClusterConfig()
		if err == nil {
			executionContext = runs_in_cluster
		} else {
			executionContext = runs_local
		}
	}

	switch executionContext {
	case unknown:
		panic("unreachable")
	case runs_in_cluster:
		return true
	case runs_local:
		return false
	default:
		panic(fmt.Errorf("unreachable: unhandled execution context"))
	}
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
