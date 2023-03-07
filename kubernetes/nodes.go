package kubernetes

import (
	"mogenius-k8s-manager/utils"
	"time"
)

func MonitorMetrics() {
	for {
		// nodes := listNodeMetrics()
		// for _, node := range nodes {
		// 	fmt.Println(node)
		// }

		// datagram := structs.CreateDatagramFrom("cluster-resource-info", nodes, c)
		// fmt.Println(datagram)

		time.Sleep(time.Duration(utils.CONFIG.Kubernetes.CheckForNodeStats) * time.Second)
	}
}
