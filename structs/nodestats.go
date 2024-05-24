package structs

import jsoniter "github.com/json-iterator/go"

type NodeStats struct {
	Name                  string `json:"name"`
	PodCount              int    `json:"podCount"`
	StartTime             string `json:"startTime"`
	CpuUsageNanoCores     int64  `json:"cpuUsageNanoCores"`
	MemoryUsageBytes      int64  `json:"memoryUsageBytes"`
	MemoryAvailableBytes  int64  `json:"memoryAvailableBytes"`
	MemoryWorkingSetBytes int64  `json:"memoryWorkingSetBytes"`
	NetworkTxBytes        int64  `json:"networkTxBytes"`
	NetworkRxBytes        int64  `json:"networkRxBytes"`
	FsAvailableBytes      int64  `json:"fsAvailableBytes"`
	FsCapacityBytes       int64  `json:"fsCapacityBytes"`
	FsUsedBytes           int64  `json:"fsUsedBytes"`
	CreatedAt             string `json:"createdAt"`
}

func UnmarshalNodeStats(dst *NodeStats, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func (data *NodeStats) ToBytes() []byte {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return bytes
}
