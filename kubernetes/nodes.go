package kubernetes

// import (
// 	"mogenius-k8s-manager/logger"
// 	"mogenius-k8s-manager/structs"
// 	"mogenius-k8s-manager/utils"
// 	"time"

// 	v1 "k8s.io/api/core/v1"
// )

// func Monitor(useLocalKubeConfig bool) {
// 	var provider *KubeProviderMetrics
// 	var err error
// 	if useLocalKubeConfig {
// 		provider, err = NewKubeProviderMetricsLocal()
// 	} else {
// 		provider, err = NewKubeProviderMetricsInCluster()
// 	}
// 	if err != nil {
// 		panic(err)
// 	}

// 	for {
// 		var nodes = make(map[string]v1.Node)

// 		pods := listAllPods(useLocalKubeConfig)
// 		for _, pod := range pods {
// 			currentPods[pod.Name] = pod
// 		}

// 		result, err := podStats(provider, currentPods)
// 		if err != nil {
// 			logger.Log.Error("podStats:", err)
// 		}

// 		datagram := structs.CreateDatagramFrom("pod-stats-collector-data", result)
// 		sendDataWs(datagram)
// 		printEntriesTable(result)

// 		time.Sleep(time.Duration(utils.CONFIG.General.UpdateInterval) * time.Second)
// 	}
// }
