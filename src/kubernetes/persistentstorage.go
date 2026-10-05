package kubernetes

import (
	"context"
	"fmt"
	"mogenius-operator/src/shutdown"
	"mogenius-operator/src/utils"
	"slices"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// pvEventRecorder is shared by all PV deletions. Previously every deletion
// built its own record.NewBroadcaster() and never called Shutdown() on it,
// leaking the broadcaster's goroutines and watch fan-out per deleted PV for
// the lifetime of the process. A sink on Events("") creates each event in
// the namespace of the object it is recorded against, so one recorder
// serves every namespace.
var (
	pvEventRecorderOnce sync.Once
	pvEventRecorder     record.EventRecorder
)

// pvDeletionEventDelay orders the Killing event after the PV deletion that
// triggers it.
const pvDeletionEventDelay = 2 * time.Second

func getPvEventRecorder() record.EventRecorder {
	pvEventRecorderOnce.Do(func() {
		broadcaster := record.NewBroadcaster()
		broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
			Interface: clientProvider.K8sClientSet().CoreV1().Events(""),
		})
		shutdown.Add(broadcaster.Shutdown)
		pvEventRecorder = broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mogenius.io/WatchPersistentVolumes"})
	})
	return pvEventRecorder
}

// handlePVDeletion records a Killing event in the namespace a mogenius NFS
// volume belonged to. It runs on the informer delete handler and must return
// promptly: the store deletion of the PV waits behind it.
func handlePVDeletion(pv *v1.PersistentVolume) {
	if !ContainsLabelKey(pv.Labels, LabelKeyVolumeName) {
		return
	}

	// Extract label value from the PV
	volumeName, err := GetLabelValue(pv.Labels, LabelKeyVolumeName)
	if err != nil {
		k8sLogger.Warn("Label value not found on PV", "label", LabelKeyVolumeName, "pv", pv.Name)
		return
	}

	// Extract namespace from the PV name
	objectMetaName := pv.Name
	namespaceName := strings.TrimSuffix(objectMetaName, "-"+volumeName)

	// Manipulate PV to match the namespace constraint for the event. pv is a
	// typed copy converted by the caller, not the informer's cached object.
	pv.Namespace = namespaceName
	pv.Name = volumeName

	recorder := getPvEventRecorder()

	// Delay off the handler goroutine instead of sleeping on it: a bulk PV
	// cleanup used to stall the store's delete path for 2s per PV.
	time.AfterFunc(pvDeletionEventDelay, func() {
		k8sLogger.Info("PV is being deleted, triggering event", "pv", objectMetaName, "namespace", namespaceName)
		recorder.Eventf(pv, v1.EventTypeNormal, PersitentVolumeKillingEventReason, "PersistentVolume %s is being deleted", objectMetaName)
	})
}

// This functions are used to generate the mogenius custom nfs storage solution
// The order is importent when creating:
// 1. PVC
// 2. PV
// 3. DEPLOYMENT
// 4. SERVICE

// CleanupNfsVolumesInNamespace removes all mogenius NFS volume resources
// (Deployment, Service, PVC, PV, and service PVC) for the given namespace.
// This should be called before deleting a workspace to prevent orphaned storage.
func CleanupNfsVolumesInNamespace(namespaceName string) {
	clientset := clientProvider.K8sClientSet()
	pvcClient := clientset.CoreV1().PersistentVolumeClaims(namespaceName)
	pvcList, err := pvcClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Warn("failed to list PVCs for NFS cleanup", "namespace", namespaceName, "error", err)
		return
	}

	prefix := fmt.Sprintf("%s-", utils.NFS_POD_PREFIX)
	for _, pvc := range pvcList.Items {
		if !strings.HasPrefix(pvc.Name, prefix) {
			continue
		}
		volumeName := strings.TrimPrefix(pvc.Name, prefix)
		k8sLogger.Info("cleaning up NFS volume before workspace deletion", "namespace", namespaceName, "volume", volumeName)

		// Delete deployment
		err := clientset.AppsV1().Deployments(namespaceName).Delete(context.Background(), fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName), metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			k8sLogger.Warn("failed to delete NFS deployment during cleanup", "namespace", namespaceName, "volume", volumeName, "error", err)
		}

		// Delete service
		err = clientset.CoreV1().Services(namespaceName).Delete(context.Background(), fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName), metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			k8sLogger.Warn("failed to delete NFS service during cleanup", "namespace", namespaceName, "volume", volumeName, "error", err)
		}

		// Delete the NFS server PVC
		err = pvcClient.Delete(context.Background(), pvc.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			k8sLogger.Warn("failed to delete NFS PVC during cleanup", "namespace", namespaceName, "volume", volumeName, "error", err)
		}

		// Delete the cluster-scoped PV (named {namespace}-{volumeName})
		pvName := fmt.Sprintf("%s-%s", namespaceName, volumeName)
		err = clientset.CoreV1().PersistentVolumes().Delete(context.Background(), pvName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			k8sLogger.Warn("failed to delete NFS PV during cleanup", "namespace", namespaceName, "volume", volumeName, "error", err)
		}

		// Delete the service PVC (named just {volumeName})
		err = pvcClient.Delete(context.Background(), volumeName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			k8sLogger.Warn("failed to delete NFS service PVC during cleanup", "namespace", namespaceName, "volume", volumeName, "error", err)
		}
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
		persistentVolumeClaims.Items = slices.DeleteFunc(persistentVolumeClaims.Items, func(pvc v1.PersistentVolumeClaim) bool {
			return !strings.HasPrefix(pvc.Name, prefix)
		})
	}

	return persistentVolumeClaims.Items, err
}
