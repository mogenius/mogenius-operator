package kubernetes

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
