package structs

import (
	"fmt"
	"mogenius-k8s-manager/src/utils"

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
	CreatedAt          string            `json:"createdAt"` // when the entry was written into the storage
	SocketConnections  map[string]uint64 `json:"socketConnections"`
}

type SocketConnections struct {
	LastUpdate  string            `json:"lastUpdate"`
	Connections map[string]uint64 `json:"connections"`
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
}

func (data *InterfaceStats) SumOrReplace(dataToAdd *InterfaceStats) {
	if dataToAdd.TransmitStartBytes > data.TransmitStartBytes || dataToAdd.ReceivedStartBytes > data.ReceivedStartBytes {
		// new startRX+startTX means an reset of the counters
		data.TransmitStartBytes = dataToAdd.TransmitStartBytes
		data.ReceivedStartBytes = dataToAdd.ReceivedStartBytes

		data.PacketsSum = dataToAdd.PacketsSum
		data.TransmitBytes = dataToAdd.TransmitBytes
		data.ReceivedBytes = dataToAdd.ReceivedBytes
		data.UnknownBytes = dataToAdd.UnknownBytes
		data.LocalTransmitBytes = dataToAdd.LocalTransmitBytes
		data.LocalReceivedBytes = dataToAdd.LocalReceivedBytes
	} else {
		// just sum the values if startRX+startTX is the same (it changes if the traffic collector restarts)
		data.PacketsSum += dataToAdd.PacketsSum
		data.TransmitBytes += dataToAdd.TransmitBytes
		data.ReceivedBytes += dataToAdd.ReceivedBytes
		data.UnknownBytes += dataToAdd.UnknownBytes
		data.LocalTransmitBytes += dataToAdd.LocalTransmitBytes
		data.LocalReceivedBytes += dataToAdd.LocalReceivedBytes
	}
}

func (data *InterfaceStats) PrintInfo() {
	message := fmt.Sprintf("%s -> Packets: %d, Send: %s | Received %s\n", data.PodName, data.PacketsSum, utils.BytesToHumanReadable(int64(data.TransmitBytes+data.TransmitStartBytes+data.LocalTransmitBytes)), utils.BytesToHumanReadable(int64(data.ReceivedBytes+data.ReceivedStartBytes+data.LocalReceivedBytes)))
	structsLogger.Info(message)
}

func UnmarshalInterfaceStats(dst *InterfaceStats, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalSocketConnections(dst *SocketConnections, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalInterfaceStatsWithoutSocketConnections(dst *InterfaceStats, data []byte) error {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(data, dst)
	if err != nil {
		return err
	}

	dst.SocketConnections = make(map[string]uint64)

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
