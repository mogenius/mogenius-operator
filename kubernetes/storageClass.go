package kubernetes

import (
	"context"
	"mogenius-k8s-manager/logger"
	"os/exec"

	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// func CreateMogeniusNfsStorageClass(job *structs.Job, wg *sync.WaitGroup) *structs.Command {
// 	cmd := structs.CreateCommand("Create StorageClass for mogenius-nfs-storage.", job)
// 	wg.Add(1)
// 	go func(cmd *structs.Command, wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start("Creating StorageClass for mogenius-nfs-storage.")

// 		kubeProvider := NewKubeProvider()
// 		storageClient := kubeProvider.ClientSet.StorageV1().StorageClasses()
// 		storageClass := utils.InitNfsStorageClassMogenius()

// 		_, err := storageClient.Create(context.TODO(), &storageClass, MoCreateOptions())
// 		if err != nil {
// 			cmd.Fail(fmt.Sprintf("Create StorageClass ERROR: %s", err.Error()))
// 		} else {
// 			cmd.Success("Created StorageClass for mogenius-nfs-storage.")
// 		}

// 	}(cmd, wg)
// 	return cmd
// }

// func DeleteMogeniusNfsStorageClass(job *structs.Job, wg *sync.WaitGroup) *structs.Command {
// 	cmd := structs.CreateCommand("Delete StorageClass for mogenius-nfs-storage.", job)
// 	wg.Add(1)
// 	go func(cmd *structs.Command, wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start("Deleting StorageClass for mogenius-nfs-storage.")

// 		kubeProvider := NewKubeProvider()
// 		storageClient := kubeProvider.ClientSet.StorageV1().StorageClasses()

// 		err := storageClient.Delete(context.TODO(), "openebs-kernel-nfs", metav1.DeleteOptions{})
// 		if err != nil {
// 			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsStorageClass ERROR: %s", err.Error()))
// 		} else {
// 			cmd.Success("Deleted StorageClass for mogenius-nfs-storage.")
// 		}
// 	}(cmd, wg)
// 	return cmd
// }

func AllStorageClasses() K8sWorkloadResult {
	result := []storage.StorageClass{}

	provider := NewKubeProvider()
	scList, err := provider.ClientSet.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("AllStorageClasses ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, pv := range scList.Items {
		result = append(result, pv)
	}
	return WorkloadResult(result, nil)
}

func UpdateK8sStorageClass(data storage.StorageClass) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	scClient := kubeProvider.ClientSet.StorageV1().StorageClasses()
	_, err := scClient.Update(context.TODO(), &data, metav1.UpdateOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DeleteK8sStorageClass(data storage.StorageClass) K8sWorkloadResult {
	kubeProvider := NewKubeProvider()
	scClient := kubeProvider.ClientSet.StorageV1().StorageClasses()
	err := scClient.Delete(context.TODO(), data.Name, metav1.DeleteOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(nil, nil)
}

func DescribeK8sStorageClass(name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "storageclass", name)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(string(output), nil)
}
