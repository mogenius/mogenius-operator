package podstatscollector

import (
	"time"
)

type NodeMetrics struct {
	Node Node  `json:"node"`
	Pods []Pod `json:"pods"`
}

type Node struct {
	NodeName         string            `json:"nodeName"`
	SystemContainers []SystemContainer `json:"systemContainers"`
	StartTime        time.Time         `json:"startTime"`
	CPU              CPU               `json:"cpu"`
	Memory           Memory            `json:"memory"`
	Network          Network           `json:"network"`
	FS               FileSystem        `json:"fs"`
	Runtime          Runtime           `json:"runtime"`
	RLimit           RLimit            `json:"rlimit"`
	Swap             Swap              `json:"swap"`
}

type SystemContainer struct {
	Name      string    `json:"name"`
	StartTime time.Time `json:"startTime"`
	CPU       CPU       `json:"cpu"`
	Memory    Memory    `json:"memory"`
	Swap      Swap      `json:"swap"`
}

type CPU struct {
	Time                 time.Time `json:"time"`
	UsageNanoCores       uint64    `json:"usageNanoCores"`
	UsageCoreNanoSeconds uint64    `json:"usageCoreNanoSeconds"`
}

type Memory struct {
	Time            time.Time `json:"time"`
	AvailableBytes  uint64    `json:"availableBytes,omitempty"`
	UsageBytes      uint64    `json:"usageBytes"`
	WorkingSetBytes uint64    `json:"workingSetBytes"`
	RSSBytes        uint64    `json:"rssBytes"`
	PageFaults      uint64    `json:"pageFaults"`
	MajorPageFaults uint64    `json:"majorPageFaults"`
}

type Swap struct {
	Time               time.Time `json:"time"`
	SwapAvailableBytes uint64    `json:"swapAvailableBytes,omitempty"`
	SwapUsageBytes     uint64    `json:"swapUsageBytes"`
}

type Network struct {
	Time       time.Time   `json:"time"`
	Name       string      `json:"name"`
	RXBytes    uint64      `json:"rxBytes"`
	RXErrors   uint64      `json:"rxErrors"`
	TXBytes    uint64      `json:"txBytes"`
	TXErrors   uint64      `json:"txErrors"`
	Interfaces []Interface `json:"interfaces"`
}

type Interface struct {
	Name     string `json:"name"`
	RXBytes  uint64 `json:"rxBytes"`
	RXErrors uint64 `json:"rxErrors"`
	TXBytes  uint64 `json:"txBytes"`
	TXErrors uint64 `json:"txErrors"`
}

type FileSystem struct {
	Time           time.Time `json:"time"`
	AvailableBytes uint64    `json:"availableBytes"`
	CapacityBytes  uint64    `json:"capacityBytes"`
	UsedBytes      uint64    `json:"usedBytes"`
	InodesFree     uint64    `json:"inodesFree"`
	Inodes         uint64    `json:"inodes"`
	InodesUsed     uint64    `json:"inodesUsed"`
}

type Runtime struct {
	ImageFs FileSystem `json:"imageFs"`
}

type RLimit struct {
	Time    time.Time `json:"time"`
	Maxpid  uint64    `json:"maxpid"`
	Curproc uint64    `json:"curproc"`
}

type Pod struct {
	PodRef           PodReference `json:"podRef"`
	StartTime        time.Time    `json:"startTime"`
	Containers       []Container  `json:"containers"`
	CPU              CPU          `json:"cpu"`
	Memory           Memory       `json:"memory"`
	Volume           []Volume     `json:"volume,omitempty"`
	EphemeralStorage FileSystem   `json:"ephemeral-storage"`
	ProcessStats     ProcessStats `json:"process_stats"`
	Swap             Swap         `json:"swap"`
}

type PodReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

type Container struct {
	Name      string     `json:"name"`
	StartTime time.Time  `json:"startTime"`
	CPU       CPU        `json:"cpu"`
	Memory    Memory     `json:"memory"`
	RootFS    FileSystem `json:"rootfs,omitempty"`
	Logs      FileSystem `json:"logs,omitempty"`
	Swap      Swap       `json:"swap"`
}

type Volume struct {
	Time           time.Time `json:"time"`
	AvailableBytes uint64    `json:"availableBytes"`
	CapacityBytes  uint64    `json:"capacityBytes"`
	UsedBytes      uint64    `json:"usedBytes"`
	InodesFree     uint64    `json:"inodesFree"`
	Inodes         uint64    `json:"inodes"`
	InodesUsed     uint64    `json:"inodesUsed"`
	Name           string    `json:"name"`
}

type ProcessStats struct {
	ProcessCount uint64 `json:"process_count"`
}
