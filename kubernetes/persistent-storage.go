package kubernetes

func CreatePersistentVolume() error {
	// jsonData.metadata.name = stage.k8sName;
	// jsonData.metadata.labels.type = stage.k8sName;
	// jsonData.spec.nfs.server = `${k8sManagerConfig.STORAGE_ACCOUNT}.file.core.windows.net`
	// jsonData.spec.nfs.path = `/${k8sManagerConfig.STORAGE_ACCOUNT}/${k8sManagerConfig.AZ_CLUSTER_NAME}/${stage.id}`;
	// jsonData.spec.storageClassName = '';
	// jsonData.spec.capacity.storage = `${stage.storageSizeInMb / 1024 ?? 1}Gi`;

	return nil
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
