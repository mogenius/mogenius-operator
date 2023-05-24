package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"
)

func ExecuteHelmChartTask(job *structs.Job, helmReleaseName string, helmRepoName string, helmRepoUrl string, helmTask string, helmChartName string, wg *sync.WaitGroup) []*structs.Command {
	cmds := []*structs.Command{}

	addRepoCmd := structs.CreateBashCommand("Add repo.", job, fmt.Sprintf(" helm repo add %s %s; helm repo update", helmReleaseName, helmRepoUrl), wg)
	installCmd := structs.CreateBashCommand("Execute chart.", job, fmt.Sprintf("helm %s %s %s", helmTask, helmReleaseName, helmChartName), wg)

	cmds = append(cmds, addRepoCmd)
	cmds = append(cmds, installCmd)

	return cmds
}

func DeleteHelmChart(job *structs.Job, helmReleaseName string, wg *sync.WaitGroup) *structs.Command {
	return structs.CreateBashCommand("Uninstall chart.", job, fmt.Sprintf("helm uninstall %s", helmReleaseName), wg)
}

func InstallMogeniusNfsStorage(job *structs.Job, clusterProvider string, wg *sync.WaitGroup) []*structs.Command {
	cmds := []*structs.Command{}

	addRepoCmd := structs.CreateBashCommand("Install/Update helm repo.", job, "helm repo add mogenius-nfs-storage https://openebs.github.io/dynamic-nfs-provisioner; helm repo update", wg)
	cmds = append(cmds, addRepoCmd)

	nfsStorageClassStr := ""

	// "BRING_YOUR_OWN", "EKS", "AKS", "GKE", "DOCKER_ENTERPRISE", "DOKS", "LINODE", "IBM", "ACK", "OKE", "OTC", "OPEN_SHIFT"
	switch clusterProvider {
	case "EKS":
		nfsStorageClassStr = " --set-string nfsStorageClass.backendStorageClass=gp2"
	case "GKE":
		nfsStorageClassStr = " --set-string nfsStorageClass.backendStorageClass=standard-rwo"
	case "AKS":
		nfsStorageClassStr = " --set-string nfsStorageClass.backendStorageClass=default"
	default:
		// nothing to do
		logger.Log.Errorf("CLUSTERPROVIDER '%s' HAS NOT BEEN TESTED YET!", clusterProvider)
	}
	instRelCmd := structs.CreateBashCommand("Install helm release.", job, fmt.Sprintf("helm install mogenius-nfs-storage mo-openebs-nfs/nfs-provisioner -n %s --create-namespace --set analytics.enabled=false%s", utils.CONFIG.Kubernetes.OwnNamespace, nfsStorageClassStr), wg)
	cmds = append(cmds, instRelCmd)
	// storageClassCmd := CreateMogeniusNfsStorageClass(job, c, wg)
	// cmds = append(cmds, storageClassCmd)

	return cmds
}

func UninstallMogeniusNfsStorage(job *structs.Job, wg *sync.WaitGroup) []*structs.Command {
	cmds := []*structs.Command{}

	uninstRelCmd := structs.CreateBashCommand("Uninstall helm release.", job, fmt.Sprintf("helm uninstall mogenius-nfs-storage -n %s", utils.CONFIG.Kubernetes.OwnNamespace), wg)
	cmds = append(cmds, uninstRelCmd)
	// storageClassCmd := DeleteMogeniusNfsStorageClass(job, c, wg)
	// cmds = append(cmds, storageClassCmd)

	return cmds
}
