package services

import (
	mokubernetes "mogenius-k8s-manager/kubernetes"
	"mogenius-k8s-manager/structs"
	"sync"

	"github.com/gorilla/websocket"
)

func UpgradeK8sManager(r K8sManagerUpgradeRequest, c *websocket.Conn) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Upgrade mogenius platform", "UPGRADE", nil, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.UpgradeMyself(&job, r.Command, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func InstallHelmChart(r ClusterHelmRequest, c *websocket.Conn) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Install Helm Chart "+r.HelmReleaseName, r.NamespaceId, nil, nil, c)
	job.Start(c)
	job.AddCmds(mokubernetes.ExecuteHelmChartTask(&job, r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmTask, r.HelmChartName, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

func DeleteHelmChart(r ClusterHelmUninstallRequest, c *websocket.Conn) structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Delete Helm Chart "+r.HelmReleaseName, r.NamespaceId, nil, nil, c)
	job.Start(c)
	job.AddCmd(mokubernetes.DeleteHelmChart(&job, r.HelmReleaseName, c, &wg))
	wg.Wait()
	job.Finish(c)
	return job
}

type K8sManagerUpgradeRequest struct {
	Command string `json:"command"` // complete helm command from platform ui
}

func K8sManagerUpgradeRequestExample() K8sManagerUpgradeRequest {
	return K8sManagerUpgradeRequest{
		Command: "helm version",
	}
}

type ClusterHelmRequest struct {
	Namespace       string `json:"namespace"`
	NamespaceId     string `json:"namespaceId"`
	HelmRepoName    string `json:"helmRepoName"`
	HelmRepoUrl     string `json:"helmRepoUrl"`
	HelmReleaseName string `json:"helmReleaseName"`
	HelmChartName   string `json:"HelmChartName"`
	HelmTask        string `json:"helmTask"` // install, upgrade, uninstall
}

func ClusterHelmRequestExample() ClusterHelmRequest {
	return ClusterHelmRequest{
		Namespace:       "default",
		NamespaceId:     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		HelmRepoName:    "bitnami",
		HelmRepoUrl:     "https://charts.bitnami.com/bitnami",
		HelmReleaseName: "test-helm-release",
		HelmChartName:   "bitnami/nginx",
		HelmTask:        "install",
	}
}

type ClusterHelmUninstallRequest struct {
	NamespaceId     string `json:"namespaceId"`
	HelmReleaseName string `json:"helmReleaseName"`
}

func ClusterHelmUninstallRequestExample() ClusterHelmUninstallRequest {
	return ClusterHelmUninstallRequest{
		NamespaceId:     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		HelmReleaseName: "test-helm-release",
	}
}
