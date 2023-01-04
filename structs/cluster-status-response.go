package structs

type ClusterStatusResponse struct {
	ClusterName           string `json:"clusterName"`
	Pods                  int    `json:"pods"`
	CpuInMilliCores       int    `json:"cpu"`
	CpuLimitInMilliCores  int    `json:"cpuLimit"`
	Memory                string `json:"memory"`
	MemoryLimit           string `json:"memoryLimit"`
	EphemeralStorageLimit string `json:"ephemeralStorageLimit"`
}
