package kubernetes

import (
	"context"
	"fmt"
	"log"

	punq "github.com/mogenius/punq/kubernetes"
	"github.com/mogenius/punq/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Metrics struct {
	Namespace                string `json:"namespace"`
	Name                     string `json:"name"`
	Kind                     string `json:"kind"`
	Cpu                      int64  `json:"cpu"`
	CpuLimit                 int64  `json:"cpuLimit"`
	CpuAverageUtilization    int64  `json:"cpuAverageUtilization"`
	Memory                   int64  `json:"memory"`
	MemoryLimit              int64  `json:"memoryLimit"`
	MemoryAverageUtilization int64  `json:"MemoryAverageUtilization"`
	CreatedAt                string `json:"createdAt"`
	WindowInMs               string `json:"window"`
}

type MetricsProvider struct {
	ClientSet    *metricsv.Clientset
	ClientConfig rest.Config
}

func NewMetricsProvider(contextId *string) (*MetricsProvider, error) {
	var provider *MetricsProvider
	var err error
	if punq.RunsInCluster {
		provider, err = newMetricsProviderInCluster(contextId)
	} else {
		provider, err = newMetricsProviderLocal(contextId)
	}

	if err != nil {
		logger.Log.Errorf("ERROR: %s", err.Error())
	}
	return provider, err
}

func newMetricsProviderLocal(contextId *string) (*MetricsProvider, error) {
	config, err := punq.ContextSwitcher(contextId)
	if err != nil {
		return nil, err
	}

	clientSet, errClientSet := metricsv.NewForConfig(config)
	if errClientSet != nil {
		return nil, errClientSet
	}

	return &MetricsProvider{
		ClientSet:    clientSet,
		ClientConfig: *config,
	}, nil
}

func newMetricsProviderInCluster(contextId *string) (*MetricsProvider, error) {
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

	clientset, err := metricsv.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &MetricsProvider{
		ClientSet:    clientset,
		ClientConfig: *config,
	}, nil
}

func GetAverageUtilizationForDeployment(data K8sController) *Metrics {
	kubeProvider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return nil
	}
	metricsProvider, err := NewMetricsProvider(nil)
	if err != nil {
		return nil
	}

	namespace := data.Namespace
	deploymentName := data.Name

	deployment, err := kubeProvider.ClientSet.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Error getting deployment: %v", err)
	}

	labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
	podList, err := kubeProvider.ClientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Fatalf("Error listing pods: %v", err)
	}

	var totalCPUUsage int64 = 0
	var totalMemoryUsage int64 = 0
	var totalCPURequests int64 = 0
	var totalMemoryRequests int64 = 0
	var podCount int64 = 0

	for _, pod := range podList.Items {
		podMetrics, err := metricsProvider.ClientSet.MetricsV1beta1().PodMetricses(namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
		if err != nil {
			log.Printf("Error getting metrics for pod %s: %v", pod.Name, err)
			continue
		}

		for i, container := range podMetrics.Containers {
			cpuUsage := container.Usage["cpu"]
			memoryUsage := container.Usage["memory"]

			// cpuUsageValue := cpuUsage.AsApproximateFloat64()
			cpuUsageValue := cpuUsage.Value()
			// memoryUsageValue := memoryUsage.Value() / (1024 * 1024) // Convert to MiB
			memoryUsageValue := memoryUsage.Value()

			// totalCPUUsage += int64(cpuUsageValue)
			totalCPUUsage += cpuUsageValue
			totalMemoryUsage += memoryUsageValue

			containerSpec := pod.Spec.Containers[i]
			cpuRequest := containerSpec.Resources.Requests["cpu"]
			memoryRequest := containerSpec.Resources.Requests["memory"]

			// cpuRequestValue := cpuRequest.AsApproximateFloat64()
			// fmt.Printf("cpuRequestValue: %d\n", cpuRequest.Value())
			// memoryRequestValue := memoryRequest.Value() / (1024 * 1024) // Convert to MiB
			cpuRequestValue := cpuRequest.Value()
			memoryRequestValue := memoryRequest.Value()

			// totalCPURequests += int64(cpuRequestValue)
			totalCPURequests += cpuRequestValue
			totalMemoryRequests += memoryRequestValue
		}

		podCount++
	}

	if podCount == 0 {
		log.Fatalf("No pods found for deployment %s", deploymentName)
	}

	avgCPUUsage := totalCPUUsage / podCount
	avgMemoryUsage := totalMemoryUsage / podCount
	avgCPURequest := totalCPURequests / podCount
	avgMemoryRequest := totalMemoryRequests / podCount

	avgCPUUtilization := (float64(avgCPUUsage) / float64(avgCPURequest)) * 100
	avgMemoryUtilization := (float64(avgMemoryUsage) / float64(avgMemoryRequest)) * 100

	fmt.Printf("Average CPU Usage (nanocores): %d\n", avgCPUUsage)
	fmt.Printf("Total CPU Request (nanocores): %d\n", totalCPURequests)
	fmt.Printf("Average CPU Utilization: %.2f%%\n", avgCPUUtilization)
	fmt.Println("")
	fmt.Printf("Average Memory Usage (bytes): %d\n", avgMemoryUsage)
	fmt.Printf("Total Memory Request (bytes): %d\n", totalMemoryRequests)
	fmt.Printf("Average Memory Utilization: %.2f%%\n", avgMemoryUtilization)
}
