package services

import (
	mokubernetes "mogenius-operator/src/kubernetes"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/websocket"
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


