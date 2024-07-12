package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/structs"
	"os"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

var helmCache = cache.New(2*time.Hour, 30*time.Minute) // cache with default expiration time of 2 hours and cleanup interval of 30 minutes

func CreateHelmChart(helmReleaseName string, helmRepoName string, helmRepoUrl string, helmTask structs.HelmTaskEnum, helmChartName string, helmFlags string, successFunc func(), failFunc func(output string, err error)) {
	structs.CreateShellCommandGoRoutine("Add/Update Helm Repo & Execute chart.", fmt.Sprintf("helm repo add %s %s; helm repo update; helm %s %s %s %s", helmRepoName, helmRepoUrl, helmTask, helmReleaseName, helmChartName, helmFlags), successFunc, failFunc)
}

func DeleteHelmChart(job *structs.Job, helmReleaseName string, wg *sync.WaitGroup) {
	structs.CreateShellCommand("helm uninstall", "Uninstall chart", job, fmt.Sprintf("helm uninstall %s", helmReleaseName), wg)
}

func HelmStatus(namespace string, chartname string) structs.SystemCheckStatus {
	cacheKey := namespace + "/" + chartname
	cacheTime := 1 * time.Second

	// Check if the data is already in the cache
	if cachedData, found := helmCache.Get(cacheKey); found {
		return cachedData.(structs.SystemCheckStatus)
	}

	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmStatus Init Error: %s", err.Error())
		helmCache.Set(cacheKey, structs.UNKNOWN_STATUS, cacheTime)
		return structs.UNKNOWN_STATUS
	}

	get := action.NewGet(actionConfig)
	chart, err := get.Run(chartname)
	if err != nil && err.Error() != "release: not found" {
		K8sLogger.Errorf("HelmStatus List Error: %s", err.Error())
		helmCache.Set(cacheKey, structs.UNKNOWN_STATUS, cacheTime)
		return structs.UNKNOWN_STATUS
	}

	if chart == nil {
		helmCache.Set(cacheKey, structs.NOT_INSTALLED, cacheTime)
		return structs.NOT_INSTALLED
	} else {
		helmCache.Set(cacheKey, OurStatusFromHelmStatus(chart.Info.Status), cacheTime)
		return OurStatusFromHelmStatus(chart.Info.Status)
	}
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
