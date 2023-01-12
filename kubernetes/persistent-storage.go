package kubernetes

import (
	"context"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePersistentVolume(stage dtos.K8sStageDto) error {
	// jsonData.metadata.name = stage.k8sName;
	// jsonData.metadata.labels.type = stage.k8sName;
	// jsonData.spec.nfs.server = `${k8sManagerConfig.STORAGE_ACCOUNT}.file.core.windows.net`
	// jsonData.spec.nfs.path = `/${k8sManagerConfig.STORAGE_ACCOUNT}/${k8sManagerConfig.AZ_CLUSTER_NAME}/${stage.id}`;
	// jsonData.spec.storageClassName = '';
	// jsonData.spec.capacity.storage = `${stage.storageSizeInMb / 1024 ?? 1}Gi`;

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
	structs.PrettyPrint(pv)

	if err != nil {
		logger.Log.Errorf("DeletePersistentVolume ERROR: %s", err.Error())
	}

	pvClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()

	logger.Log.Infof("Deleting PersistentVolume '%s'.", stage.K8sName)

	_, err = pvClient.Create(context.TODO(), &pv, metav1.CreateOptions{})
	if err != nil {
		logger.Log.Errorf("DeletePersistentVolume ERROR: %s", err.Error())
	} else {
		logger.Log.Infof("Deleted PersistentVolume '%s'.", stage.K8sName)
	}

	return err
}

func CreatePersistentVolumeClaim() error {
	// jsonData.metadata.name = stage.k8sName;
	// jsonData.metadata.namespace = stage.k8sName;
	// jsonData.spec.selector.matchLabels.type = stage.k8sName;
	// jsonData.spec.storageClassName = '';mo3-mysql.mysql.database.azure.com
	// jsonData.spec.resources.requests.storage = `${stage.storageSizeInMb / 1024 ?? 1}Gi`;
	// jsonData.spec.resources.limits.storage = `${stage.storageSizeInMb / 1024 ?? 1}Gi`;

	return nil
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
