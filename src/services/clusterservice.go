package services

import (
	"fmt"
	"io"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/websocket"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	NameMetricsServer             = "Metrics Server"
	NameMetalLB                   = "MetalLB (LoadBalancer)"
	NameIngressController         = "Ingress Controller"
	NameLocalDevSetup             = "Local Dev Setup"
	NameInternalContainerRegistry = "Internal Container Registry"
	NameExternalSecrets           = "External Secrets"
	NameClusterIssuerResource     = "letsencrypt-cluster-issuer"
	NameKepler                    = "Kepler"
	NameNfsStorageClass           = "nfs-storageclass"
)

const (
	MetricsHelmIndex                  = "https://kubernetes-sigs.github.io/metrics-server"
	IngressControllerTraefikHelmIndex = "https://traefik.github.io/charts"
	ContainerRegistryHelmIndex        = "https://phntom.kix.co.il/charts"
	ExternalSecretsHelmIndex          = "https://charts.external-secrets.io"
	CertManagerHelmIndex              = "https://charts.jetstack.io"
	KeplerHelmIndex                   = "https://sustainable-computing-io.github.io/kepler-helm-chart"
	MetalLBHelmIndex                  = "https://metallb.github.io/metallb"
	MogeniusHelmIndex                 = "https://helm.mogenius.com/public"
)

func InstallHelmChart(eventClient websocket.WebsocketClient, r ClusterHelmRequest) *structs.Job {
	job := structs.CreateJob(eventClient, "Install Helm Chart "+r.HelmReleaseName, r.NamespaceId, "", "")
	job.Start(eventClient)
	result, err := helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
	if err != nil {
		job.Fail(fmt.Sprintf("Failed to install helm chart %s: %s\n%s", r.HelmReleaseName, result, err.Error()))
	}
	job.Finish(eventClient)
	return job
}

func DeleteHelmChart(eventClient websocket.WebsocketClient, r ClusterHelmUninstallRequest) *structs.Job {
	job := structs.CreateJob(eventClient, "Delete Helm Chart "+r.HelmReleaseName, r.NamespaceId, "", "")
	job.Start(eventClient)
	result, err := helm.DeleteHelmChart(r.HelmReleaseName, r.NamespaceId)
	if err != nil {
		job.Fail(fmt.Sprintf("Failed to delete helm chart %s: %s\n%s", r.HelmReleaseName, result, err.Error()))
	}
	job.Finish(eventClient)
	return job
}

func CreateMogeniusNfsVolume(eventClient websocket.WebsocketClient, r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob(eventClient, "Create mogenius nfs-volume.", r.NamespaceName, "", "")
	job.Start(eventClient)
	// FOR K8SMANAGER
	mokubernetes.CreateMogeniusNfsServiceSync(eventClient, job, r.NamespaceName, r.VolumeName)
	mokubernetes.CreateMogeniusNfsPersistentVolumeClaim(eventClient, job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg)
	mokubernetes.CreateMogeniusNfsDeployment(eventClient, job, r.NamespaceName, r.VolumeName, &wg)
	// FOR SERVICES THAT WANT TO MOUNT
	mokubernetes.CreateMogeniusNfsPersistentVolumeForService(eventClient, job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg)
	mokubernetes.CreateMogeniusNfsPersistentVolumeClaimForService(eventClient, job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg)
	wg.Wait()
	job.Finish(eventClient)

	nfsService := mokubernetes.ServiceForNfsVolume(r.NamespaceName, r.VolumeName)
	mokubernetes.Mount(r.NamespaceName, r.VolumeName, nfsService)

	return job.DefaultReponse()
}

func DeleteMogeniusNfsVolume(eventClient websocket.WebsocketClient, r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob(eventClient, "Delete mogenius nfs-volume.", r.NamespaceName, "", "")
	job.Start(eventClient)
	// FOR K8SMANAGER
	mokubernetes.DeleteMogeniusNfsDeployment(eventClient, job, r.NamespaceName, r.VolumeName, &wg)
	mokubernetes.DeleteMogeniusNfsService(eventClient, job, r.NamespaceName, r.VolumeName, &wg)
	mokubernetes.DeleteMogeniusNfsPersistentVolumeClaim(eventClient, job, r.NamespaceName, r.VolumeName, &wg)
	// FOR SERVICES THAT WANT TO MOUNT
	mokubernetes.DeleteMogeniusNfsPersistentVolumeForService(eventClient, job, r.VolumeName, r.NamespaceName, &wg)
	mokubernetes.DeleteMogeniusNfsPersistentVolumeClaimForService(eventClient, job, r.NamespaceName, r.VolumeName, &wg)
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

func StatsMogeniusNfsNamespace(r NfsNamespaceStatsRequest) []NfsVolumeStatsResponse {
	result := []NfsVolumeStatsResponse{}

	if r.NamespaceName == "null" || r.NamespaceName == "" {
		serviceLogger.Error("StatsMogeniusNfsNamespace", "error", "namespaceName cannot be null or empty")
		return result
	}

	// get all pvc for single namespace
	pvcs := kubernetes.AllPersistentVolumeClaims(r.NamespaceName)

	for _, pvc := range pvcs {
		// skip pvcs which are not mogenius-nfs
		if !strings.HasPrefix(pvc.Name, fmt.Sprintf("%s-", utils.NFS_POD_PREFIX)) {
			continue
		}
		// remove podname "nfs-server-pod-"
		pvc.Name = strings.Replace(pvc.Name, fmt.Sprintf("%s-", utils.NFS_POD_PREFIX), "", 1)

		entry := NfsVolumeStatsResponse{
			VolumeName: pvc.Name,
			FreeBytes:  0,
			UsedBytes:  0,
			TotalBytes: 0,
		}

		mountPath := utils.MountPath(r.NamespaceName, pvc.Name, "/", clientProvider.RunsInCluster())

		if utils.ClusterProviderCached == utils.DOCKER_DESKTOP || utils.ClusterProviderCached == utils.K3S {
			var usedBytes uint64 = sumAllBytesOfFolder(mountPath)
			entry.FreeBytes = uint64(pvc.Spec.Resources.Requests.Storage().Value()) - usedBytes
			entry.UsedBytes = usedBytes
			entry.TotalBytes = uint64(pvc.Spec.Resources.Requests.Storage().Value())
		} else {
			free, used, total, err := diskUsage(mountPath)
			if err != nil {
				continue
			} else {
				entry.FreeBytes = free
				entry.UsedBytes = used
				entry.TotalBytes = total
			}
		}

		message := fmt.Sprintf("💾: '%s' -> %s / %s (Free: %s)", mountPath, utils.BytesToHumanReadable(int64(entry.UsedBytes)), utils.BytesToHumanReadable(int64(entry.TotalBytes)), utils.BytesToHumanReadable(int64(entry.FreeBytes)))
		serviceLogger.Info(message)
		result = append(result, entry)
	}
	return result
}

func sumAllBytesOfFolder(root string) uint64 {
	var total uint64
	var wg sync.WaitGroup
	var sumWg sync.WaitGroup
	fileSizes := make(chan uint64)

	sumWg.Add(1)
	// Start a goroutine to sum file sizes.
	go func() {
		defer sumWg.Done() // Signal completion of summing
		for size := range fileSizes {
			total += size
		}
	}()

	// Walk the file tree concurrently.
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				info, err := d.Info()
				if err != nil {
					return // handle error
				}
				fileSizes <- uint64(info.Size())
			}()
		}
		return nil
	})
	if err != nil {
		serviceLogger.Error("Error while summing bytes in path", "error", err)
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

func ClusterHelmRequestExample() ClusterHelmRequest {
	return ClusterHelmRequest{
		Namespace:       "mogenius",
		NamespaceId:     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		HelmRepoUrl:     "https://charts.bitnami.com/bitnami",
		HelmReleaseName: "test-helm-release",
		HelmChartName:   "bitnami/nginx",
		HelmValues:      "",
	}
}

type ClusterHelmUninstallRequest struct {
	NamespaceId     string `json:"namespaceId" validate:"required"`
	HelmReleaseName string `json:"helmReleaseName" validate:"required"`
}

func ClusterHelmUninstallRequestExample() ClusterHelmUninstallRequest {
	return ClusterHelmUninstallRequest{
		NamespaceId:     "B0919ACB-92DD-416C-AF67-E59AD4B25265",
		HelmReleaseName: "test-helm-release",
	}
}

type ClusterWriteConfigMap struct {
	Namespace string            `json:"namespace" validate:"required"`
	Name      string            `json:"name" validate:"required"`
	Labels    map[string]string `json:"labels" validate:"required"`
	Data      string            `json:"data" validate:"required"`
}

func ClusterWriteConfigMapExample() ClusterWriteConfigMap {
	return ClusterWriteConfigMap{
		Namespace: "mogenius",
		Name:      "my-funky-configmap",
		Labels: map[string]string{
			"app": "my-funky-app",
		},
		Data: "my-funky-data-yaml-string",
	}
}

type ClusterListWorkloads struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Prefix        string `json:"prefix"`
}

func ClusterListWorkloadsExample() ClusterListWorkloads {
	return ClusterListWorkloads{
		Namespace:     "mogenius",
		LabelSelector: "",
		Prefix:        "metal",
	}
}

type ClusterUpdateLocalTlsSecret struct {
	LocalTlsCrt string `json:"localTlsCrt" validate:"required"`
	LocalTlsKey string `json:"localTlsKey" validate:"required"`
}

func ClusterUpdateLocalTlsSecretExample() ClusterUpdateLocalTlsSecret {
	return ClusterUpdateLocalTlsSecret{
		LocalTlsCrt: "my-funky-crt",
		LocalTlsKey: "my-funky-key",
	}
}

type ClusterGetConfigMap struct {
	Namespace string `json:"namespace" validate:"required"`
	Name      string `json:"name" validate:"required"`
}

func ClusterGetConfigMapExample() ClusterGetConfigMap {
	return ClusterGetConfigMap{
		Namespace: "mogenius",
		Name:      "my-funky-configmap",
	}
}

type ClusterGetDeployment struct {
	Namespace string `json:"namespace" validate:"required"`
	Name      string `json:"name" validate:"required"`
}

func ClusterGetDeploymentExample() ClusterGetDeployment {
	return ClusterGetDeployment{
		Namespace: "mogenius",
		Name:      "my-funky-deployment",
	}
}

type ClusterGetPersistentVolume struct {
	Namespace string `json:"namespace" validate:"required"`
	Name      string `json:"name" validate:"required"`
}

func ClusterGetPersistentVolumeExample() ClusterGetPersistentVolume {
	return ClusterGetPersistentVolume{
		Namespace: "mogenius",
		Name:      "nfs-server-my-funky-nfs",
	}
}

type NfsStorageInstallRequest struct {
	ClusterProvider utils.KubernetesProvider `json:"clusterProvider"`
}

func NfsStorageInstallRequestExample() NfsStorageInstallRequest {
	return NfsStorageInstallRequest{
		ClusterProvider: utils.AKS,
	}
}

type NfsVolumeRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
	VolumeName    string `json:"volumeName" validate:"required"`
	SizeInGb      int    `json:"sizeInGb" validate:"required"`
}

func NfsVolumeRequestExample() NfsVolumeRequest {
	return NfsVolumeRequest{
		NamespaceName: "name",
		VolumeName:    "my-fancy-volume-name",
		SizeInGb:      10,
	}
}

type NfsVolumeStatsRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
	VolumeName    string `json:"volumeName" validate:"required"`
}

func NfsVolumeStatsRequestExample() NfsVolumeStatsRequest {
	return NfsVolumeStatsRequest{
		NamespaceName: "name",
		VolumeName:    "my-fancy-volume-name",
	}
}

type NfsNamespaceStatsRequest struct {
	NamespaceName string `json:"namespaceName" validate:"required"`
}

func NfsNamespaceStatsRequestExample() NfsNamespaceStatsRequest {
	return NfsNamespaceStatsRequest{
		NamespaceName: "test-bene-prod-a7fm72",
	}
}

type NfsVolumeStatsResponse struct {
	VolumeName string `json:"volumeName"`
	TotalBytes uint64 `json:"totalBytes"`
	FreeBytes  uint64 `json:"freeBytes"`
	UsedBytes  uint64 `json:"usedBytes"`
}

// @TODO: add request/respionse example for nfs status
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

var keplerHostAndPort string = ""

var energyConsumptionCollectionInProgress bool = false

func EnergyConsumption() []structs.EnergyConsumptionResponse {
	if energyConsumptionCollectionInProgress {
		return structs.CurrentEnergyConsumptionResponse
	}

	if keplerHostAndPort == "" {
		keplerservice := mokubernetes.ServiceWithLabels("app.kubernetes.io/component=exporter,app.kubernetes.io/name=kepler")
		if keplerservice != nil {
			keplerHostAndPort = fmt.Sprintf("%s:%d", keplerservice.Name, keplerservice.Spec.Ports[0].Port)
		} else {
			serviceLogger.Error("EnergyConsumption", "error", "kepler service not found.")
			return structs.CurrentEnergyConsumptionResponse
		}
		// if config.Get("MO_STAGE") == utils.STAGE_LOCAL {
		// 	ServiceLogger.Warning("OVERWRITTEN ACTUAL IP BECAUSE RUNNING IN LOCAL MODE! 192.168.178.132:9102")
		// 	keplerHostAndPort = "127.0.0.1:9102"
		// }
	}
	if structs.KeplerDaemonsetRunningSince == 0 {
		keplerPod := mokubernetes.KeplerPod()
		if keplerPod != nil && keplerPod.Status.StartTime != nil {
			structs.KeplerDaemonsetRunningSince = keplerPod.Status.StartTime.Time.Unix()
		}
	}

	go func() {
		energyConsumptionCollectionInProgress = true
		structs.CurrentEnergyConsumptionResponse = make([]structs.EnergyConsumptionResponse, structs.EnergyConsumptionResponseSize)
		for i := 0; i < structs.EnergyConsumptionResponseSize; i++ {
			// download the data
			response, err := http.Get(fmt.Sprintf("http://%s/metrics", keplerHostAndPort))
			if err != nil {
				serviceLogger.Error("EnergyConsumption", "error", err)
				return
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if err != nil {
				serviceLogger.Error("EnergyConsumptionRead", "error", err)
				return
			}

			// parse the data
			structs.CreateEnergyConsumptionResponse(string(data), i)
			time.Sleep(structs.EnergyConsumptionTimeInterval * time.Second)
		}
		energyConsumptionCollectionInProgress = false
	}()

	return structs.CurrentEnergyConsumptionResponse
}

func InstallTrafficCollector() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmRepoName:    "mogenius",
		HelmReleaseName: utils.HelmReleaseNameTrafficCollector,
		HelmChartName:   "mogenius/" + utils.HelmReleaseNameTrafficCollector,
		HelmValues: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, config.Get("MO_OWN_NAMESPACE"), config.Get("MO_STAGE")),
	}
	if config.Get("MO_STAGE") == utils.STAGE_DEV {
		r.HelmChartName = "mogenius/dev-" + utils.HelmReleaseNameTrafficCollector
		r.HelmReleaseName = "dev-" + utils.HelmReleaseNameTrafficCollector
		version, err := GetCurrentTrafficCollectorVersion()
		if err != nil {
			serviceLogger.Error("Error getting current traffic collector version", "error", err)
		}
		r.HelmChartVersion = version
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
}

func UpgradeTrafficCollector() (string, error) {
	r := helm.HelmChartInstallUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Release:   utils.HelmReleaseNameTrafficCollector,
		Chart:     "mogenius/" + utils.HelmReleaseNameTrafficCollector,
		Values: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, config.Get("MO_OWN_NAMESPACE"), config.Get("MO_STAGE")),
	}
	if config.Get("MO_STAGE") == utils.STAGE_DEV {
		r.Chart = "mogenius/dev-" + utils.HelmReleaseNameTrafficCollector
		r.Release = "dev-" + utils.HelmReleaseNameTrafficCollector
		version, err := GetCurrentTrafficCollectorVersion()
		if err != nil {
			serviceLogger.Error("Error getting current traffic collector version", "error", err)
		}
		r.Version = version
	}
	return helm.HelmReleaseUpgrade(r)
}

func InstallPodStatsCollector() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    "mogenius",
		HelmRepoUrl:     MogeniusHelmIndex,
		HelmReleaseName: utils.HelmReleaseNamePodStatsCollector,
		HelmChartName:   "mogenius/" + utils.HelmReleaseNamePodStatsCollector,
		HelmValues: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, config.Get("MO_OWN_NAMESPACE"), config.Get("MO_STAGE")),
	}
	if config.Get("MO_STAGE") == utils.STAGE_DEV {
		r.HelmChartName = "mogenius/dev-" + utils.HelmReleaseNamePodStatsCollector
		r.HelmReleaseName = "dev-" + utils.HelmReleaseNamePodStatsCollector
		version, err := GetCurrentPodStatsCollectorVersion()
		if err != nil {
			serviceLogger.Error("Error getting current pod stats collector version", "error", err)
		}
		r.HelmChartVersion = version
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
}

func UpgradePodStatsCollector() (string, error) {
	r := helm.HelmChartInstallUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Release:   utils.HelmReleaseNamePodStatsCollector,
		Chart:     "mogenius/" + utils.HelmReleaseNamePodStatsCollector,
		Values: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, config.Get("MO_OWN_NAMESPACE"), config.Get("MO_STAGE")),
	}
	if config.Get("MO_STAGE") == utils.STAGE_DEV {
		r.Chart = "mogenius/dev-" + utils.HelmReleaseNamePodStatsCollector
		r.Release = "dev-" + utils.HelmReleaseNamePodStatsCollector
		version, err := GetCurrentPodStatsCollectorVersion()
		if err != nil {
			serviceLogger.Error("Error getting current pod stats collector version", "error", err)
		}
		r.Version = version
	}
	return helm.HelmReleaseUpgrade(r)
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

func InstallContainerRegistry() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    "phntom",
		HelmRepoUrl:     ContainerRegistryHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameDistributionRegistry,
		HelmChartName:   "phntom/docker-registry",
		HelmValues:      "",
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
}

func InstallExternalSecrets() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameExternalSecrets,
		HelmRepoUrl:     ExternalSecretsHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameExternalSecrets,
		HelmChartName:   "external-secrets/external-secrets",
		HelmValues:      "",
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues, r.HelmChartVersion)
}

func UpgradeContainerRegistry() (string, error) {
	r := helm.HelmChartInstallUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Release:   utils.HelmReleaseNameDistributionRegistry,
		Chart:     "phntom/docker-registry",
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

func UninstallTrafficCollector() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNameTrafficCollector,
	}
	if config.Get("MO_STAGE") == utils.STAGE_DEV {
		r.HelmReleaseName = "dev-" + utils.HelmReleaseNameTrafficCollector
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallPodStatsCollector() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNamePodStatsCollector,
	}
	if config.Get("MO_STAGE") == utils.STAGE_DEV {
		r.HelmReleaseName = "dev-" + utils.HelmReleaseNamePodStatsCollector
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
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

func UninstallContainerRegistry() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNameDistributionRegistry,
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallExternalSecrets() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNameExternalSecrets,
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
	defaultAppsConfigmap := kubernetes.ConfigMapFor(config.Get("MO_OWN_NAMESPACE"), utils.MOGENIUS_CONFIGMAP_DEFAULT_APPS_NAME, false)
	if defaultAppsConfigmap != nil {
		if installCommands, exists := defaultAppsConfigmap.Data["install-commands"]; exists {
			userApps = installCommands
		}
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

func GetCurrentTrafficCollectorVersion() (string, error) {
	data, err := utils.GetVersionData(utils.HELM_INDEX)
	if err != nil {
		return "NO_VERSION_FOUND", err
	}

	chartName := utils.HelmReleaseNameTrafficCollector
	if config.Get("MO_STAGE") == utils.STAGE_DEV {
		chartName = "dev-" + utils.HelmReleaseNameTrafficCollector
	}
	trafficCollector := data.Entries[chartName]
	trafficResult := "NO_VERSION_FOUND"
	if len(trafficCollector) > 0 {
		trafficResult = trafficCollector[0].Version
	}
	return trafficResult, nil
}

func GetCurrentPodStatsCollectorVersion() (string, error) {
	data, err := utils.GetVersionData(utils.HELM_INDEX)
	if err != nil {
		return "NO_VERSION_FOUND", err
	}

	chartName := utils.HelmReleaseNamePodStatsCollector
	if config.Get("MO_STAGE") == utils.STAGE_DEV {
		chartName = "dev-" + utils.HelmReleaseNamePodStatsCollector
	}
	podstatsCollector := data.Entries[chartName]
	podstatsResult := "NO_VERSION_FOUND"
	if len(podstatsCollector) > 0 {
		podstatsResult = podstatsCollector[0].Version
	}
	return podstatsResult, nil
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
