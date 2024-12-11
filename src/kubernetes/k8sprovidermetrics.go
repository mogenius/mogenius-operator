package kubernetes

import (
	"k8s.io/client-go/rest"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type KubeProviderMetrics struct {
	ClientSet    *metricsv.Clientset
	ClientConfig rest.Config
}

func NewKubeProviderMetrics() (*KubeProviderMetrics, error) {
	provider, err := newKubeProviderMetricsInCluster()
	if err == nil {
		return provider, nil
	} else {
		provider, err = newKubeProviderMetricsLocal()
	}

	if err != nil {
		k8sLogger.Error("NewKubeProviderMetrics", "error", err.Error())
	}
	return provider, err
}

func newKubeProviderMetricsLocal() (*KubeProviderMetrics, error) {
	config := clientProvider.ClientConfig()

	clientSet, errClientSet := metricsv.NewForConfig(config)
	if errClientSet != nil {
		return nil, errClientSet
	}

	return &KubeProviderMetrics{
		ClientSet:    clientSet,
		ClientConfig: *config,
	}, nil
}

func newKubeProviderMetricsInCluster() (*KubeProviderMetrics, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := metricsv.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeProviderMetrics{
		ClientSet:    clientset,
		ClientConfig: *config,
	}, nil
}
