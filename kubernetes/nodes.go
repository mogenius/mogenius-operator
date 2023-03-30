package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
)

func GetNodeStats() []dtos.NodeStat {
	result := []dtos.NodeStat{}
	nodes := ListNodes()

	for index, node := range nodes {
		mem, _ := node.Status.Capacity.Memory().AsInt64()
		cpu, _ := node.Status.Capacity.Cpu().AsInt64()
		maxPods, _ := node.Status.Capacity.Pods().AsInt64()
		ephemeral, _ := node.Status.Capacity.StorageEphemeral().AsInt64()

		nodeStat := dtos.NodeStat{
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
