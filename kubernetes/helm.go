package kubernetes

import (
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os"
	"strings"
	"sync"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	punq "github.com/mogenius/punq/kubernetes"
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

func HelmStatus(namespace string, chartname string) punq.SystemCheckStatus {
	settings := cli.New()
	settings.SetNamespace(namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), log.Infof); err != nil {
		log.Errorf("HelmStatus Init Error: %s", err.Error())
		return punq.UNKNOWN_STATUS
	}

	list := action.NewList(actionConfig)
	releases, err := list.Run()
	if err != nil {
		log.Errorf("HelmStatus List Error: %s", err.Error())
		return punq.UNKNOWN_STATUS
	}

	for _, rel := range releases {
		if rel.Name == chartname {
			return OurStatusFromHelmStatus(rel.Info.Status)
		}
	}

	return punq.NOT_INSTALLED
}

func OurStatusFromHelmStatus(status release.Status) punq.SystemCheckStatus {
	switch status {
	case release.StatusUnknown:
		return punq.UNKNOWN_STATUS
	case release.StatusDeployed, release.StatusSuperseded:
		return punq.INSTALLED
	case release.StatusUninstalled, release.StatusFailed:
		return punq.NOT_INSTALLED
	case release.StatusUninstalling:
		return punq.UNINSTALLING
	case release.StatusPendingInstall, release.StatusPendingUpgrade, release.StatusPendingRollback:
		return punq.INSTALLING
	default:
		return punq.UNKNOWN_STATUS
	}
}

// type PuncOperationResult {
// 	interface{}
// }

type PuncOperation func(string, string) (string, error)
type PuncOperationClusterIssuer func(string, *string) (*v1.ClusterIssuer, error)

type EntryProps struct {
	Name             string
	HelmChartIndex   string
	InstalledErrMsg  string
	Description      string
	InstallPattern   string
	UninstallPattern string
	UpgradePattern   string
	// PuncOperation    PuncOperation
}

func entryFactory(ep EntryProps, isAlreadyInstalled bool, message string) punq.SystemCheckEntry {

	if !isAlreadyInstalled {
		message = fmt.Sprintf("%s is not installed.\n.", ep.Name)
	}
	currentChartVersion := GetMostCurrentHelmChartVersion(ep.HelmChartIndex, ep.Name)

	chartEntry := punq.CreateSystemCheckEntry(ep.Name, isAlreadyInstalled, message, ep.Description, false, true, response, currentChartVersion)
	chartEntry.InstallPattern = ep.InstallPattern
	chartEntry.UninstallPattern = ep.UninstallPattern
	chartEntry.UpgradePattern = ep.UpgradePattern
	chartEntry.Status = HelmStatus(utils.CONFIG.Kubernetes.OwnNamespace, ep.Name)

	return chartEntry
}

func EntryFactoryOp(ep EntryProps, operation PuncOperation) punq.SystemCheckEntry {
	chartVersion, chartInstalledErr := operation(utils.CONFIG.Kubernetes.OwnNamespace, ep.Name)
	message := fmt.Sprintf("%s (Version: %s) is installed.", ep.Name, chartVersion)
	isAlreadyInstalled := chartInstalledErr == nil

	return entryFactory(ep, isAlreadyInstalled, message)
}

func EntryFactoryOpClusterIssuer(ep EntryProps, operation PuncOperationClusterIssuer) punq.SystemCheckEntry {
	_, chartInstalledErr := operation(utils.CONFIG.Kubernetes.OwnNamespace, &ep.Name)
	message := fmt.Sprintf("%s is installed.", ep.Name)
	isAlreadyInstalled := chartInstalledErr == nil

	return entryFactory(ep, isAlreadyInstalled, message)
}

func GetMostCurrentHelmChartVersion(url string, chartname string) string {
	url = addIndexYAMLtoURL(url)
	data, err := utils.GetVersionData(url)
	if err != nil {
		log.Errorf("Error getting helm chart version (%s/%s): %s", url, chartname, err)
		return ""
	}
	chartsArray := data.Entries[chartname]
	result := "NO_VERSION_FOUND"
	if len(chartsArray) > 0 {
		result = chartsArray[0].Version
	}

	return result
}

func addIndexYAMLtoURL(url string) string {
	if !strings.HasSuffix(url, "index.yaml") {
		// Check if the URL ends with a slash; if not, add one.
		if !strings.HasSuffix(url, "/") {
			url += "/"
		}
		url += "index.yaml"
	}
	return url
}
