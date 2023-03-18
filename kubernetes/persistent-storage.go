package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePersistentVolume(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Updating Deployment '%s'.", stage.K8sName), job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating Deployment '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreatePersistentVolume ERROR: %s", err.Error()), c)
		}

		pv := utils.InitPersistentVolume()
		pv.ObjectMeta.Name = stage.K8sName
		pv.ObjectMeta.Labels["type"] = stage.K8sName
		// XXX -> Azure Specific
		pv.Spec.NFS.Server = utils.CONFIG.Misc.StorageAccount + ".file.core.windows.net"
		pv.Spec.NFS.Path = "/" + utils.CONFIG.Misc.StorageAccount + "/" + utils.CONFIG.Kubernetes.ClusterName + "/" + stage.Id
		pv.Spec.StorageClassName = ""
		pv.Spec.Capacity.Storage().Set(int64(stage.StorageSizeInMb / 1024))

		pvClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()
		_, err = pvClient.Create(context.TODO(), &pv, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreatePersistentVolume ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Creating PersistentVolume '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func CreatePersistentVolumeClaim(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Updating Deployment '%s'.", stage.K8sName), job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Updating Deployment '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreatePersistentVolumeClaim ERROR: %s", err.Error()), c)
		}

		pvc := utils.InitPersistentVolumeClaim()
		pvc.ObjectMeta.Name = stage.K8sName
		pvc.ObjectMeta.Namespace = stage.K8sName
		pvc.Spec.StorageClassName = utils.Pointer("cephfs")
		pvc.Spec.Resources.Requests.Storage().Set(int64(stage.StorageSizeInMb / 1024))
		pvc.Spec.Resources.Limits.Storage().Set(int64(stage.StorageSizeInMb / 1024))

		pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(stage.K8sName)
		_, err = pvcClient.Create(context.TODO(), &pvc, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreatePersistentVolumeClaim ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Creating PersistentVolumeClaim '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd
}

func DeletePersistentVolume(job *structs.Job, stage dtos.K8sStageDto, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Deleting PersistentVolume '%s'.", stage.K8sName), job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting PersistentVolume '%s'.", stage.K8sName), c)

		var kubeProvider *KubeProvider
		var err error
		if !utils.CONFIG.Kubernetes.RunInCluster {
			kubeProvider, err = NewKubeProviderLocal()
		} else {
			kubeProvider, err = NewKubeProviderInCluster()
		}
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeletePersistentVolume ERROR: %s", err.Error()), c)
		}

		pvClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()
		err = pvClient.Delete(context.TODO(), stage.K8sName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeletePersistentVolume ERROR: %s", err.Error()), c)
		} else {
			cmd.Success(fmt.Sprintf("Deleted PersistentVolume '%s'.", stage.K8sName), c)
		}
	}(cmd, wg)
	return cmd

}
