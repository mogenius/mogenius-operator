package kubernetes

import (
	"context"

	punq "github.com/mogenius/punq/kubernetes"
	corev1 "k8s.io/api/core/v1" // Add this line to import the corev1 package
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Metrics struct {
	Namespace                string      `json:"namespace"`
	Name                     string      `json:"name"`
	Kind                     string      `json:"kind"`
	Cpu                      int64       `json:"cpu"`
	CpuLimit                 int64       `json:"cpuLimit"`
	CpuAverageUtilization    int64       `json:"cpuAverageUtilization"`
	Memory                   int64       `json:"memory"`
	MemoryLimit              int64       `json:"memoryLimit"`
	MemoryAverageUtilization int64       `json:"MemoryAverageUtilization"`
	CreatedAt                metav1.Time `json:"createdAt"`
	WindowInMs               int64       `json:"windowInMs"`
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
		K8sLogger.Errorf("ERROR: %s", err.Error())
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

	deployment, err := kubeProvider.ClientSet.AppsV1().Deployments(data.Namespace).Get(context.TODO(), data.Name, metav1.GetOptions{})
	if err != nil {
		K8sLogger.Errorf("Error getting deployment: %v", err)
		return nil
	}

	labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
	podList, err := kubeProvider.ClientSet.CoreV1().Pods(data.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		K8sLogger.Errorf("Error listing pods: %v", err)
		return nil
	}

	podMetricsList, err := metricsProvider.ClientSet.MetricsV1beta1().PodMetricses(data.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		K8sLogger.Errorf("Error getting pods metrics: %v", err)
	}

	// create an inner strcut to hold data from pods for direct access with key pod_name and container_name
	type PodResourceRequests struct {
		Requests corev1.ResourceList
	}

	podResourceRequestsMap := make(map[string]map[string]PodResourceRequests)
	// fill map with pod metrics
	for _, pod := range podList.Items {
		podResourceRequestsMap[pod.Name] = make(map[string]PodResourceRequests)
		for _, container := range pod.Spec.Containers {
			podResourceRequestsMap[pod.Name][container.Name] = PodResourceRequests{
				Requests: container.Resources.Requests,
			}
		}
	}

	var totalCPUUsage int64 = 0
	var totalMemoryUsage int64 = 0
	var totalCPURequests int64 = 0
	var totalMemoryRequests int64 = 0
	var podCount int64 = 0
	var avgWindowInMs int64 = 0

	for _, podMetrics := range podMetricsList.Items {
		for _, container := range podMetrics.Containers {
			// Check if the pod exists in the map
			if containerMap, podExists := podResourceRequestsMap[podMetrics.Name]; podExists {
				// Check if the container exists in the map
				if resources, containerExists := containerMap[container.Name]; containerExists {
					// Check if the Requests map is nil
					if resources.Requests == nil {
						K8sLogger.Warningf("No resource requests found for container %s in pod %s", container.Name, podMetrics.Name)
						continue
					}
				}
			}

			cpuUsage := container.Usage["cpu"]
			memoryUsage := container.Usage["memory"]

			cpuUsageValue := cpuUsage.Value()
			memoryUsageValue := memoryUsage.Value()

			totalCPUUsage += cpuUsageValue
			totalMemoryUsage += memoryUsageValue

			containerSpec := podResourceRequestsMap[podMetrics.Name][container.Name]
			cpuRequest := containerSpec.Requests["cpu"]
			memoryRequest := containerSpec.Requests["memory"]

			cpuRequestValue := cpuRequest.Value()
			memoryRequestValue := memoryRequest.Value()

			totalCPURequests += cpuRequestValue
			totalMemoryRequests += memoryRequestValue
		}
		// window duration in ms
		avgWindowInMs += podMetrics.Window.Milliseconds()

		podCount++
	}

	// for _, pod := range podList.Items {
	// 	podMetrics, err := metricsProvider.ClientSet.MetricsV1beta1().PodMetricses(data.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
	// 	if err != nil {
	// 		K8sLogger.Errorf("Error getting metrics for pod %s: %v", pod.Name, err)
	// 		continue
	// 	}

	// 	for i, container := range podMetrics.Containers {
	// 		cpuUsage := container.Usage["cpu"]
	// 		memoryUsage := container.Usage["memory"]

	// 		cpuUsageValue := cpuUsage.Value()
	// 		memoryUsageValue := memoryUsage.Value()

	// 		totalCPUUsage += cpuUsageValue
	// 		totalMemoryUsage += memoryUsageValue

	// 		containerSpec := pod.Spec.Containers[i]
	// 		cpuRequest := containerSpec.Resources.Requests["cpu"]
	// 		memoryRequest := containerSpec.Resources.Requests["memory"]

	// 		cpuRequestValue := cpuRequest.Value()
	// 		memoryRequestValue := memoryRequest.Value()

	// 		totalCPURequests += cpuRequestValue
	// 		totalMemoryRequests += memoryRequestValue
	// 	}

	// 	// window duration in ms
	// 	avgWindowInMs += podMetrics.Window.Milliseconds()

	// 	podCount++
	// }

	if podCount == 0 {
		K8sLogger.Errorf("No pods found for deployment %s", data.Name)
		return nil
	}

	avgWindowInMs = avgWindowInMs / podCount
	avgCPUUsage := totalCPUUsage / podCount
	avgMemoryUsage := totalMemoryUsage / podCount
	avgCPURequest := totalCPURequests / podCount
	avgMemoryRequest := totalMemoryRequests / podCount

	avgCPUUtilization := (float64(avgCPUUsage) / float64(avgCPURequest)) * 100
	avgMemoryUtilization := (float64(avgMemoryUsage) / float64(avgMemoryRequest)) * 100

	return &Metrics{
		Namespace:                data.Namespace,
		Name:                     data.Name,
		Kind:                     data.Kind,
		Cpu:                      avgCPUUsage,
		CpuLimit:                 totalCPURequests,
		CpuAverageUtilization:    int64(avgCPUUtilization),
		Memory:                   avgMemoryUsage,
		MemoryLimit:              totalMemoryRequests,
		MemoryAverageUtilization: int64(avgMemoryUtilization),
		CreatedAt:                metav1.Now(),
		WindowInMs:               avgWindowInMs,
	}
}
