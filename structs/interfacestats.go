package structs

import (
	jsoniter "github.com/json-iterator/go"
)

type InterfaceStats struct {
	Ip                 string                         `json:"ip"`
	PodName            string                         `json:"podName"`
	Namespace          string                         `json:"namespace"`
	PacketsSum         uint64                         `json:"packetsSum"`
	TransmitBytes      uint64                         `json:"transmitBytes"`
	ReceivedBytes      uint64                         `json:"receivedBytes"`
	UnknownBytes       uint64                         `json:"unknownBytes"`
	LocalTransmitBytes uint64                         `json:"localTransmitBytes"`
	LocalReceivedBytes uint64                         `json:"localReceivedBytes"`
	TransmitStartBytes uint64                         `json:"transmitStartBytes"`
	ReceivedStartBytes uint64                         `json:"receivedStartBytes"`
	StartTime          string                         `json:"startTime"`
	CreatedAt          string                         `json:"createdAt"`
	Connections        map[uint64]InterfaceConnection `json:"connections"`
}

type InterfaceConnection struct {
	Ip1       string `json:"ip1"`
	Ip2       string `json:"ip2"`
	PacketSum uint64 `json:"packetSum"`
}

func UnmarshalInterfaceStats(dst *InterfaceStats, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func (data *InterfaceStats) ToBytes() []byte {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return bytes
}
