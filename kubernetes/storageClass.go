package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateMogeniusNfsStorageClass(job *structs.Job, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Create StorageClass for mogenius-nfs-storage.", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start("Creating StorageClass for mogenius-nfs-storage.", c)

		kubeProvider := NewKubeProvider()
		storageClient := kubeProvider.ClientSet.StorageV1().StorageClasses()
		storageClass := utils.InitNfsStorageClassMogenius()

		_, err := storageClient.Create(context.TODO(), &storageClass, MoCreateOptions())
		if err != nil {
			cmd.Fail(fmt.Sprintf("Create StorageClass ERROR: %s", err.Error()), c)
		} else {
			cmd.Success("Created StorageClass for mogenius-nfs-storage.", c)
		}

	}(cmd, wg)
	return cmd
}

func DeleteMogeniusNfsStorageClass(job *structs.Job, c *websocket.Conn, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand("Delete StorageClass for mogenius-nfs-storage.", job, c)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start("Deleting StorageClass for mogenius-nfs-storage.", c)

		kubeProvider := NewKubeProvider()
		storageClient := kubeProvider.ClientSet.StorageV1().StorageClasses()

		err := storageClient.Delete(context.TODO(), "openebs-rwx", metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsStorageClass ERROR: %s", err.Error()), c)
		} else {
			cmd.Success("Deleted StorageClass for mogenius-nfs-storage.", c)
		}
	}(cmd, wg)
	return cmd
}
