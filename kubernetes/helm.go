package kubernetes

import (
	"fmt"
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

	addRepoCmd := structs.CreateBashCommand("Install/Update helm repo.", job, "helm repo add mo-openebs-nfs https://openebs.github.io/dynamic-nfs-provisioner; helm repo update", wg)
	cmds = append(cmds, addRepoCmd)
	// AWS --set-string nfsStorageClass.backendStorageClass=gp2
	// GCP --set-string nfsStorageClass.backendStorageClass=standard-rwo
	// AZRUE --set-string nfsStorageClass.backendStorageClass=default
	nfsStorageClassStr := ""
	if clusterProvider == "AWS" {
		nfsStorageClassStr = " --set-string nfsStorageClass.backendStorageClass=gp2"
	}
	if clusterProvider == "GCP" {
		nfsStorageClassStr = " --set-string nfsStorageClass.backendStorageClass=standard-rwo"
	}
	if clusterProvider == "AZURE" {
		nfsStorageClassStr = " --set-string nfsStorageClass.backendStorageClass=default"
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
