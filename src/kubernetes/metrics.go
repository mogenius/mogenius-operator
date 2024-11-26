package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1" // Add this line to import the corev1 package
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Metrics struct {
	Namespace                string       `json:"namespace"`
	Name                     string       `json:"name"`
	Kind                     string       `json:"kind"`
	PodsMetrics              []PodMetrics `json:"podsMetrics"`
	CpuAverageUtilization    int64        `json:"cpuAverageUtilization"`
	MemoryAverageUtilization int64        `json:"memoryAverageUtilization"`
	CreatedAt                metav1.Time  `json:"createdAt"`
	WindowInMs               int64        `json:"windowInMs"`
}

type PodMetrics struct {
	Name       string             `json:"name"`
	Containers []ContainerMetrics `json:"containers"`
}

type ContainerMetrics struct {
	Name       string `json:"name"`
	CpuUsage   int64  `json:"cpuUsage"`
	CpuRequest int64  `json:"cpuRequest"`
	MemUsage   int64  `json:"memUsage"`
	MemRequest int64  `json:"memRequest"`
}

func GetAverageUtilizationForDeployment(data K8sController) *Metrics {
	kubeProvider, err := NewKubeProvider()
	if err != nil {
		return nil
	}

	metricsProvider, err := NewKubeProviderMetrics()
	if err != nil {
		return nil
	}

	deployment, err := kubeProvider.ClientSet.AppsV1().Deployments(data.Namespace).Get(context.TODO(), data.Name, metav1.GetOptions{})
	if err != nil {
		k8sLogger.Error("Error getting deployment", "namespace", data.Namespace, "deployment", data.Name, "error", err)
		return nil
	}

	labelSelector := metav1.FormatLabelSelector(deployment.Spec.Selector)
	podList, err := kubeProvider.ClientSet.CoreV1().Pods(data.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		k8sLogger.Error("Error listing pods", "namespace", data.Namespace, "labelSelector", labelSelector, "error", err)
		return nil
	}

	podMetricsList, err := metricsProvider.ClientSet.MetricsV1beta1().PodMetricses(data.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		k8sLogger.Error("Error getting pods metrics", "namespace", data.Namespace, "labelSelector", labelSelector, "error", err)
	}

	// create an inner strcut to hold data from pods for direct access with key pod_name and container_name
	type PodResourceRequests struct {
		resources corev1.ResourceList
	}

	podResourceRequestsMap := make(map[string]map[string]PodResourceRequests)
	// fill map with pod metrics
	for _, pod := range podList.Items {
		podResourceRequestsMap[pod.Name] = make(map[string]PodResourceRequests)
		for _, container := range pod.Spec.Containers {
			podResourceRequestsMap[pod.Name][container.Name] = PodResourceRequests{
				resources: container.Resources.Requests,
			}
		}
	}

	var totalCPUUsage int64 = 0
	var totalMemoryUsage int64 = 0
	var totalCPURequests int64 = 0
	var totalMemoryRequests int64 = 0
	var podCount int64 = 0
	var avgWindowInMs int64 = 0

	pods := []PodMetrics{}

MetricLoop:
	for _, podMetrics := range podMetricsList.Items {

		containers := []ContainerMetrics{}

		for _, container := range podMetrics.Containers {
			// Check if the pod exists in the map
			if containerMap, podExists := podResourceRequestsMap[podMetrics.Name]; podExists {
				// Check if the container exists in the map
				if resources, containerExists := containerMap[container.Name]; containerExists {
					// Check if the Requests map is nil
					if resources.resources == nil {
						k8sLogger.Warn("No resource requests found for container %s in pod %s", container.Name, podMetrics.Name)
						continue MetricLoop
					}
				}
			}

			cpuUsage := container.Usage.Cpu()
			memoryUsage := container.Usage.Memory()

			cpuUsageValue := cpuUsage.MilliValue()
			memoryUsageValue := memoryUsage.Value()

			totalCPUUsage += cpuUsageValue
			totalMemoryUsage += memoryUsageValue

			containerSpec := podResourceRequestsMap[podMetrics.Name][container.Name]
			cpuRequest := containerSpec.resources.Cpu()
			memoryRequest := containerSpec.resources.Memory()

			cpuRequestValue := cpuRequest.MilliValue()
			memoryRequestValue := memoryRequest.Value()

			totalCPURequests += cpuRequestValue
			totalMemoryRequests += memoryRequestValue

			// map into ContainerMetrics
			containers = append(containers, ContainerMetrics{
				Name:       container.Name,
				CpuUsage:   cpuUsageValue,
				CpuRequest: cpuRequestValue,
				MemUsage:   memoryUsageValue,
				MemRequest: memoryRequestValue,
			})
		}

		if len(containers) > 0 {
			pods = append(pods, PodMetrics{
				Name:       podMetrics.Name,
				Containers: containers,
			})
		}

		// window duration in ms
		avgWindowInMs += podMetrics.Window.Milliseconds()

		podCount++
	}

	if podCount == 0 {
		k8sLogger.Error("No pods found for deployment", "deployment", data.Name)
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
		PodsMetrics:              pods,
		CpuAverageUtilization:    int64(avgCPUUtilization),
		MemoryAverageUtilization: int64(avgMemoryUtilization),
		CreatedAt:                metav1.Now(),
		WindowInMs:               avgWindowInMs,
	}
}
