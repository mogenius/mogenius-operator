package dtos

import "mogenius-operator/src/structs"

type NodeStat struct {
	Name                   string                `json:"name" validate:"required"`
	MaschineId             string                `json:"maschineId" validate:"required"`
	CpuInCores             int64                 `json:"cpuInCores" validate:"required"`
	CpuInCoresUtilized     float64               `json:"cpuInCoresUtilized" validate:"required"`
	CpuInCoresRequested    float64               `json:"cpuInCoresRequested" validate:"required"`
	CpuInCoresLimited      float64               `json:"cpuInCoresLimited" validate:"required"`
	MachineStats           *structs.MachineStats `json:"machineStats"`
	MemoryInBytes          int64                 `json:"memoryInBytes" validate:"required"`
	MemoryInBytesUtilized  int64                 `json:"memoryInBytesUtilized" validate:"required"`
	MemoryInBytesRequested int64                 `json:"memoryInBytesRequested" validate:"required"`
	MemoryInBytesLimited   int64                 `json:"memoryInBytesLimited" validate:"required"`
	EphemeralInBytes       int64                 `json:"ephemeralInBytes" validate:"required"`
	MaxPods                int64                 `json:"maxPods" validate:"required"`
	TotalPods              int64                 `json:"totalPods" validate:"required"`
	KubletVersion          string                `json:"kubletVersion" validate:"required"`
	OsType                 string                `json:"osType" validate:"required"`
	OsImage                string                `json:"osImage" validate:"required"`
	OsKernelVersion        string                `json:"osKernelVersion" validate:"required"`
	Architecture           string                `json:"architecture" validate:"required"`
}
