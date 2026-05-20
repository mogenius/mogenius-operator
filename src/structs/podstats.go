package structs

import (
	"time"
)

type PodStats struct {
	// Namespace is encoded with omitempty so writers can zero it before
	// persisting (the namespace is already part of the stream key); readers
	// fill it back from the query context. Older entries that still have
	// the field stored are unmarshalled normally.
	Namespace             string    `json:"namespace,omitempty"`
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
