package services

import (
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
)

var ALL_REQUESTS = []string{
	"ClusterStatus",
	"cicd/build-info GET",
	"cicd/build-info-array POST",
	"cicd/build-log GET",
}

func ExecuteRequest(datagram structs.Datagram) interface{} {
	switch datagram.Pattern {
	case "ClusterStatus":
		return mokubernetes.ClusterStatus()
	case "cicd/build-info GET":
		return BuildInfo(datagram.Payload.(BuildInfoRequest))
	case "cicd/build-info-array POST":
		return BuildInfoArray(datagram.Payload.(BuildInfoArrayRequest))
	case "cicd/build-log GET":
		return BuildLog(datagram.Payload.(BuildLogRequest))
	}
	datagram.Err = "Pattern not found"
	return datagram
}
