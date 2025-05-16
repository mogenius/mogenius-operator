package structs

import jsoniter "github.com/json-iterator/go"

type PodStats struct {
	Namespace             string `json:"namespace"`
	PodName               string `json:"podName"`
	ContainerName         string `json:"containerName"`
	Cpu                   int64  `json:"cpu"`
	CpuLimit              int64  `json:"cpuLimit"`
	Memory                int64  `json:"memory"`
	MemoryLimit           int64  `json:"memoryLimit"`
	EphemeralStorage      int64  `json:"ephemeralStorage"`
	EphemeralStorageLimit int64  `json:"ephemeralStorageLimit"`
	StartTime             string `json:"startTime"`
	CreatedAt             string `json:"createdAt"`
}

func (data *PodStats) ToBytes() []byte {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return bytes
}

type Stats struct {
	Cluster               string `json:"cluster"`
	Namespace             string `json:"namespace"`
	PodName               string `json:"podName"`
	Cpu                   int64  `json:"cpu"`
	CpuLimit              int64  `json:"cpuLimit"`
	Memory                int64  `json:"memory"`
	MemoryLimit           int64  `json:"memoryLimit"`
	EphemeralStorageLimit int64  `json:"ephemeralStorageLimit"`
	StartTime             string `json:"startTime"`
}
