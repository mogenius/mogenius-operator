package structs

import (
	"time"
)

type PodStats struct {
	Namespace             string    `json:"namespace"`
	PodName               string    `json:"podName"`
	ContainerName         string    `json:"containerName"`
	Cpu                   int64     `json:"cpu"`
	CpuLimit              int64     `json:"cpuLimit"`
	Memory                int64     `json:"memory"`
	MemoryLimit           int64     `json:"memoryLimit"`
	EphemeralStorage      int64     `json:"ephemeralStorage"`
	EphemeralStorageLimit int64     `json:"ephemeralStorageLimit"`
	StartTime             time.Time `json:"startTime"`
	CreatedAt             time.Time `json:"createdAt"`
}
