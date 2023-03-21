package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/structs"
	"sync"

	"github.com/gorilla/websocket"
)

func ExecuteHelmChartTask(job *structs.Job, helmReleaseName string, helmRepoName string, helmRepoUrl string, helmTask string, helmChartName string, c *websocket.Conn, wg *sync.WaitGroup) []*structs.Command {
	cmds := []*structs.Command{}

	addRepoCmd := structs.CreateBashCommand("Add repo.", job, fmt.Sprintf(" helm repo add %s %s; helm repo update", helmReleaseName, helmRepoUrl), c, wg)
	installCmd := structs.CreateBashCommand("Execute chart.", job, fmt.Sprintf("helm %s %s %s", helmTask, helmReleaseName, helmChartName), c, wg)

	cmds = append(cmds, addRepoCmd)
	cmds = append(cmds, installCmd)

	return cmds
}

func DeleteHelmChart(job *structs.Job, helmReleaseName string, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	return structs.CreateBashCommand("Uninstall chart.", job, fmt.Sprintf("helm uninstall %s", helmReleaseName), c, wg)
}
