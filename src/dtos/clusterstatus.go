package dtos

import (
	"mogenius-k8s-manager/src/utils"
)

type ClusterStatusDto struct {
	ClusterName                  string                `json:"clusterName"`
	Pods                         int                   `json:"pods"`
	PodCpuUsageInMilliCores      int                   `json:"podCpuUsageInMilliCores"`
	PodCpuLimitInMilliCores      int                   `json:"podCpuLimitInMilliCores"`
	PodMemoryUsageInBytes        int64                 `json:"podMemoryUsageInBytes"`
	PodMemoryLimitInBytes        int64                 `json:"podMemoryLimitInBytes"`
	EphemeralStorageLimitInBytes int64                 `json:"ephemeralStorageLimitInBytes"`
	CurrentTime                  string                `json:"currentTime"`
	KubernetesVersion            string                `json:"kubernetesVersion"`
	Platform                     string                `json:"platform"`
	Country                      *utils.CountryDetails `json:"country"`
}
