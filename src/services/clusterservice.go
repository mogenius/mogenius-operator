package services

import (
	"mogenius-operator/src/assert"
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/store"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/websocket"
	"strings"
	"sync"
)

const (
	MogeniusHelmIndex = "https://helm.mogenius.com/public"
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

	return job.DefaultReponse()
}

func StatsMogeniusNfsVolume(r NfsVolumeStatsRequest) NfsVolumeStatsResponse {
	free, used, total, err := mokubernetes.NfsDiskUsage(r.NamespaceName, r.VolumeName)
	if err != nil {
		serviceLogger.Error("StatsMogeniusNfsVolume", "namespace", r.NamespaceName, "volume", r.VolumeName, "error", err)
	}
	result := NfsVolumeStatsResponse{
		VolumeName: r.VolumeName,
		FreeBytes:  free,
		UsedBytes:  used,
		TotalBytes: total,
	}

	serviceLogger.Info("💾: nfs volume stats",
		"namespace", r.NamespaceName,
		"volume", r.VolumeName,
		"usedBytes", utils.BytesToHumanReadable(int64(result.UsedBytes)),
		"totalBytes", utils.BytesToHumanReadable(int64(result.TotalBytes)),
		"freeBytes", utils.BytesToHumanReadable(int64(result.FreeBytes)),
	)
	return result
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

	return chartname + "-" + result
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
