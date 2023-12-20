package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strings"
	"sync"

	punq "github.com/mogenius/punq/kubernetes"
	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetVolumeMountsForK8sManager() ([]structs.Volume, error) {
	result := []structs.Volume{}

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return result, err
	}
	pvcClient := provider.ClientSet.CoreV1().PersistentVolumeClaims("")
	pvcList, err := pvcClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return result, err
	}
	for _, pvc := range pvcList.Items {
		if strings.HasPrefix(pvc.Name, utils.CONFIG.Misc.NfsPodPrefix) {
			capacity := pvc.Spec.Resources.Requests[v1.ResourceStorage]
			volName := strings.Replace(pvc.Name, fmt.Sprintf("%s-", utils.CONFIG.Misc.NfsPodPrefix), "", 1)
			result = append(result, structs.Volume{
				Namespace:  pvc.Namespace,
				VolumeName: volName,
				SizeInGb:   int(capacity.Value() / 1024 / 1024 / 1024),
			})
		}
	}
	return result, err
}

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

		storageClass := utils.StorageClassForClusterProvider(utils.ClusterProviderCached)

		pvc := utils.InitMogeniusNfsPersistentVolumeClaim()
		pvc.Name = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)
		pvc.Namespace = namespaceName
		pvc.Spec.StorageClassName = punqUtils.Pointer(storageClass)
		pvc.Spec.Resources.Requests = v1.ResourceList{}
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

		// add labels
		pvc.Labels = MoAddLabels(&pvc.Labels, map[string]string{
			"mo-nfs-volume-identifier": fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName),
			"mo-nfs-volume-name":       volumeName,
		})

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		_, err = pvcClient.Create(context.TODO(), &pvc, metav1.CreateOptions{})
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

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		err = pvcClient.Delete(context.TODO(), fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName), metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted PersistentVolumeClaim '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func CreateMogeniusNfsPersistentVolumeForService(job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Create PersistentVolume '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		k8sVolumeName := fmt.Sprintf("%s-%s", namespaceName, volumeName)
		cmd.Start(fmt.Sprintf("Creating PersistentVolume '%s'.", k8sVolumeName))

		nfsService := ServiceForNfsVolume(namespaceName, volumeName)
		if nfsService == nil {
			cmd.Fail(fmt.Sprintf("CreateMogeniusNfsPersistentVolume ERROR: Could not find service for volume '%s' in order to get IP-Address.", k8sVolumeName))
			return
		}

		pv := utils.InitMogeniusNfsPersistentVolumeForService()
		pv.Name = k8sVolumeName
		pv.Namespace = namespaceName
		pv.Spec.NFS.Server = nfsService.Spec.ClusterIP
		pv.Spec.Capacity = v1.ResourceList{}
		pv.Spec.Capacity[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

		// add labels
		pv.Labels = MoAddLabels(&pv.Labels, map[string]string{
			"mo-nfs-volume-identifier": fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName),
			"mo-nfs-volume-name":       volumeName,
		})

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		client := provider.ClientSet.CoreV1().PersistentVolumes()
		_, err = client.Create(context.TODO(), &pv, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateMogeniusNfsPersistentVolume ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created PersistentVolume '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func DeleteMogeniusNfsPersistentVolumeForService(job *structs.Job, volumeName string, namespaceName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Delete PersistentVolume '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		k8sVolumeName := fmt.Sprintf("%s-%s", namespaceName, volumeName)
		cmd.Start(fmt.Sprintf("Deleting DeleteMogeniusNfsPersistentVolumeForService '%s'.", k8sVolumeName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumes()

		// LIST ALL PV
		pvList, err := pvcClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// IN CASE: NOT FOUND -> IT HAS ALREADY BEEN DELETED. e.g. by the provisioneer
				cmd.Success(fmt.Sprintf("Deleted PersistentVolumeForService '%s'.", k8sVolumeName))
			} else {
				cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeForService ERROR: %s", err.Error()))
			}
		}
		// FIND VOLUME WITH THE RIGHT CLAIM AND DELETE IT
		for _, pv := range pvList.Items {
			if pv.Spec.ClaimRef != nil {
				if pv.Spec.ClaimRef.Name == volumeName && pv.Spec.ClaimRef.Namespace == namespaceName {
					err := pvcClient.Delete(context.TODO(), k8sVolumeName, metav1.DeleteOptions{})
					if err != nil {
						if apierrors.IsNotFound(err) {
							// IN CASE: NOT FOUND -> IT HAS ALREADY BEEN DELETED. e.g. by the provisioneer
							cmd.Success(fmt.Sprintf("Deleted PersistentVolume '%s'.", k8sVolumeName))
						} else {
							cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeForService ERROR: %s", err.Error()))
						}
						return
					} else {
						cmd.Success(fmt.Sprintf("Deleted PersistentVolume '%s'.", k8sVolumeName))
						return
					}
				}
			}
		}
	}(cmd, wg)
	return cmd
}

func CreateMogeniusNfsPersistentVolumeClaimForService(job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Create PersistentVolumeClaim '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Creating PersistentVolumeClaim '%s'.", volumeName))

		pvc := utils.InitMogeniusNfsPersistentVolumeClaimForService()
		pvc.Name = volumeName
		pvc.Namespace = namespaceName
		pvc.Spec.Resources.Requests = v1.ResourceList{}
		pvc.Spec.VolumeName = fmt.Sprintf("%s-%s", namespaceName, volumeName)
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		_, err = pvcClient.Create(context.TODO(), &pvc, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("CreateMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Created PersistentVolumeClaim '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func DeleteMogeniusNfsPersistentVolumeClaimForService(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Delete PersistentVolumeClaim '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting PersistentVolumeClaim '%s'.", volumeName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		err = pvcClient.Delete(context.TODO(), volumeName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeClaimForService ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted PersistentVolumeClaim '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func CreateMogeniusNfsServiceSync(job *structs.Job, namespaceName string, volumeName string) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Create PersistentVolume Service '%s'.", volumeName), job)
	cmd.Start(fmt.Sprintf("Creating PersistentVolume Service '%s'.", volumeName))

	service := utils.InitMogeniusNfsService()
	service.Name = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)
	service.Namespace = namespaceName
	service.Spec.Selector["app"] = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
		return cmd
	}
	serviceClient := provider.ClientSet.CoreV1().Services(namespaceName)
	_, err = serviceClient.Create(context.TODO(), &service, metav1.CreateOptions{})
	if err != nil {
		cmd.Fail(fmt.Sprintf("CreateMogeniusNfsService ERROR: %s", err.Error()))
	} else {
		cmd.Success(fmt.Sprintf("Created PersistentVolume Service '%s'.", volumeName))
	}
	return cmd
}

func DeleteMogeniusNfsService(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) *structs.Command {
	cmd := structs.CreateCommand(fmt.Sprintf("Delete PersistentVolume Service '%s'.", volumeName), job)
	wg.Add(1)
	go func(cmd *structs.Command, wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(fmt.Sprintf("Deleting PersistentVolume Service '%s'.", volumeName))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().Services(namespaceName)
		err = pvcClient.Delete(context.TODO(), fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName), metav1.DeleteOptions{})
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
		deployment.Name = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)
		deployment.Namespace = namespaceName
		deployment.Spec.Template.Labels = make(map[string]string)
		deployment.Spec.Template.Labels["app"] = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)
		deployment.Spec.Selector.MatchLabels = make(map[string]string)
		deployment.Spec.Selector.MatchLabels["app"] = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)
		deployment.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
		_, err = deploymentClient.Create(context.TODO(), &deployment, metav1.CreateOptions{})
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

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
		err = deploymentClient.Delete(context.TODO(), fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName), metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(fmt.Sprintf("DeleteMogeniusNfsDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(fmt.Sprintf("Deleted PersistentVolume Deployment '%s'.", volumeName))
		}
	}(cmd, wg)
	return cmd
}

func ListPersistentVolumeClaimsWithFieldSelector(namespace string, labelSelector string, prefix string) K8sWorkloadResult {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	client := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespace)

	persistentVolumeClaims, err := client.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return WorkloadResult(nil, err)
	}

	// delete all persistentVolumeClaims that do not start with prefix
	if prefix != "" {
		for i := len(persistentVolumeClaims.Items) - 1; i >= 0; i-- {
			if !strings.HasPrefix(persistentVolumeClaims.Items[i].Name, prefix) {
				persistentVolumeClaims.Items = append(persistentVolumeClaims.Items[:i], persistentVolumeClaims.Items[i+1:]...)
			}
		}
	}

	return WorkloadResult(persistentVolumeClaims.Items, err)
}

func GetPersistentVolumeClaim(namespace string, name string) K8sWorkloadResult {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return WorkloadResult(nil, err)
	}
	client := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespace)

	deployment, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return WorkloadResult(nil, err)
	}
	return WorkloadResult(deployment, err)
}
