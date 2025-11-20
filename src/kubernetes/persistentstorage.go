package kubernetes

import (
	"context"
	"fmt"
	"mogenius-operator/src/structs"
	"mogenius-operator/src/utils"
	"mogenius-operator/src/websocket"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	scheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	// PV label keys
	LabelKeyVolumeIdentifier string = "mo-nfs-volume-identifier"
	LabelKeyVolumeName       string = "mo-nfs-volume-name"
	// PV onDelete event reason
	PersitentVolumeKillingEventReason string = "Killing"
)

func handlePVDeletion(pv *v1.PersistentVolume) {
	clientset := clientProvider.K8sClientSet()

	if !ContainsLabelKey(pv.Labels, LabelKeyVolumeName) {
		return
	}

	// Extract label value from the PV
	volumeName, err := GetLabelValue(pv.Labels, LabelKeyVolumeName)
	if err != nil {
		k8sLogger.Warn("Label value for identifier:'%s' not found on PV %s", LabelKeyVolumeName, pv.Name)
		return
	}

	// Extract namespace from the PV name
	objectMetaName := pv.ObjectMeta.Name
	namespaceName := strings.TrimSuffix(objectMetaName, "-"+volumeName)

	// Set up a dynamic event broadcaster for the specific namespace
	broadcaster := record.NewBroadcaster()
	eventInterface := clientset.CoreV1().Events(namespaceName)
	broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})
	namespaceRecorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mogenius.io/WatchPersistentVolumes"})

	// Manipulate PV to match the namespace constraint for the event
	pv.ObjectMeta.Namespace = namespaceName
	pv.ObjectMeta.Name = volumeName

	delayDuration := 2 * time.Second
	time.Sleep(delayDuration)

	// Trigger custom event
	k8sLogger.Info("PV %s is being deleted in namespace %s, triggering event", objectMetaName, namespaceName)
	namespaceRecorder.Eventf(pv, v1.EventTypeNormal, PersitentVolumeKillingEventReason, "PersistentVolume %s is being deleted", objectMetaName)
}

func GetVolumeMountsForK8sManager() ([]structs.Volume, error) {
	result := []structs.Volume{}

	clientset := clientProvider.K8sClientSet()
	pvcClient := clientset.CoreV1().PersistentVolumeClaims("")
	pvcList, err := pvcClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return result, err
	}
	for _, pvc := range pvcList.Items {
		if strings.HasPrefix(pvc.Name, utils.NFS_POD_PREFIX) {
			capacity := pvc.Spec.Resources.Requests[v1.ResourceStorage]
			volName := strings.Replace(pvc.Name, fmt.Sprintf("%s-", utils.NFS_POD_PREFIX), "", 1)
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

func CreateMogeniusNfsPersistentVolumeClaim(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int) {
	cmd := structs.CreateCommand(eventClient, "create", "Create PersistentVolumeClaim", job)
	cmd.Start(eventClient, job, "Creating PersistentVolumeClaim")

	storageClass := StorageClassForClusterProvider(utils.ClusterProviderCached)
	if storageClass == "" {
		cmd.Fail(eventClient, job, fmt.Sprintf("No default storageClass found and could not find storage class for cluster provider '%s'.", utils.ClusterProviderCached))
		return
	}

	pvc := utils.InitMogeniusNfsPersistentVolumeClaim()
	pvc.Name = fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName)
	pvc.Namespace = namespaceName
	pvc.Spec.StorageClassName = utils.Pointer(storageClass)
	pvc.Spec.Resources.Requests = v1.ResourceList{}
	pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

	// add labels
	pvc.Labels = MoAddLabels(&pvc.Labels, map[string]string{
		LabelKeyVolumeIdentifier: fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName),
		LabelKeyVolumeName:       volumeName,
	})

	clientset := clientProvider.K8sClientSet()
	pvcClient := clientset.CoreV1().PersistentVolumeClaims(namespaceName)
	_, err := pvcClient.Create(context.Background(), &pvc, metav1.CreateOptions{})
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("CreateMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Created PersistentVolumeClaim")
	}
}

func DeleteMogeniusNfsPersistentVolumeClaim(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName string, volumeName string) {
	cmd := structs.CreateCommand(eventClient, "delete", "Delete PersistentVolumeClaim", job)
	cmd.Start(eventClient, job, "Deleting PersistentVolumeClaim")

	clientset := clientProvider.K8sClientSet()
	pvcClient := clientset.CoreV1().PersistentVolumeClaims(namespaceName)
	err := pvcClient.Delete(context.Background(), fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName), metav1.DeleteOptions{})
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Deleted PersistentVolumeClaim")
	}
}

func CreateMogeniusNfsPersistentVolumeForService(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int) {
	cmd := structs.CreateCommand(eventClient, "create", "Create PersistentVolume", job)
	k8sVolumeName := fmt.Sprintf("%s-%s", namespaceName, volumeName)
	cmd.Start(eventClient, job, "Creating PersistentVolume")

	nfsService := ServiceForNfsVolume(namespaceName, volumeName)
	if nfsService == nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("CreateMogeniusNfsPersistentVolume ERROR: Could not find service for volume '%s' in order to get IP-Address.", k8sVolumeName))
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
		LabelKeyVolumeIdentifier: fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName),
		LabelKeyVolumeName:       volumeName,
	})

	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().PersistentVolumes()
	_, err := client.Create(context.Background(), &pv, metav1.CreateOptions{})
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("CreateMogeniusNfsPersistentVolume ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Created PersistentVolume")
	}
}

func DeleteMogeniusNfsPersistentVolumeForService(eventClient websocket.WebsocketClient, job *structs.Job, volumeName string, namespaceName string) {
	cmd := structs.CreateCommand(eventClient, "delete", "Delete PersistentVolume", job)
	k8sVolumeName := fmt.Sprintf("%s-%s", namespaceName, volumeName)
	cmd.Start(eventClient, job, "Deleting DeleteMogeniusNfsPersistentVolumeForService")

	clientset := clientProvider.K8sClientSet()
	pvcClient := clientset.CoreV1().PersistentVolumes()

	// LIST ALL PV
	pvList, err := pvcClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// IN CASE: NOT FOUND -> IT HAS ALREADY BEEN DELETED. e.g. by the provisioneer
			cmd.Success(eventClient, job, "Deleted PersistentVolumeForService")
		} else {
			cmd.Fail(eventClient, job, fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeForService ERROR: %s", err.Error()))
		}
	}
	// FIND VOLUME WITH THE RIGHT CLAIM AND DELETE IT
	for _, pv := range pvList.Items {
		if pv.Spec.ClaimRef != nil {
			if pv.Spec.ClaimRef.Name == volumeName && pv.Spec.ClaimRef.Namespace == namespaceName {
				err := pvcClient.Delete(context.Background(), k8sVolumeName, metav1.DeleteOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						// IN CASE: NOT FOUND -> IT HAS ALREADY BEEN DELETED. e.g. by the provisioneer
						cmd.Success(eventClient, job, "Deleted PersistentVolume")
					} else {
						cmd.Fail(eventClient, job, fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeForService ERROR: %s", err.Error()))
					}
					return
				} else {
					cmd.Success(eventClient, job, "Deleted PersistentVolume")
					return
				}
			}
		}
	}
}

func CreateMogeniusNfsPersistentVolumeClaimForService(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int) {
	cmd := structs.CreateCommand(eventClient, "create", "Create PersistentVolumeClaim", job)
	cmd.Start(eventClient, job, "Creating PersistentVolumeClaim '%s'.")

	pvc := utils.InitMogeniusNfsPersistentVolumeClaimForService()
	pvc.Name = volumeName
	pvc.Namespace = namespaceName
	pvc.Spec.Resources.Requests = v1.ResourceList{}
	pvc.Spec.VolumeName = fmt.Sprintf("%s-%s", namespaceName, volumeName)
	pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

	clientset := clientProvider.K8sClientSet()
	pvcClient := clientset.CoreV1().PersistentVolumeClaims(namespaceName)
	_, err := pvcClient.Create(context.Background(), &pvc, metav1.CreateOptions{})
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("CreateMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Created PersistentVolumeClaim")
	}
}

func DeleteMogeniusNfsPersistentVolumeClaimForService(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName string, volumeName string) {
	cmd := structs.CreateCommand(eventClient, "delete", "Delete PersistentVolumeClaim", job)
	cmd.Start(eventClient, job, "Deleting PersistentVolumeClaim")

	clientset := clientProvider.K8sClientSet()
	pvcClient := clientset.CoreV1().PersistentVolumeClaims(namespaceName)
	err := pvcClient.Delete(context.Background(), volumeName, metav1.DeleteOptions{})
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeClaimForService ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Deleted PersistentVolumeClaim")
	}
}

func CreateMogeniusNfsServiceSync(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName string, volumeName string) {
	cmd := structs.CreateCommand(eventClient, "create", "Create PersistentVolume Service", job)
	cmd.Start(eventClient, job, "Creating PersistentVolume Service")

	service := utils.InitMogeniusNfsService()
	service.Name = fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName)
	service.Namespace = namespaceName
	service.Spec.Selector["app"] = fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName)

	clientset := clientProvider.K8sClientSet()
	serviceClient := clientset.CoreV1().Services(namespaceName)
	_, err := serviceClient.Create(context.Background(), &service, metav1.CreateOptions{})
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("CreateMogeniusNfsService ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Created PersistentVolume")
	}
}

func DeleteMogeniusNfsService(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName string, volumeName string) {
	cmd := structs.CreateCommand(eventClient, "create", "Delete PersistentVolume Service", job)
	cmd.Start(eventClient, job, "Deleting PersistentVolume Service")

	clientset := clientProvider.K8sClientSet()
	pvcClient := clientset.CoreV1().Services(namespaceName)
	err := pvcClient.Delete(context.Background(), fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName), metav1.DeleteOptions{})
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("DeleteMogeniusNfsService ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Deleted PersistentVolume Service")
	}
}

func CreateMogeniusNfsDeployment(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName string, volumeName string) {
	cmd := structs.CreateCommand(eventClient, "create", "Create PersistentVolume Deployment", job)
	cmd.Start(eventClient, job, "Creating PersistentVolume Deployment")

	deployment := utils.InitMogeniusNfsDeployment()
	deployment.Name = fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName)
	deployment.Namespace = namespaceName
	deployment.Spec.Template.Labels = make(map[string]string)
	deployment.Spec.Template.Labels["app"] = fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName)
	deployment.Spec.Selector.MatchLabels = make(map[string]string)
	deployment.Spec.Selector.MatchLabels["app"] = fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName)
	deployment.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName)

	clientset := clientProvider.K8sClientSet()
	deploymentClient := clientset.AppsV1().Deployments(namespaceName)
	_, err := deploymentClient.Create(context.Background(), &deployment, metav1.CreateOptions{})
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("CreateMogeniusNfsDeployment ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Created PersistentVolume Deployment")
	}
}

func DeleteMogeniusNfsDeployment(eventClient websocket.WebsocketClient, job *structs.Job, namespaceName string, volumeName string) {
	cmd := structs.CreateCommand(eventClient, "delete", "Delete PersistentVolume Deployment", job)
	cmd.Start(eventClient, job, "Deleting PersistentVolume Deployment")

	clientset := clientProvider.K8sClientSet()
	deploymentClient := clientset.AppsV1().Deployments(namespaceName)
	err := deploymentClient.Delete(context.Background(), fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName), metav1.DeleteOptions{})
	if err != nil {
		cmd.Fail(eventClient, job, fmt.Sprintf("DeleteMogeniusNfsDeployment ERROR: %s", err.Error()))
	} else {
		cmd.Success(eventClient, job, "Deleted PersistentVolume Deployment")
	}
}

func ListPersistentVolumeClaimsWithFieldSelector(namespace string, labelSelector string, prefix string) ([]v1.PersistentVolumeClaim, error) {
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().PersistentVolumeClaims(namespace)

	persistentVolumeClaims, err := client.List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return persistentVolumeClaims.Items, err
	}

	// delete all persistentVolumeClaims that do not start with prefix
	if prefix != "" {
		for i := len(persistentVolumeClaims.Items) - 1; i >= 0; i-- {
			if !strings.HasPrefix(persistentVolumeClaims.Items[i].Name, prefix) {
				persistentVolumeClaims.Items = append(persistentVolumeClaims.Items[:i], persistentVolumeClaims.Items[i+1:]...)
			}
		}
	}

	return persistentVolumeClaims.Items, err
}
