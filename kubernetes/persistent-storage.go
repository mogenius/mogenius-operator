package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	punq "github.com/mogenius/punq/kubernetes"
	"github.com/mogenius/punq/logger"
	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	scheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

const (
	// PV label keys
	LabelKeyVolumeIdentifier string = "mo-nfs-volume-identifier"
	LabelKeyVolumeName       string = "mo-nfs-volume-name"
	// PV onDelete event reason
	PersitentVolumeKillingEventReason string = "Killing"
	// PV watcher
	persistentVolumeKind       string = "PersistentVolume"
	persistentVolumeAPIVersion string = "v1"
	resourceKind               string = "persistentvolumes"
)

func PersistentVolumes(listOptions *metav1.ListOptions, contextId *string) []v1.PersistentVolume {
	result := []v1.PersistentVolume{}

	provider, err := punq.NewKubeProvider(contextId)
	if err != nil {
		return result
	}

	if listOptions == nil {
		listOptions = &metav1.ListOptions{}
	}

	pvList, err := provider.ClientSet.CoreV1().PersistentVolumes().List(context.TODO(), *listOptions)
	if err != nil {
		logger.Log.Errorf("PersistentVolumes ERROR: %s", err.Error())
		return result
	}

	for _, v := range pvList.Items {
		v.Kind = persistentVolumeKind
		v.APIVersion = persistentVolumeAPIVersion
		result = append(result, v)
	}

	return result
}

func WatchPersistentVolumes() {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		log.Fatalf("Error creating provider for watcher. Cannot continue because it is vital: %s", err.Error())
		return
	}

	// Retry watching resources with exponential backoff in case of failures
	err = retry.OnError(wait.Backoff{
		Steps:    5,
		Duration: 1 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}, apierrors.IsServiceUnavailable, func() error {
		return watchPersistentVolumes(provider, resourceKind)
	})
	if err != nil {
		log.Fatalf("Error watching persistentvolumes: %s", err.Error())
	}

	// Wait forever
	select {}
}

func watchPersistentVolumes(provider *punq.KubeProvider, kindName string) error {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			castedObj := obj.(*v1.PersistentVolume)
			castedObj.Kind = persistentVolumeKind
			castedObj.APIVersion = persistentVolumeAPIVersion

			log.Debugf("Added PersistentVolume: %s", castedObj.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			castedObj := newObj.(*v1.PersistentVolume)
			castedObj.Kind = persistentVolumeKind
			castedObj.APIVersion = persistentVolumeAPIVersion

			log.Debugf("Updated PersistentVolume: %s", castedObj.Name)
		},
		DeleteFunc: func(obj interface{}) {
			castedObj := obj.(*v1.PersistentVolume)
			castedObj.Kind = persistentVolumeKind
			castedObj.APIVersion = persistentVolumeAPIVersion

			log.Debugf("Deleted PersistentVolume: %s", castedObj.Name)

			handlePVDeletion(castedObj, provider)
		},
	}
	listWatch := cache.NewListWatchFromClient(
		provider.ClientSet.CoreV1().RESTClient(),
		kindName,
		v1.NamespaceAll,
		fields.Nothing(),
	)
	resourceInformer := cache.NewSharedInformer(listWatch, &v1.PersistentVolume{}, 0)
	_, err := resourceInformer.AddEventHandler(handler)
	if err != nil {
		return err
	}

	stopCh := make(chan struct{})
	go resourceInformer.Run(stopCh)

	// Wait for the informer to sync and start processing events
	if !cache.WaitForCacheSync(stopCh, resourceInformer.HasSynced) {
		return fmt.Errorf("failed to sync cache")
	}

	// This loop will keep the function alive as long as the stopCh is not closed
	for {
		select {
		case <-stopCh:
			// stopCh closed, return from the function
			return nil
		case <-time.After(30 * time.Second):
			// This is to avoid a tight loop in case stopCh is never closed.
			// You can adjust the time as per your needs.
		}
	}
}

func handlePVDeletion(pv *v1.PersistentVolume, provider *punq.KubeProvider) {
	if !containsLabelKey(pv.Labels, LabelKeyVolumeName) {
		return
	}

	// Extract label value from the PV
	volumeName := getLabelValue(pv.Labels, LabelKeyVolumeName)
	if volumeName == "" {
		log.Warnf("Label value for identifier:'%s' not found on PV %s", LabelKeyVolumeName, pv.Name)
		return
	}

	// Extract namespace from the PV name
	objectMetaName := pv.ObjectMeta.Name
	namespaceName := strings.TrimSuffix(objectMetaName, "-"+volumeName)

	// Set up a dynamic event broadcaster for the specific namespace
	broadcaster := record.NewBroadcaster()
	eventInterface := provider.ClientSet.CoreV1().Events(namespaceName)
	broadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})
	namespaceRecorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "mogenius.io/WatchPersistentVolumes"})

	// Manipulate PV to match the namespace constraint for the event
	pv.ObjectMeta.Namespace = namespaceName
	pv.ObjectMeta.Name = volumeName

	delayDuration := 2 * time.Second
	time.Sleep(delayDuration)

	// Trigger custom event
	log.Infof("PV %s is being deleted in namespace %s, triggering event", objectMetaName, namespaceName)
	namespaceRecorder.Eventf(pv, v1.EventTypeNormal, PersitentVolumeKillingEventReason, "PersistentVolume %s is being deleted", objectMetaName)
}

func getLabelValue(labels map[string]string, labelKey string) string {
	if val, ok := labels[labelKey]; ok {
		return val
	}
	return ""
}

func containsLabelKey(labels map[string]string, key string) bool {
	_, ok := labels[key]
	return ok
}

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

func CreateMogeniusNfsPersistentVolumeClaim(job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create PersistentVolumeClaim", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating PersistentVolumeClaim")

		storageClass := StorageClassForClusterProvider(utils.ClusterProviderCached)
		if storageClass == "" {
			cmd.Fail(job, fmt.Sprintf("No default storageClass found and could not find storage class for cluster provider '%s'.", utils.ClusterProviderCached))
			return
		}

		pvc := utils.InitMogeniusNfsPersistentVolumeClaim()
		pvc.Name = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)
		pvc.Namespace = namespaceName
		pvc.Spec.StorageClassName = punqUtils.Pointer(storageClass)
		pvc.Spec.Resources.Requests = v1.ResourceList{}
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

		// add labels
		pvc.Labels = MoAddLabels(&pvc.Labels, map[string]string{
			LabelKeyVolumeIdentifier: fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName),
			LabelKeyVolumeName:       volumeName,
		})

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		_, err = pvcClient.Create(context.TODO(), &pvc, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Created PersistentVolumeClaim")
		}
	}(wg)
}

func DeleteMogeniusNfsPersistentVolumeClaim(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete PersistentVolumeClaim", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting PersistentVolumeClaim")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		err = pvcClient.Delete(context.TODO(), fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName), metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted PersistentVolumeClaim")
		}
	}(wg)
}

func CreateMogeniusNfsPersistentVolumeForService(job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create PersistentVolume", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		k8sVolumeName := fmt.Sprintf("%s-%s", namespaceName, volumeName)
		cmd.Start(job, "Creating PersistentVolume")

		nfsService := ServiceForNfsVolume(namespaceName, volumeName)
		if nfsService == nil {
			cmd.Fail(job, fmt.Sprintf("CreateMogeniusNfsPersistentVolume ERROR: Could not find service for volume '%s' in order to get IP-Address.", k8sVolumeName))
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
			LabelKeyVolumeIdentifier: fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName),
			LabelKeyVolumeName:       volumeName,
		})

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		client := provider.ClientSet.CoreV1().PersistentVolumes()
		_, err = client.Create(context.TODO(), &pv, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateMogeniusNfsPersistentVolume ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Created PersistentVolume")
		}
	}(wg)
}

func DeleteMogeniusNfsPersistentVolumeForService(job *structs.Job, volumeName string, namespaceName string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete PersistentVolume", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		k8sVolumeName := fmt.Sprintf("%s-%s", namespaceName, volumeName)
		cmd.Start(job, "Deleting DeleteMogeniusNfsPersistentVolumeForService")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumes()

		// LIST ALL PV
		pvList, err := pvcClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// IN CASE: NOT FOUND -> IT HAS ALREADY BEEN DELETED. e.g. by the provisioneer
				cmd.Success(job, "Deleted PersistentVolumeForService")
			} else {
				cmd.Fail(job, fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeForService ERROR: %s", err.Error()))
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
							cmd.Success(job, "Deleted PersistentVolume")
						} else {
							cmd.Fail(job, fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeForService ERROR: %s", err.Error()))
						}
						return
					} else {
						cmd.Success(job, "Deleted PersistentVolume")
						return
					}
				}
			}
		}
	}(wg)
}

func CreateMogeniusNfsPersistentVolumeClaimForService(job *structs.Job, namespaceName string, volumeName string, volumeSizeInGb int, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create PersistentVolumeClaim", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating PersistentVolumeClaim '%s'.")

		pvc := utils.InitMogeniusNfsPersistentVolumeClaimForService()
		pvc.Name = volumeName
		pvc.Namespace = namespaceName
		pvc.Spec.Resources.Requests = v1.ResourceList{}
		pvc.Spec.VolumeName = fmt.Sprintf("%s-%s", namespaceName, volumeName)
		pvc.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", volumeSizeInGb))

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		_, err = pvcClient.Create(context.TODO(), &pvc, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateMogeniusNfsPersistentVolumeClaim ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Created PersistentVolumeClaim")
		}
	}(wg)
}

func DeleteMogeniusNfsPersistentVolumeClaimForService(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete PersistentVolumeClaim", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting PersistentVolumeClaim")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().PersistentVolumeClaims(namespaceName)
		err = pvcClient.Delete(context.TODO(), volumeName, metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteMogeniusNfsPersistentVolumeClaimForService ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted PersistentVolumeClaim")
		}
	}(wg)
}

func CreateMogeniusNfsServiceSync(job *structs.Job, namespaceName string, volumeName string) {
	cmd := structs.CreateCommand("create", "Create PersistentVolume Service", job)
	cmd.Start(job, "Creating PersistentVolume Service")

	service := utils.InitMogeniusNfsService()
	service.Name = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)
	service.Namespace = namespaceName
	service.Spec.Selector["app"] = fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)

	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
		return
	}
	serviceClient := provider.ClientSet.CoreV1().Services(namespaceName)
	_, err = serviceClient.Create(context.TODO(), &service, metav1.CreateOptions{})
	if err != nil {
		cmd.Fail(job, fmt.Sprintf("CreateMogeniusNfsService ERROR: %s", err.Error()))
	} else {
		cmd.Success(job, "Created PersistentVolume")
	}
}

func DeleteMogeniusNfsService(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Delete PersistentVolume Service", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting PersistentVolume Service")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		pvcClient := provider.ClientSet.CoreV1().Services(namespaceName)
		err = pvcClient.Delete(context.TODO(), fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName), metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteMogeniusNfsService ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted PersistentVolume Service")
		}
	}(wg)
}

func CreateMogeniusNfsDeployment(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("create", "Create PersistentVolume Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Creating PersistentVolume Deployment")

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
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
		_, err = deploymentClient.Create(context.TODO(), &deployment, metav1.CreateOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("CreateMogeniusNfsDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Created PersistentVolume Deployment")
		}
	}(wg)
}

func DeleteMogeniusNfsDeployment(job *structs.Job, namespaceName string, volumeName string, wg *sync.WaitGroup) {
	cmd := structs.CreateCommand("delete", "Delete PersistentVolume Deployment", job)
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		cmd.Start(job, "Deleting PersistentVolume Deployment")

		provider, err := punq.NewKubeProvider(nil)
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("ERROR: %s", err.Error()))
			return
		}
		deploymentClient := provider.ClientSet.AppsV1().Deployments(namespaceName)
		err = deploymentClient.Delete(context.TODO(), fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName), metav1.DeleteOptions{})
		if err != nil {
			cmd.Fail(job, fmt.Sprintf("DeleteMogeniusNfsDeployment ERROR: %s", err.Error()))
		} else {
			cmd.Success(job, "Deleted PersistentVolume Deployment")
		}
	}(wg)
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
