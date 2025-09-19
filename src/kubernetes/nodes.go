package kubernetes

import (
	v1 "k8s.io/api/core/v1"
)

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
