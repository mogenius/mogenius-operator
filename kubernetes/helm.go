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

func HelmStatus(namespace string, chartname string) release.Status {
	cacheKey := namespace + "/" + chartname
	cacheTime := 1 * time.Second

	// Check if the data is already in the cache
	if cachedData, found := helmCache.Get(cacheKey); found {
		return cachedData.(release.Status)
	}

	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), K8sLogger.Infof); err != nil {
		K8sLogger.Errorf("HelmStatus Init Error: %s", err.Error())
		helmCache.Set(cacheKey, release.StatusUnknown, cacheTime)
		return release.StatusUnknown
	}

	get := action.NewGet(actionConfig)
	chart, err := get.Run(chartname)
	if err != nil && err.Error() != "release: not found" {
		K8sLogger.Errorf("HelmStatus List Error: %s", err.Error())
		helmCache.Set(cacheKey, release.StatusUnknown, cacheTime)
		return release.StatusUnknown
	}

	if chart == nil {
		return release.StatusUnknown
	} else {
		return chart.Info.Status
	}
}
