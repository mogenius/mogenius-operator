package kubernetes

import (
	"context"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePersistentVolume(stage dtos.K8sStageDto) error {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("CreatePersistentVolume ERROR: %s", err.Error())
	}

	pv := utils.InitPersistentVolume()
	if err != nil {
		logger.Log.Error(err)
	}
	pv.ObjectMeta.Name = stage.K8sName
	pv.ObjectMeta.Labels["type"] = stage.K8sName
	pv.Spec.NFS.Server = utils.CONFIG.Misc.StorageAccount + ".file.core.windows.net"
	// TODO: LÖSUNG FINDEN
	//pv.Spec.NFS.Path = "/" + utils.CONFIG.Misc.StorageAccount + "/" + utils.CONFIG.Kubernetes.AzClusterName + "/" + stage.Id
	pv.Spec.StorageClassName = ""
	pv.Spec.Capacity.Storage().Set(int64(stage.StorageSizeInMb / 1024))

	if err != nil {
		logger.Log.Errorf("CreatePersistentVolume ERROR: %s", err.Error())
	}

	pvClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()

	logger.Log.Infof("Creating PersistentVolume '%s'.", stage.K8sName)

	_, err = pvClient.Create(context.TODO(), &pv, metav1.CreateOptions{})
	if err != nil {
		logger.Log.Errorf("CreatePersistentVolume ERROR: %s", err.Error())
	} else {
		logger.Log.Infof("Creating PersistentVolume '%s'.", stage.K8sName)
	}

	return err
}

func CreatePersistentVolumeClaim(stage dtos.K8sStageDto) error {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("CreatePersistentVolumeClaim ERROR: %s", err.Error())
	}

	pvc := utils.InitPersistentVolumeClaim()
	if err != nil {
		logger.Log.Error(err)
	}
	pvc.ObjectMeta.Name = stage.K8sName
	pvc.ObjectMeta.Labels["type"] = stage.K8sName
	pvc.Spec.Selector.MatchLabels["type"] = stage.K8sName
	var storageClassName = ""
	pvc.Spec.StorageClassName = &storageClassName
	pvc.Spec.Resources.Requests.Storage().Set(int64(stage.StorageSizeInMb / 1024))
	pvc.Spec.Resources.Limits.Storage().Set(int64(stage.StorageSizeInMb / 1024))

	if err != nil {
		logger.Log.Errorf("CreatePersistentVolumeClaim ERROR: %s", err.Error())
	}

	pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(stage.K8sName)

	logger.Log.Infof("Creating PersistentVolumeClaim '%s'.", stage.K8sName)

	_, err = pvcClient.Create(context.TODO(), &pvc, metav1.CreateOptions{})
	if err != nil {
		logger.Log.Errorf("CreatePersistentVolumeClaim ERROR: %s", err.Error())
	} else {
		logger.Log.Infof("Created PersistentVolumeClaim '%s'.", stage.K8sName)
	}

	return err
}

func DeletePersistentVolume(stage dtos.K8sStageDto) error {
	var kubeProvider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderLocal()
	} else {
		kubeProvider, err = NewKubeProviderInCluster()
	}

	if err != nil {
		logger.Log.Errorf("DeletePersistentVolume ERROR: %s", err.Error())
	}

	pvClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()

	logger.Log.Infof("Deleting PersistentVolume '%s'.", stage.K8sName)

	err = pvClient.Delete(context.TODO(), stage.K8sName, metav1.DeleteOptions{})
	if err != nil {
		logger.Log.Errorf("DeletePersistentVolume ERROR: %s", err.Error())
	} else {
		logger.Log.Infof("Deleted PersistentVolume '%s'.", stage.K8sName)
	}

	return err
}
