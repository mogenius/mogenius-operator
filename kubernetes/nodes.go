package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/utils"
)

type NodeStat struct {
	Name             string `json:"name" validate:"required"`
	MaschineId       string `json:"maschineId" validate:"required"`
	Cpus             int64  `json:"cpus" validate:"required"`
	MemoryInBytes    int64  `json:"memoryInBytes" validate:"required"`
	EphemeralInBytes int64  `json:"ephemeralInBytes" validate:"required"`
	MaxPods          int64  `json:"maxPods" validate:"required"`
	KubletVersion    string `json:"kubletVersion" validate:"required"`
	OsType           string `json:"osType" validate:"required"`
	OsImage          string `json:"osImage" validate:"required"`
	Architecture     string `json:"architecture" validate:"required"`
}

func (o *NodeStat) PrintPretty() {
	fmt.Printf("%s: %s %s [%s/%s] - CPUs: %d, RAM: %s, Ephemeral: %s, MaxPods: %d\n",
		o.Name,
		o.KubletVersion,
		o.OsImage,
		o.OsType,
		o.Architecture,
		o.Cpus,
		utils.BytesToHumanReadable(o.MemoryInBytes),
		utils.BytesToHumanReadable(o.EphemeralInBytes),
		o.MaxPods,
	)
}

func GetNodeStats() []NodeStat {
	result := []NodeStat{}
	nodes := listNodes()

	for index, node := range nodes {
		mem, _ := node.Status.Capacity.Memory().AsInt64()
		cpu, _ := node.Status.Capacity.Cpu().AsInt64()
		maxPods, _ := node.Status.Capacity.Pods().AsInt64()
		ephemeral, _ := node.Status.Capacity.StorageEphemeral().AsInt64()

		nodeStat := NodeStat{
			Name:             fmt.Sprintf("Node-%d", index+1),
			MaschineId:       node.Status.NodeInfo.MachineID,
			Cpus:             cpu,
			MemoryInBytes:    mem,
			EphemeralInBytes: ephemeral,
			MaxPods:          maxPods,
			KubletVersion:    node.Status.NodeInfo.KubeletVersion,
			OsType:           node.Status.NodeInfo.OperatingSystem,
			OsImage:          node.Status.NodeInfo.OSImage,
			Architecture:     node.Status.NodeInfo.Architecture,
		}
		result = append(result, nodeStat)
		nodeStat.PrintPretty()
	}
	return result
}
