package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"os/exec"
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

func DescribeK8sNode(name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "node", name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(err.Error())
	}
	return WorkloadResult(string(output))
}
