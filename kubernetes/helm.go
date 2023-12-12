package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/structs"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
)

func CreateHelmChartCmd(helmReleaseName string, helmRepoName string, helmRepoUrl string, helmTask string, helmChartName string, helmFlags string, uninstalling bool, status *punq.SystemCheckStatus, doneFunction func()) {
	structs.CreateBashCommandGoRoutine("Add/Update Helm Repo & Execute chart.", fmt.Sprintf("helm repo add %s %s; helm repo update; helm %s %s %s %s", helmRepoName, helmRepoUrl, helmTask, helmReleaseName, helmChartName, helmFlags), uninstalling, status, doneFunction)
}

func DeleteHelmChart(job *structs.Job, helmReleaseName string, wg *sync.WaitGroup) *structs.Command {
	return structs.CreateBashCommand("Uninstall chart.", job, fmt.Sprintf("helm uninstall %s", helmReleaseName), wg)
}
