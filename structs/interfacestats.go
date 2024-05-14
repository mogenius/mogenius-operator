package structs

import (
	"mogenius-k8s-manager/utils"

	jsoniter "github.com/json-iterator/go"
)

type InterfaceStats struct {
	Ip                 string            `json:"ip"`
	PodName            string            `json:"podName"`
	Namespace          string            `json:"namespace"`
	PacketsSum         uint64            `json:"packetsSum"`
	TransmitBytes      uint64            `json:"transmitBytes"`
	ReceivedBytes      uint64            `json:"receivedBytes"`
	UnknownBytes       uint64            `json:"unknownBytes"`
	LocalTransmitBytes uint64            `json:"localTransmitBytes"`
	LocalReceivedBytes uint64            `json:"localReceivedBytes"`
	TransmitStartBytes uint64            `json:"transmitStartBytes"`
	ReceivedStartBytes uint64            `json:"receivedStartBytes"`
	StartTime          string            `json:"startTime"` // start time of the Interface/Pod
	CreatedAt          string            `json:"createdAt"` // when the entry was written into boltDB
	SocketConnections  map[string]uint64 `json:"socketConnections"`
}

type InterfaceConnection struct {
	Ip1       string `json:"ip1"`
	Ip2       string `json:"ip2"`
	PacketSum uint64 `json:"packetSum"`
}

func (data *InterfaceStats) Sum(dataToAdd *InterfaceStats) {
	data.PacketsSum += dataToAdd.PacketsSum
	data.TransmitBytes += dataToAdd.TransmitBytes
	data.ReceivedBytes += dataToAdd.ReceivedBytes
	data.UnknownBytes += dataToAdd.UnknownBytes
	data.LocalTransmitBytes += dataToAdd.LocalTransmitBytes
	data.LocalReceivedBytes += dataToAdd.LocalReceivedBytes
	data.TransmitStartBytes += dataToAdd.TransmitStartBytes
	data.ReceivedStartBytes += dataToAdd.ReceivedStartBytes

	// Merge socket connections
	for key, newValue := range dataToAdd.SocketConnections {
		//value, _ := data.SocketConnections[key]
		data.SocketConnections[key] = newValue
	}

	// overwrite start time to determine the earliest start time
	if utils.IsFirstTimestampNewer(dataToAdd.StartTime, data.StartTime) {
		data.StartTime = dataToAdd.StartTime
	}
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
