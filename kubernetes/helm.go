package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/structs"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

func CreateHelmChartCmd(helmReleaseName string, helmRepoName string, helmRepoUrl string, helmTask structs.HelmTaskEnum, helmChartName string, helmFlags string, successFunc func(), failFunc func(output string, err error)) {
	structs.CreateShellCommandGoRoutine("Add/Update Helm Repo & Execute chart.", fmt.Sprintf("helm repo add %s %s; helm repo update; helm %s %s %s %s", helmRepoName, helmRepoUrl, helmTask, helmReleaseName, helmChartName, helmFlags), successFunc, failFunc)
}

func DeleteHelmChart(job *structs.Job, helmReleaseName string, wg *sync.WaitGroup) {
	structs.CreateShellCommand("helm uninstall", "Uninstall chart", job, fmt.Sprintf("helm uninstall %s", helmReleaseName), wg)
}

func HelmStatus(namespace string, chartname string) structs.SystemCheckStatus {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), log.Infof); err != nil {
		log.Errorf("HelmStatus Init Error: %s", err.Error())
		return structs.UNKNOWN_STATUS
	}

	list := action.NewList(actionConfig)
	releases, err := list.Run()
	if err != nil {
		log.Errorf("HelmStatus List Error: %s", err.Error())
		return structs.UNKNOWN_STATUS
	}

	for _, rel := range releases {
		if rel.Name == chartname {
			return OurStatusFromHelmStatus(rel.Info.Status)
		}
	}

	return structs.NOT_INSTALLED
}

func OurStatusFromHelmStatus(status release.Status) structs.SystemCheckStatus {
	switch status {
	case release.StatusUnknown:
		return structs.UNKNOWN_STATUS
	case release.StatusDeployed, release.StatusSuperseded:
		return structs.INSTALLED
	case release.StatusUninstalled, release.StatusFailed:
		return structs.NOT_INSTALLED
	case release.StatusUninstalling:
		return structs.UNINSTALLING
	case release.StatusPendingInstall, release.StatusPendingUpgrade, release.StatusPendingRollback:
		return structs.INSTALLING
	default:
		return structs.UNKNOWN_STATUS
	}
}
