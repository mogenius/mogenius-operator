package kubernetes

import (
	"context"
	"mogenius-k8s-manager/src/dtos"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func GetNodeStats() []dtos.NodeStat {
	result := []dtos.NodeStat{}
	nodes := ListNodes()
	nodeMetrics := ListNodeMetricss()

	for index, node := range nodes {

		allPods := AllPodsOnNode(node.Name)
		requestCpuCores, limitCpuCores := SumCpuResources(allPods)
		requestMemoryBytes, limitMemoryBytes := SumMemoryResources(allPods)

		utilizedCores := float64(0)
		utilizedMemory := int64(0)
		if len(nodeMetrics) > 0 {
			// Find the corresponding node metrics
			var nodeMetric *v1metrics.NodeMetrics
			for _, nm := range nodeMetrics {
				if nm.Name == node.Name {
					nodeMetric = &nm
					break
				}
			}
			if nodeMetric == nil {
				k8sLogger.Error("Failed to find node metrics for node", "node.name", node.Name)
				continue
			}

			// CPU
			cpuUsage, works := nodeMetric.Usage.Cpu().AsDec().Unscaled()
			if !works {
				k8sLogger.Error("Failed to get CPU usage for node", "node.name", node.Name)
			}
			if cpuUsage == 0 {
				cpuUsage = 1
			}
			utilizedCores = float64(cpuUsage) / 1000000000

			// Memory
			utilizedMemory, works = nodeMetric.Usage.Memory().AsInt64()
			if !works {
				k8sLogger.Error("Failed to get MEMORY usage for node", "node.name", node.Name)
			}
		}

		mem, _ := node.Status.Capacity.Memory().AsInt64()
		cpu, _ := node.Status.Capacity.Cpu().AsInt64()
		maxPods, _ := node.Status.Capacity.Pods().AsInt64()
		ephemeral, _ := node.Status.Capacity.StorageEphemeral().AsInt64()

		nodeStat := dtos.NodeStat{
			Name:                   node.Name,
			MaschineId:             node.Status.NodeInfo.MachineID,
			CpuInCores:             cpu,
			CpuInCoresUtilized:     utilizedCores,
			CpuInCoresRequested:    requestCpuCores,
			CpuInCoresLimited:      limitCpuCores,
			MemoryInBytes:          mem,
			MemoryInBytesUtilized:  utilizedMemory,
			MemoryInBytesRequested: requestMemoryBytes,
			MemoryInBytesLimited:   limitMemoryBytes,
			EphemeralInBytes:       ephemeral,
			MaxPods:                maxPods,
			TotalPods:              int64(len(allPods)),
			KubletVersion:          node.Status.NodeInfo.KubeletVersion,
			OsType:                 node.Status.NodeInfo.OperatingSystem,
			OsImage:                node.Status.NodeInfo.OSImage,
			Architecture:           node.Status.NodeInfo.Architecture,
		}
		result = append(result, nodeStat)
		//nodeStat.PrintPretty()
	}
	return result
}

func SumMemoryResources(pods []v1.Pod) (request int64, limit int64) {
	resultRequest := int64(0)
	resultLimit := int64(0)
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			memReq, works := container.Resources.Requests.Memory().AsDec().Unscaled()
			if works && memReq != 0 {
				resultRequest += memReq
			}
			memLim, works := container.Resources.Limits.Memory().AsDec().Unscaled()
			if works && memLim != 0 {
				resultLimit += memLim
			}
		}
	}
	return resultRequest, resultLimit
}

func SumCpuResources(pods []v1.Pod) (request float64, limit float64) {
	resultRequest := float64(0)
	resultLimit := float64(0)
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			resultRequest += float64(container.Resources.Requests.Cpu().MilliValue())
			resultLimit += float64(container.Resources.Limits.Cpu().MilliValue())
		}
	}
	if resultLimit > 0 {
		resultLimit = resultLimit / 1000
	}
	if resultRequest > 0 {
		resultRequest = resultRequest / 1000
	}
	return resultRequest, resultLimit
}

func GetK8sNode(name string) (*v1.Node, error) {
	clientset := clientProvider.K8sClientSet()
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
	node.Kind = "Node"
	node.APIVersion = "v1"
	return node, err
}
