package services

import (
	"fmt"
	"io"
	"mogenius-k8s-manager/src/helm"
	"mogenius-k8s-manager/src/kubernetes"
	mokubernetes "mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/disk"

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

func UpgradeK8sManager(r K8sManagerUpgradeRequest) *structs.Job {
	var wg sync.WaitGroup

	job := structs.CreateJob("Upgrade mogenius platform", "UPGRADE", "", "")
	job.Start()
	mokubernetes.UpgradeMyself(job, r.Command, &wg)
	wg.Wait()
	job.Finish()
	return job
}

func InstallHelmChart(r ClusterHelmRequest) *structs.Job {
	job := structs.CreateJob("Install Helm Chart "+r.HelmReleaseName, r.NamespaceId, "", "")
	job.Start()
	result, err := helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
	if err != nil {
		job.Fail(fmt.Sprintf("Failed to install helm chart %s: %s\n%s", r.HelmReleaseName, result, err.Error()))
	}
	job.Finish()
	return job
}

func DeleteHelmChart(r ClusterHelmUninstallRequest) *structs.Job {
	job := structs.CreateJob("Delete Helm Chart "+r.HelmReleaseName, r.NamespaceId, "", "")
	job.Start()
	result, err := helm.DeleteHelmChart(r.HelmReleaseName, r.NamespaceId)
	if err != nil {
		job.Fail(fmt.Sprintf("Failed to delete helm chart %s: %s\n%s", r.HelmReleaseName, result, err.Error()))
	}
	job.Finish()
	return job
}

func CreateMogeniusNfsVolume(r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob("Create mogenius nfs-volume.", r.NamespaceName, "", "")
	job.Start()
	// FOR K8SMANAGER
	mokubernetes.CreateMogeniusNfsServiceSync(job, r.NamespaceName, r.VolumeName)
	mokubernetes.CreateMogeniusNfsPersistentVolumeClaim(job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg)
	mokubernetes.CreateMogeniusNfsDeployment(job, r.NamespaceName, r.VolumeName, &wg)
	// FOR SERVICES THAT WANT TO MOUNT
	mokubernetes.CreateMogeniusNfsPersistentVolumeForService(job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg)
	mokubernetes.CreateMogeniusNfsPersistentVolumeClaimForService(job, r.NamespaceName, r.VolumeName, r.SizeInGb, &wg)
	wg.Wait()
	job.Finish()

	nfsService := mokubernetes.ServiceForNfsVolume(r.NamespaceName, r.VolumeName)
	mokubernetes.Mount(r.NamespaceName, r.VolumeName, nfsService)

	return job.DefaultReponse()
}

func DeleteMogeniusNfsVolume(r NfsVolumeRequest) structs.DefaultResponse {
	var wg sync.WaitGroup
	job := structs.CreateJob("Delete mogenius nfs-volume.", r.NamespaceName, "", "")
	job.Start()
	// FOR K8SMANAGER
	mokubernetes.DeleteMogeniusNfsDeployment(job, r.NamespaceName, r.VolumeName, &wg)
	mokubernetes.DeleteMogeniusNfsService(job, r.NamespaceName, r.VolumeName, &wg)
	mokubernetes.DeleteMogeniusNfsPersistentVolumeClaim(job, r.NamespaceName, r.VolumeName, &wg)
	// FOR SERVICES THAT WANT TO MOUNT
	mokubernetes.DeleteMogeniusNfsPersistentVolumeForService(job, r.VolumeName, r.NamespaceName, &wg)
	mokubernetes.DeleteMogeniusNfsPersistentVolumeClaimForService(job, r.NamespaceName, r.VolumeName, &wg)
	wg.Wait()
	job.Finish()

	mokubernetes.Umount(r.NamespaceName, r.VolumeName)

	return job.DefaultReponse()
}

func StatsMogeniusNfsVolume(r NfsVolumeStatsRequest) NfsVolumeStatsResponse {
	mountPath := utils.MountPath(r.NamespaceName, r.VolumeName, "/", mokubernetes.RunsInCluster())
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

		mountPath := utils.MountPath(r.NamespaceName, pvc.Name, "/", mokubernetes.RunsInCluster())

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

type K8sManagerUpgradeRequest struct {
	Command string `json:"command" validate:"required"` // complete helm command from platform ui
}

type ClusterHelmRequest struct {
	Namespace       string `json:"namespace" validate:"required"`
	NamespaceId     string `json:"namespaceId" validate:"required"`
	HelmRepoName    string `json:"helmRepoName" validate:"required"`
	HelmRepoUrl     string `json:"helmRepoUrl" validate:"required"`
	HelmReleaseName string `json:"helmReleaseName" validate:"required"`
	HelmChartName   string `json:"helmChartName" validate:"required"`
	HelmValues      string `json:"helmValues" validate:"required"`
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
		HelmReleaseName: utils.HelmReleaseNameTrafficCollector,
		HelmChartName:   "mogenius/" + utils.HelmReleaseNameTrafficCollector,
		HelmValues: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, config.Get("MO_OWN_NAMESPACE"), config.Get("MO_STAGE")),
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeTrafficCollector() (string, error) {
	r := helm.HelmReleaseUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Release:   utils.HelmReleaseNameTrafficCollector,
		Chart:     "mogenius/" + utils.HelmReleaseNameTrafficCollector,
		Values: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, config.Get("MO_OWN_NAMESPACE"), config.Get("MO_STAGE")),
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
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradePodStatsCollector() (string, error) {
	r := helm.HelmReleaseUpgradeRequest{
		Namespace: config.Get("MO_OWN_NAMESPACE"),
		Release:   utils.HelmReleaseNamePodStatsCollector,
		Chart:     "mogenius/" + utils.HelmReleaseNamePodStatsCollector,
		Values: fmt.Sprintf(`global:
  namespace: %s
  stage: %s
`, config.Get("MO_OWN_NAMESPACE"), config.Get("MO_STAGE")),
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
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeMetricsServer() (string, error) {
	r := helm.HelmReleaseUpgradeRequest{
		Namespace: "default",
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
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeIngressControllerTreafik() (string, error) {
	r := helm.HelmReleaseUpgradeRequest{
		Namespace: "default",
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
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeCertManager() (string, error) {
	r := helm.HelmReleaseUpgradeRequest{
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
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func InstallExternalSecrets() (string, error) {
	r := ClusterHelmRequest{
		HelmRepoName:    utils.HelmReleaseNameExternalSecrets,
		HelmRepoUrl:     ExternalSecretsHelmIndex,
		HelmReleaseName: utils.HelmReleaseNameExternalSecrets,
		HelmChartName:   "external-secrets/external-secrets",
		HelmValues:      "",
	}
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeContainerRegistry() (string, error) {
	r := helm.HelmReleaseUpgradeRequest{
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
	helmResultStr, err := helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
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
	r := helm.HelmReleaseUpgradeRequest{
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
	return helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
}

func UpgradeKepler() (string, error) {
	r := helm.HelmReleaseUpgradeRequest{
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
	time.Sleep(3 * time.Second) // wait for cert-manager to be ready
	maxRetries := 10
	if currentRetries >= maxRetries {
		return "", fmt.Errorf("No suitable Ingress Controller found. Please install Traefik or Nginx Ingress Controller first.")
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
			result, err := helm.CreateHelmChart(r.HelmReleaseName, r.HelmRepoName, r.HelmRepoUrl, r.HelmChartName, r.HelmValues)
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
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallPodStatsCollector() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       config.Get("MO_OWN_NAMESPACE"),
		HelmReleaseName: utils.HelmReleaseNamePodStatsCollector,
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallMetricsServer() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       "default",
		HelmReleaseName: utils.HelmReleaseNameMetricsServer,
	}
	return helm.DeleteHelmChart(r.HelmReleaseName, r.Namespace)
}

func UninstallIngressControllerTreafik() (string, error) {
	r := ClusterHelmRequest{
		Namespace:       "default",
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
# install kubectl
if command -v kubectl >/dev/null 2>&1; then
    echo "kubectl is installed. Skipping installation."
else
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/${GOARCH}/kubectl"
chmod +x kubectl
mv kubectl /usr/local/bin/kubectl
	echo "kubectl is installed. 🚀"
fi

# install popeye
if command -v popeye >/dev/null 2>&1; then
    echo "popeye is installed. Skipping installation."
else
	if [ "${GOARCH}" = "amd64" ]; then
		curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_x86_64.tar.gz;
	elif [ "${GOARCH}" = "arm64" ]; then
		curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_arm64.tar.gz;
	elif [ "${GOARCH}" = "arm" ]; then
		curl -fsSL -o popeye.tar.gz https://github.com/derailed/popeye/releases/download/v0.11.1/popeye_Linux_arm.tar.gz;
	else
		echo "Unsupported architecture";
	fi
	tar -xf popeye.tar.gz popeye
	chmod +x popeye
	mv popeye /usr/local/bin/popeye
	rm popeye.tar.gz
	echo "popeye is installed. 🚀"
fi

# install grype
if type grype >/dev/null 2>&1; then
    echo "grype is installed. Skipping installation."
else
	curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin
	echo "grype is installed. 🚀"
fi

# install dive
if type dive >/dev/null 2>&1; then
    echo "dive is installed. Skipping installation."
else
	DIVE_VERSION=$(curl -sL "https://api.github.com/repos/wagoodman/dive/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
	if [ "${GOARCH}" = "amd64" ]; then
		curl -o dive.tar.gz -L https://github.com/wagoodman/dive/releases/download/v${DIVE_VERSION}/dive_${DIVE_VERSION}_linux_amd64.tar.gz
	elif [ "${GOARCH}" = "arm64" ]; then
		curl -o dive.tar.gz -L https://github.com/wagoodman/dive/releases/download/v${DIVE_VERSION}/dive_${DIVE_VERSION}_linux_arm64.tar.gz
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
	curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin latest
	echo "trivy is installed. 🚀"
fi

# install k9s
if type k9s >/dev/null 2>&1; then
    echo "k9s is installed. Skipping installation."
else
	K9S_VERSION=$(curl -sL "https://api.github.com/repos/derailed/k9s/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')
	if [ "${GOARCH}" = "amd64" ]; then
		curl -o k9s.tar.gz -L https://github.com/derailed/k9s/releases/download/v${K9S_VERSION}/k9s_Linux_amd64.tar.gz
	elif [ "${GOARCH}" = "arm64" ]; then
		curl -o k9s.tar.gz -L https://github.com/derailed/k9s/releases/download/v${K9S_VERSION}/k9s_Linux_arm64.tar.gz
	elif [ "${GOARCH}" = "arm" ]; then
		curl -o k9s.tar.gz -L https://github.com/derailed/k9s/releases/download/v${K9S_VERSION}/k9s_Linux_armv7.tar.gz
	else
		echo "Unsupported architecture";
	fi
	tar -xf k9s.tar.gz k9s
	chmod +x k9s
	mv k9s /usr/local/bin/k9s
	rm k9s.tar.gz
	echo "k9s is installed. 🚀"
fi

# create kubeconfig
cat <<EOF > kubeconfig.tmpl
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: @@CACRT@@
    server: https://@@IP@@:6443
  name: local
contexts:
- context:
    cluster: local
    user: mogenius
    namespace: mogenius
  name: mogenius
current-context: mogenius
kind: Config
preferences: {}
users:
- name: mogenius
  user:
    token: @@TOKEN@@
EOF
cat kubeconfig.tmpl | sed -e s/@@CACRT@@/$(echo -n "$(cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt)" | base64 | tr -d '\n')/| sed -e s/@@TOKEN@@/$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)/ | sed -e s/@@IP@@/$(kubectl get nodes -o json | jq '.items[0].status.addresses[0].address' | sed -e s/\"//g)/ > kubeconfig.yaml
echo "KUBECONFIG created. 🚀"
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

func getCurrentTrafficCollectorVersion() (string, error) {
	data, err := utils.GetVersionData(utils.HELM_INDEX)
	if err != nil {
		return "NO_VERSION_FOUND", err
	}
	trafficCollector := data.Entries[utils.HelmReleaseNameTrafficCollector]
	trafficResult := "NO_VERSION_FOUND"
	if len(trafficCollector) > 0 {
		trafficResult = trafficCollector[0].Version
	}
	return trafficResult, nil
}

func getCurrentPodStatsCollectorVersion() (string, error) {
	data, err := utils.GetVersionData(utils.HELM_INDEX)
	if err != nil {
		return "NO_VERSION_FOUND", err
	}
	podstatsCollector := data.Entries[utils.HelmReleaseNamePodStatsCollector]
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
