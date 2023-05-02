package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// func CreatePersistentVolume(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
// 	cmd := structs.CreateCommand(fmt.Sprintf("Updating Deployment '%s'.", stage.K8sName), job, c)
// 	wg.Add(1)
// 	go func(cmd *structs.Command, wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(fmt.Sprintf("Updating Deployment '%s'.", stage.K8sName), c)

// 		kubeProvider := NewKubeProvider()
// 		pv := utils.InitPersistentVolume()
// 		pv.ObjectMeta.Name = stage.K8sName
// 		pv.ObjectMeta.Labels["type"] = stage.K8sName
// 		// XXX -> Azure Specific
// 		pv.Spec.NFS.Server = utils.CONFIG.Misc.StorageAccount + ".file.core.windows.net"
// 		pv.Spec.NFS.Path = "/" + utils.CONFIG.Misc.StorageAccount + "/" + utils.CONFIG.Kubernetes.ClusterName + "/" + stage.Id
// 		pv.Spec.StorageClassName = ""
// 		pv.Spec.Capacity.Storage().Set(int64(stage.StorageSizeInMb / 1024))

// 		pv.Labels = MoUpdateLabels(&pv.Labels, &job.NamespaceId, &stage, nil)

// 		pvClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()
// 		_, err := pvClient.Create(context.TODO(), &pv, MoCreateOptions())
// 		if err != nil {
// 			cmd.Fail(fmt.Sprintf("CreatePersistentVolume ERROR: %s", err.Error()), c)
// 		} else {
// 			cmd.Success(fmt.Sprintf("Creating PersistentVolume '%s'.", stage.K8sName), c)
// 		}
// 	}(cmd, wg)
// 	return cmd
// }

// func CreatePersistentVolumeClaim(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
// 	cmd := structs.CreateCommand(fmt.Sprintf("Updating Deployment '%s'.", stage.K8sName), job, c)
// 	wg.Add(1)
// 	go func(cmd *structs.Command, wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(fmt.Sprintf("Updating Deployment '%s'.", stage.K8sName), c)

// 		kubeProvider := NewKubeProvider()
// 		pvc := utils.InitPersistentVolumeClaim()
// 		pvc.ObjectMeta.Name = stage.K8sName
// 		pvc.ObjectMeta.Namespace = stage.K8sName
// 		pvc.Spec.StorageClassName = utils.Pointer("cephfs")
// 		pvc.Spec.Resources.Requests.Storage().Set(int64(stage.StorageSizeInMb / 1024))
// 		pvc.Spec.Resources.Limits.Storage().Set(int64(stage.StorageSizeInMb / 1024))

// 		pvc.Labels = MoUpdateLabels(&pvc.Labels, &job.NamespaceId, &stage, nil)

// 		pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(stage.K8sName)
// 		_, err := pvcClient.Create(context.TODO(), &pvc, MoCreateOptions())
// 		if err != nil {
// 			cmd.Fail(fmt.Sprintf("CreatePersistentVolumeClaim ERROR: %s", err.Error()), c)
// 		} else {
// 			cmd.Success(fmt.Sprintf("Creating PersistentVolumeClaim '%s'.", stage.K8sName), c)
// 		}
// 	}(cmd, wg)
// 	return cmd
// }

// func DeletePersistentVolume(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
// 	cmd := structs.CreateCommand(fmt.Sprintf("Deleting PersistentVolume '%s'.", stage.K8sName), job, c)
// 	wg.Add(1)
// 	go func(cmd *structs.Command, wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(fmt.Sprintf("Deleting PersistentVolume '%s'.", stage.K8sName), c)

// 		kubeProvider := NewKubeProvider()
// 		pvClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()
// 		err := pvClient.Delete(context.TODO(), stage.K8sName, metav1.DeleteOptions{})
// 		if err != nil {
// 			cmd.Fail(fmt.Sprintf("DeletePersistentVolume ERROR: %s", err.Error()), c)
// 		} else {
// 			cmd.Success(fmt.Sprintf("Deleted PersistentVolume '%s'.", stage.K8sName), c)
// 		}
// 	}(cmd, wg)
// 	return cmd
// }

func CreateMogeniusNfsPersistentVolumeClaim(job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Create PersistentVolume/Claim '%s'.", volumeName), job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating PersistentVolume/Claim '%s'.", volumeName), c)

		pvc := utils.NfsPersistentVolumeClaimMogenius()
		pvc.Name = volumeName
		pvc.Namespace = namespaceName
		pvc.Spec.Resources.Requests = v1.ResourceList{}
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

		kubeProvider := NewKubeProvider()
		pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		_, err := pvcClient.Create(context.TODO(), &pvc, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Created PersistentVolume/Claim '%s'.", volumeName), c)
		}
	}(cmd, wg)
	return cmd
}

func DeleteMogeniusNfsPersistentVolumeClaim(job *structs.Job, namespaceName string, volumeName string, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Delete PersistentVolume/Claim '%s'.", volumeName), job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting PersistentVolume/Claim '%s'.", volumeName), c)

		kubeProvider := NewKubeProvider()
		pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		err := pvcClient.Delete(context.TODO(), volumeName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Deleted PersistentVolume/Claim '%s'.", volumeName), c)
		}
	}(cmd, wg)
	return cmd
}
