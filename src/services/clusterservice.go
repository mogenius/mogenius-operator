package services

import (
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/websocket"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	NameMetricsServer         = "Metrics Server"
	NameMetalLB               = "MetalLB (LoadBalancer)"
	NameIngressController     = "Ingress Controller"
	NameClusterIssuerResource = "letsencrypt-cluster-issuer"
	NameKepler                = "Kepler"
	NameNfsStorageClass       = "nfs-storageclass"
)

const (
	MetricsHelmIndex                  = "https://kubernetes-sigs.github.io/metrics-server"
	IngressControllerTraefikHelmIndex = "https://traefik.github.io/charts"
	CertManagerHelmIndex              = "https://charts.jetstack.io"
	KeplerHelmIndex                   = "https://sustainable-computing-io.github.io/kepler-helm-chart"
	MetalLBHelmIndex                  = "https://metallb.github.io/metallb"
	MogeniusHelmIndex                 = "https://helm.mogenius.com/public"
)

func CreateMogeniusNfsVolume(eventClient websocket.WebsocketClient, r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob(eventClient, "Create mogenius nfs-volume.", r.NamespaceName, "", "", serviceLogger)
	job.Start(eventClient)
	// FOR K8SMANAGER
	mokubernetes.CreateMogeniusNfsServiceSync(eventClient, job, r.NamespaceName, r.VolumeName)
	wg.Go(func() {
		mokubernetes.CreateMogeniusNfsPersistentVolumeClaim(eventClient, job, r.NamespaceName, r.VolumeName, r.SizeInGb)
	})
	wg.Go(func() {
		mokubernetes.CreateMogeniusNfsDeployment(eventClient, job, r.NamespaceName, r.VolumeName)
	})
	// FOR SERVICES THAT WANT TO MOUNT
	wg.Go(func() {
		mokubernetes.CreateMogeniusNfsPersistentVolumeForService(eventClient, job, r.NamespaceName, r.VolumeName, r.SizeInGb)
	})
	wg.Go(func() {
		mokubernetes.CreateMogeniusNfsPersistentVolumeClaimForService(eventClient, job, r.NamespaceName, r.VolumeName, r.SizeInGb)
	})
	wg.Wait()
	job.Finish(eventClient)

	nfsService := mokubernetes.ServiceForNfsVolume(r.NamespaceName, r.VolumeName)
	mokubernetes.Mount(r.NamespaceName, r.VolumeName, nfsService)

	return job.DefaultReponse()
}

func DeleteMogeniusNfsVolume(eventClient websocket.WebsocketClient, r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob(eventClient, "Delete mogenius nfs-volume.", r.NamespaceName, "", "", serviceLogger)
	job.Start(eventClient)
	// FOR K8SMANAGER
	wg.Go(func() {
		mokubernetes.DeleteMogeniusNfsDeployment(eventClient, job, r.NamespaceName, r.VolumeName)
	})
	wg.Go(func() {
		mokubernetes.DeleteMogeniusNfsService(eventClient, job, r.NamespaceName, r.VolumeName)
	})
	wg.Go(func() {
		mokubernetes.DeleteMogeniusNfsPersistentVolumeClaim(eventClient, job, r.NamespaceName, r.VolumeName)
	})
	// FOR SERVICES THAT WANT TO MOUNT
	wg.Go(func() {
		mokubernetes.DeleteMogeniusNfsPersistentVolumeForService(eventClient, job, r.VolumeName, r.NamespaceName)
	})
	wg.Go(func() {
		mokubernetes.DeleteMogeniusNfsPersistentVolumeClaimForService(eventClient, job, r.NamespaceName, r.VolumeName)
	})
	wg.Wait()
	job.Finish(eventClient)

	mokubernetes.Umount(r.NamespaceName, r.VolumeName)

	return job.DefaultReponse()
}

func StatsMogeniusNfsVolume(r NfsVolumeStatsRequest) NfsVolumeStatsResponse {
	mountPath := utils.MountPath(r.NamespaceName, r.VolumeName, "/", clientProvider.RunsInCluster())
	free, used, total, _ := diskUsage(mountPath)
	result := NfsVolumeStatsResponse{
		VolumeName: r.VolumeName,
		FreeBytes:  free,
		UsedBytes:  used,
		TotalBytes: total,
	}

	serviceLogger.Info("💾: nfs volume stats",
		"mountPath", mountPath,
		"usedBytes", utils.BytesToHumanReadable(int64(result.UsedBytes)),
		"totalBytes", utils.BytesToHumanReadable(int64(result.TotalBytes)),
		"freeBytes", utils.BytesToHumanReadable(int64(result.FreeBytes)),
	)
	return result
}

func diskUsage(mountPath string) (uint64, uint64, uint64, error) {
	usage, err := disk.Usage(mountPath)
	if err != nil {
		serviceLogger.Error("StatsMogeniusNfsVolume",
			"mountPath", mountPath,
			"error", err,
		)
		return 0, 0, 0, err
	} else {
		return usage.Free, usage.Used, usage.Total, nil
	}
}

func sumAllBytesOfFolder(root string) uint64 {
	var total uint64
	var wg sync.WaitGroup
	var sumWg sync.WaitGroup
	fileSizes := make(chan uint64)

	sumWg.Go(func() {
		for size := range fileSizes {
			total += size
		}
	})

	// Walk the file tree concurrently.
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			wg.Go(func() {
				info, err := d.Info()
				if err != nil {
					return // handle error
				}
				fileSizes <- uint64(info.Size())
			})
		}
		return nil
	})
	if err != nil {
		serviceLogger.Error("Error while summing bytes in path", "error", err.Error())
	}

	wg.Wait()
	close(fileSizes) // Close channel to finish summing
	sumWg.Wait()     // Wait for summing to complete

	return total
}

type ClusterHelmRequest struct {
	Namespace        string `json:"namespace" validate:"required"`
	NamespaceId      string `json:"namespaceId" validate:"required"`
	HelmRepoName     string `json:"helmRepoName" validate:"required"`
	HelmRepoUrl      string `json:"helmRepoUrl" validate:"required"`
	HelmReleaseName  string `json:"helmReleaseName" validate:"required"`
	HelmChartName    string `json:"helmChartName" validate:"required"`
	HelmChartVersion string `json:"helmChartVersion"`
	HelmValues       string `json:"helmValues" validate:"required"`
}

type ClusterListWorkloads struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Prefix        string `json:"prefix"`
}

type NfsVolumeRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
	VolumeName    string `json:"volumeName" validate:"required"`
	SizeInGb      int    `json:"sizeInGb" validate:"required"`
}

type NfsVolumeStatsRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
	VolumeName    string `json:"volumeName" validate:"required"`
}

type NfsVolumeStatsResponse struct {
	VolumeName string `json:"volumeName"`
	TotalBytes uint64 `json:"totalBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
	UsedBytes  uint64 `json:"usedBytes"`
}

type NfsStatusRequest struct {
	Name             string `json:"name" validate:"required"`
	Namespace        string `json:"namespace"`
	StorageAPIObject string `json:"type" validate:"required"`
}

type NfsStatusResponse struct {
	VolumeName    string                `json:"volumeName"`
	NamespaceName string                `json:"namespaceName"`
	TotalBytes    uint64                `json:"totalBytes"`
	FreeBytes     uint64                `json:"freeBytes"`
	UsedBytes     uint64                `json:"usedBytes"`
	Status        VolumeStatusType      `json:"status"`
	Messages      []VolumeStatusMessage `json:"messages,omitempty"`
	UsedByPods    []string              `json:"usedByPods,omitempty"`
}

func InstallMetricsServer() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameMetricsServer,
		HelmRepoUrl:     MetricsHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetricsServer,
		HelmChartName:   utils.HelmReleaseNameMetricsServer + "/" + utils.HelmReleaseNameMetricsServer,
		HelmValues: `args:
  - "--kubelet-insecure-tls"
  - "--secure-port=10250"
  - "--cert-dir=/tmp"
  - "--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname"
  - "--kubelet-use-node-status-port"
  - "--metric-resolution=15s"
`,
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
}

func UpgradeMetricsServer() (string, error) {
	r := helm.HelmChartInstallUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Release:   utils.HelmReleaseNameMetricsServer,
		Chart:     utils.HelmReleaseNameMetricsServer + "/" + utils.HelmReleaseNameMetricsServer,
		Values: `args:
  - "--kubelet-insecure-tls"
  - "--secure-port=10250"
  - "--cert-dir=/tmp"
  - "--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname"
  - "--kubelet-use-node-status-port"
  - "--metric-resolution=15s"
`,
	}
	return helm.HelmReleaseUpgrade(r)
}

func InstallIngressControllerTreafik() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameTraefik,
		HelmRepoUrl:     IngressControllerTraefikHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameTraefik,
		HelmChartName:   utils.HelmReleaseNameTraefik + "/" + utils.HelmReleaseNameTraefik,
		HelmValues:      "",
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
}

func UpgradeIngressControllerTreafik() (string, error) {
	r := helm.HelmChartInstallUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Release:   utils.HelmReleaseNameTraefik,
		Chart:     utils.HelmReleaseNameTraefik + "/" + utils.HelmReleaseNameTraefik,
	}
	return helm.HelmReleaseUpgrade(r)
}

func InstallCertManager() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    "jetstack",
		HelmRepoUrl:     CertManagerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameCertManager,
		HelmChartName:   "jetstack/" + utils.HelmReleaseNameCertManager,
		HelmValues: fmt.Sprintf(`namespace: %s
startupapicheck:
  enabled: false
installCRDs: true
`, config.Get("MO_OWN_NAMESPACE")),
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
}

func UpgradeCertManager() (string, error) {
	r := helm.HelmChartInstallUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Release:   utils.HelmReleaseNameCertManager,
		Chart:     "jetstack/" + utils.HelmReleaseNameCertManager,
		Values:    "startupapicheck.enabled=false\ninstallCRDs=true",
	}
	return helm.HelmReleaseUpgrade(r)
}

func InstallMetalLb() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameMetalLb,
		HelmRepoUrl:     MetalLBHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameMetalLb,
		HelmChartName:   utils.HelmReleaseNameMetalLb + "/" + utils.HelmReleaseNameMetalLb,
		HelmValues:      "",
	}
	helmResultStr, err := helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
	if err == nil {
		for {
			// this is important because the control plane needs some time to make the CRDs available
			time.Sleep(1 * time.Second)
			err := mokubernetes.CreateYamlString(InstallAddressPool())
			if err != nil && !apierrors.IsAlreadyExists(err) {
				serviceLogger.Error("Error installing metallb address pool", "error", err)
			}
			if err != nil && apierrors.IsInternalError(err) {
				serviceLogger.Info("Control plane not ready. Waiting for metallb address pool installation ...")
			}
			if err == nil {
				break
			}
		}
	}
	return helmResultStr, err
}

func UpgradeMetalLb() (string, error) {
	r := helm.HelmChartInstallUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Release:   utils.HelmReleaseNameMetalLb,
		Chart:     utils.HelmReleaseNameMetalLb + "/" + utils.HelmReleaseNameMetalLb,
	}
	return helm.HelmReleaseUpgrade(r)
}

func InstallKepler() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameKepler,
		HelmRepoUrl:     KeplerHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameKepler,
		HelmChartName:   utils.HelmReleaseNameKepler + "/" + utils.HelmReleaseNameKepler,
		HelmValues: fmt.Sprintf(`global:
  namespace: "%s"
extraEnvVars:
  EXPOSE_IRQ_COUNTER_METRICS: "false"
  EXPOSE_KUBELET_METRICS: "false"
  ENABLE_PROCESS_METRICS: "false"
`, config.Get("MO_OWN_NAMESPACE")),
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
}

func UpgradeKepler() (string, error) {
	r := helm.HelmChartInstallUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Chart:     utils.HelmReleaseNameKepler + "/" + utils.HelmReleaseNameKepler,
		Release:   utils.HelmReleaseNameKepler,
		Values: fmt.Sprintf(`global:
  namespace: "%s"
extraEnvVars:
  EXPOSE_IRQ_COUNTER_METRICS: "false"
  EXPOSE_KUBELET_METRICS: "false"
  ENABLE_PROCESS_METRICS: "false"
`, config.Get("MO_OWN_NAMESPACE")),
	}
	return helm.HelmReleaseUpgrade(r)
}

func InstallClusterIssuer(email string, currentRetries int) (string, error) {
	_, err := kubernetes.DetermineIngressControllerType()
	if err != nil {
		return "", fmt.Errorf("Please install a IngressController before installing the ClusterIssuer.")
	}
	isCertManagerInstalled, err := kubernetes.IsCertManagerInstalled()
	if err != nil || !isCertManagerInstalled {
		return "", fmt.Errorf("Please install the Cert-Manager before installing the ClusterIssuer.")
	}

	time.Sleep(3 * time.Second) // wait for cert-manager to be ready
	maxRetries := 20
	if currentRetries >= maxRetries {
		return "", fmt.Errorf("ClusterIssuer installation exceeded max retries (%d). <br>- Make sure you have Cert-Manager setup and running.<br>- Make sure you have a IngressController setup and running. Please retry the installation in a few moments.", maxRetries)
	} else {
		ingType, err := kubernetes.DetermineIngressControllerType()
		if err != nil {
			serviceLogger.Error("InstallClusterIssuer: Error determining ingress controller type", "error", err)
		}
		if ingType == kubernetes.TRAEFIK || ingType == kubernetes.NGINX {
			r := ClusterHelmRequest{
				HelmRepoName:    "mogenius",
				HelmRepoUrl:     MogeniusHelmIndex,
				HelmReleaseName: utils.HelmReleaseNameClusterIssuer,
				HelmChartName:   "mogenius/mogenius-cluster-issuer",
				HelmValues: fmt.Sprintf(`global:
  clusterissuermail: "%s"
  ingressclass: "%s"
`, email, strings.ToLower(ingType.String())),
			}
			result, err := helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
			if err != nil {
				currentRetries++
				_, err := InstallClusterIssuer(email, currentRetries)
				if err != nil {
					serviceLogger.Debug("Error installing cluster issuer", "error", err)
				}
			}
			return result, err
		}
		serviceLogger.Info("No suitable Ingress Controller found. Retry in 3 seconds ...",
			"ingType", ingType.String(),
			"currentRetries", currentRetries,
			"maxRetries", maxRetries,
		)
		currentRetries++
		return InstallClusterIssuer(email, currentRetries)
	}
}

func UninstallMetricsServer() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNameMetricsServer,
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallIngressControllerTreafik() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNameTraefik,
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallCertManager() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNameCertManager,
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallMetalLb() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNameMetalLb,
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallKepler() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNameKepler,
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallClusterIssuer() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNameClusterIssuer,
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func InstallDefaultApplications() (string, string) {
	userApps := ""
	basicApps := `
# install grype
if type grype >/dev/null 2>&1; then
    echo "grype is installed. Skipping installation."
else
	wget -O /dev/stdout "https://raw.githubusercontent.com/anchore/grype/main/install.sh" | sh -s -- -b /usr/local/bin
	echo "grype is installed. 🚀"
fi

# install dive
if type dive >/dev/null 2>&1; then
    echo "dive is installed. Skipping installation."
else
	DIVE_VERSION=$(wget -O /dev/stdout "https://api.github.com/repos/wagoodman/dive/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
	if [ "${GOARCH}" = "amd64" ]; then
		wget -O dive.tar.gz "https://github.com/wagoodman/dive/releases/download/v${DIVE_VERSION}/dive_${DIVE_VERSION}_linux_amd64.tar.gz"
	elif [ "${GOARCH}" = "arm64" ]; then
		wget -O dive.tar.gz "https://github.com/wagoodman/dive/releases/download/v${DIVE_VERSION}/dive_${DIVE_VERSION}_linux_arm64.tar.gz"
	else
		echo "Unsupported architecture";
	fi
	tar -xf dive.tar.gz dive
	chmod +x dive
	mv dive /usr/local/bin/dive
	rm dive.tar.gz
	echo "dive is installed. 🚀"
fi

# install trivy
if type trivy >/dev/null 2>&1; then
    echo "trivy is installed. Skipping installation."
else
	wget -O /dev/stdout "https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh" | sh -s -- -b /usr/local/bin latest
	echo "trivy is installed. 🚀"
fi
`

	defaultAppsConfigmap := store.GetConfigMap(
		config.Get("MO_OWN_NAMESPACE"),
		utils.MOGENIUS_CONFIGMAP_DEFAULT_APPS_NAME,
	)
	if defaultAppsConfigmap == nil {
		return basicApps, userApps
	}
	assert.Assert(defaultAppsConfigmap != nil)
	assert.Assert(defaultAppsConfigmap.Data != nil)

	if installCommands, exists := defaultAppsConfigmap.Data["install-commands"]; exists {
		userApps = installCommands
	}

	return basicApps, userApps
}

func InstallAddressPool() string {
	return `apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: mo-pool
  namespace: mogenius
spec:
  addresses:
  - 192.168.66.1-192.168.66.50
  - fc00:f853:0ccd:e797::/124`
}

func getMostCurrentHelmChartVersion(url string, chartname string) string {
	url = addIndexYAMLtoURL(url)
	data, err := utils.GetVersionData(url)
	if err != nil {
		serviceLogger.Error("Error getting helm chart version",
			"chartUrl", url,
			"chartName", chartname,
			"error", err,
		)
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
