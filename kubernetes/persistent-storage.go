package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This functions are used to generate the mogenius custom nfs storage solution
// The order is importent when creating:
// 1. PVC
// 2. PV
// 3. DEPLOYMENT
// 4. SERVICE

func CreateMogeniusNfsPersistentVolumeClaim(job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Create PersistentVolumeClaim '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating PersistentVolumeClaim '%s'.", volumeName))

		storageClass := utils.StorageClassForClusterProvider(utils.CONFIG.Misc.ClusterProvider)

		pvc := utils.InitMogeniusNfsPersistentVolumeClaim()
		pvc.Name = volumeName
		pvc.Namespace = namespaceName
		pvc.Spec.StorageClassName = utils.Pointer(storageClass)
		pvc.Spec.Resources.Requests = v1.ResourceList{}
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

		kubeProvider := NewKubeProvider()
		pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		_, err := pvcClient.Create(context.TODO(), &pvc, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created PersistentVolumeClaim '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func DeleteMogeniusNfsPersistentVolumeClaim(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Delete PersistentVolumeClaim '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting PersistentVolumeClaim '%s'.", volumeName))

		kubeProvider := NewKubeProvider()
		pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		err := pvcClient.Delete(context.TODO(), volumeName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted PersistentVolumeClaim '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

// func CreateMogeniusNfsPersistentVolume(job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int, wg *sync.WaitGroup) *structs.Command {
// 	cmd := structs.CreateCommand(fmt.Sprintf("Create PersistentVolume '%s'.", volumeName), job)
// 	wg.Add(1)
// 	go func(cmd *structs.Command, wg *sync.WaitGroup) {
// 		defer wg.Done()
// 		cmd.Start(fmt.Sprintf("Creating PersistentVolume '%s'.", volumeName))

// 		pvc := utils.NfsPersistentVolumeClaimMogenius()
// 		pvc.Name = volumeName
// 		pvc.Namespace = namespaceName
// 		pvc.Spec.Resources.Requests = v1.ResourceList{}
// 		pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

// 		kubeProvider := NewKubeProvider()
// 		pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
// 		_, err := pvcClient.Create(context.TODO(), &pvc, metav1.CreateOptions{})
// 		if err != nil {
// 			cmd.Fail(fmt.Sprintf("CreateMogeniusNfsPersistentVolume ERROR: %s", err.Error()))
// 		} else {
// 			cmd.Success(fmt.Sprintf("Created PersistentVolume '%s'.", volumeName))
// 		}
// 	}(cmd, wg)
// 	return cmd
// }

func DeleteMogeniusNfsPersistentVolume(job *structs.Job, volumeName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Delete PersistentVolume '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting PersistentVolume '%s'.", volumeName))

		kubeProvider := NewKubeProvider()
		pvcClient := kubeProvider.ClientSet.CoreV1().PersistentVolumes()
		err := pvcClient.Delete(context.TODO(), volumeName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsPersistentVolume ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted PersistentVolume '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func CreateMogeniusNfsService(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Create PersistentVolume Service '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating PersistentVolume Service '%s'.", volumeName))

		service := utils.InitMogeniusNfsService()
		service.Name = volumeName
		service.Namespace = namespaceName
		service.Spec.Selector["app"] = volumeName

		kubeProvider := NewKubeProvider()
		serviceClient := kubeProvider.ClientSet.CoreV1().Services(namespaceName)
		_, err := serviceClient.Create(context.TODO(), &service, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateMogeniusNfsService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created PersistentVolume Service '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func DeleteMogeniusNfsService(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Delete PersistentVolume Service '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting PersistentVolume Service '%s'.", volumeName))

		kubeProvider := NewKubeProvider()
		pvcClient := kubeProvider.ClientSet.CoreV1().Services(namespaceName)
		err := pvcClient.Delete(context.TODO(), volumeName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted PersistentVolume Service '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func CreateMogeniusNfsDeployment(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Create PersistentVolume Deployment '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating PersistentVolume Deployment '%s'.", volumeName))

		deployment := utils.InitMogeniusNfsDeployment()
		deployment.Name = volumeName
		deployment.Namespace = namespaceName
		deployment.Spec.Template.Labels = make(map[string]string)
		deployment.Spec.Template.Labels["app"] = volumeName
		deployment.Spec.Selector.MatchLabels = make(map[string]string)
		deployment.Spec.Selector.MatchLabels["app"] = volumeName
		deployment.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = volumeName

		kubeProvider := NewKubeProvider()
		deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(namespaceName)
		_, err := deploymentClient.Create(context.TODO(), &deployment, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateMogeniusNfsDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created PersistentVolume Deployment '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func DeleteMogeniusNfsDeployment(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Delete PersistentVolume Deployment '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting PersistentVolume Deployment '%s'.", volumeName))

		kubeProvider := NewKubeProvider()
		deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(namespaceName)
		err := deploymentClient.Delete(context.TODO(), volumeName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted PersistentVolume Deployment '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}
