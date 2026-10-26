package kubernetes

import (
	v1 "k8s.io/api/core/v1"
)

func SumMemoryResources(pods []v1.Pod) (request int64, limit int64) {
	resultRequest := int64(0)
	resultLimit := int64(0)
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			// Value() is the quantity in bytes. AsDec().Unscaled() returned
			// the unscaled mantissa, i.e. "1G" counted as 1 byte and "1.5Gi"
			// as 1.5e18.
			resultRequest += container.Resources.Requests.Memory().Value()
			resultLimit += container.Resources.Limits.Memory().Value()
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
