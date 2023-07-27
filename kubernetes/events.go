package kubernetes

import (
	"context"
	"log"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"os/exec"
	"strconv"
	"time"

	v1Core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

// var lastMountedPaths = []string{}
// var eventsFirstStart = true

//var waitList = []structs.WaitListEntry{}

func WatchEvents() {
	kubeProvider := NewKubeProvider()

	var lastResourceVersion = ""
	for {
		// Create a watcher for all Kubernetes events
		watcher, err := kubeProvider.ClientSet.CoreV1().Events("").Watch(context.TODO(), v1.ListOptions{Watch: true, ResourceVersion: lastResourceVersion})

		if err != nil || watcher == nil {
			if apierrors.IsGone(err) {
				lastResourceVersion = ""
			}
			log.Printf("Error creating watcher: %v", err)
			time.Sleep(RETRYTIMEOUT * time.Second) // Wait for 5 seconds before retrying
			continue
		} else {
			logger.Log.Notice("Watcher connected successfully. Start watching events...")
		}

		// Start watching events
		for event := range watcher.ResultChan() {
			if event.Object != nil {
				eventDto := dtos.CreateEvent(string(event.Type), event.Object)
				datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto)

				eventObj, isEvent := event.Object.(*v1Core.Event)
				if isEvent {
					currentVersion, errCurrentVer := strconv.Atoi(eventObj.ObjectMeta.ResourceVersion)
					lastVersion, _ := strconv.Atoi(lastResourceVersion)
					if errCurrentVer == nil {
						lastResourceVersion = eventObj.ObjectMeta.ResourceVersion
						message := eventObj.Message
						kind := eventObj.InvolvedObject.Kind
						reason := eventObj.Reason
						count := eventObj.Count
						if currentVersion > lastVersion {

							// processWaitList(eventObj)
							structs.EventServerSendData(datagram, kind, reason, message, count)
						} else {
							logger.Log.Errorf("Versions are out of order: %d / %d", lastVersion, currentVersion)
							logger.Log.Errorf("%s/%s -> %s (Count: %d)\n", kind, reason, message, count)
						}
					}
				} else if event.Type == "ERROR" {
					var errObj *v1.Status = event.Object.(*v1.Status)
					logger.Log.Errorf("WATCHER (%d): '%s'", errObj.Code, errObj.Message)
					logger.Log.Error("WATCHER: Reset lastResourceVersion to empty.")
					lastResourceVersion = ""
					time.Sleep(RETRYTIMEOUT * time.Second) // Wait for 5 seconds before retrying
					break
				}
			}
		}

		// If the watcher channel is closed, wait for 5 seconds before retrying
		logger.Log.Errorf("Watcher channel closed. Waiting before retrying with '%s' ...", lastResourceVersion)
		watcher.Stop()
		time.Sleep(RETRYTIMEOUT * time.Second)
	}
}

// func AppendToWaitList(entry structs.WaitListEntry) {
// 	waitList = append(waitList, entry)
// }

// func processWaitList(event *v1Core.Event) {
// 	if event != nil {
// 		message := event.Message
// 		kind := event.InvolvedObject.Kind
// 		reason := event.Reason

// 		for index, waitListEntry := range waitList {
// 			if waitListEntry.IsExpired() {
// 				waitListEntry.Job.AddCmd(structs.CreateCommand("Operation timed out.", &waitListEntry.Job))
// 				waitListEntry.Job.Finish()
// 				utils.Remove(waitList, index)
// 			}
// 			if waitListEntry.WaitForKind == kind && waitListEntry.WaitForReason == reason && waitListEntry.WaitForMessage == message {
// 				waitListEntry.Job.Finish()
// 				utils.Remove(waitList, index)
// 			}
// 		}
// 	}
// }

// func UpdateK8sManagerVolumeMounts(deleteVolumeName string, deleteVolumeNamespace string) error {
// 	// EXIT if started locally
// 	if !utils.CONFIG.Kubernetes.RunInCluster {
// 		return nil
// 	}
// 	// EXIT if AutoMountNfs is disabled
// 	if !utils.CONFIG.Misc.AutoMountNfs {
// 		return nil
// 	}

// 	allMountedPaths := []string{}

// 	time.Sleep(2 * time.Second)

// 	// 1: LIST all matching PersistentVolumes
// 	allPvcs := AllPersistentVolumeClaims("")
// 	mogeniusPvs := []v1Core.PersistentVolumeClaim{}
// 	for _, pvc := range allPvcs {
// 		if pvc.Spec.StorageClassName != nil {
// 			if *pvc.Spec.StorageClassName == "openebs-kernel-nfs" && pvc.Status.Phase == v1Core.ClaimBound {
// 				if pvc.Namespace != deleteVolumeNamespace && pvc.Name != deleteVolumeName {
// 					mogeniusPvs = append(mogeniusPvs, pvc)
// 				}
// 			}
// 		}
// 	}

// 	// 2: Get own deployment for future update
// 	kubeProvider := NewKubeProvider()
// 	deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(NAMESPACE)
// 	ownDeployment, err := deploymentClient.Get(context.TODO(), DEPLOYMENTNAME, v1.GetOptions{})
// 	if err != nil {
// 		return err
// 	}

// 	// 3: Update own Deployment
// 	hasBeenUpdated := false
// 	if len(mogeniusPvs) > 0 {
// 		for _, mopvc := range mogeniusPvs {
// 			mountPath := utils.MountPath(mopvc.Namespace, mopvc.Name, "/")
// 			allMountedPaths = append(allMountedPaths, mountPath)
// 			// 3.1 Add VolumeMount
// 			ownDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = appendVolumeMountIfNotExists(ownDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
// 				MountPath: mountPath,
// 				Name:      mopvc.Name,
// 			})
// 			// 3.2 Add Volume
// 			ownDeployment.Spec.Template.Spec.Volumes = appendVolumeIfNotExists(ownDeployment.Spec.Template.Spec.Volumes, core.Volume{
// 				Name: mopvc.Name,
// 				VolumeSource: core.VolumeSource{
// 					PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
// 						ClaimName: fmt.Sprintf("nfs-%s", mopvc.Spec.VolumeName),
// 					},
// 				},
// 			})
// 		}
// 	} else {
// 		ownDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = []core.VolumeMount{}
// 		ownDeployment.Spec.Template.Spec.Volumes = []core.Volume{}
// 	}

// 	// List all mounts marking new ones
// 	logger.Log.Infof("Currently mounted Volumes (%d):", len(lastMountedPaths))
// 	for index, currentMountPath := range lastMountedPaths {
// 		logger.Log.Infof("%d: %s", index+1, currentMountPath)
// 	}

// 	// check if something changed
// 	diff := utils.Diff(allMountedPaths, lastMountedPaths)
// 	if len(diff) > 0 {
// 		for index, diffPath := range diff {
// 			logger.Log.Infof("CHANGED (%d): %s", index+1, diffPath)
// 		}
// 		hasBeenUpdated = true
// 	}

// 	// 5: Redeploy on up
// 	if hasBeenUpdated || eventsFirstStart || deleteVolumeName != "" {
// 		lastMountedPaths = allMountedPaths
// 		_, err := deploymentClient.Update(context.TODO(), ownDeployment, v1.UpdateOptions{})
// 		if err != nil {
// 			return err
// 		}
// 		eventsFirstStart = false
// 	}
// 	return nil
// }

func appendVolumeMountIfNotExists(items []v1Core.VolumeMount, newItem v1Core.VolumeMount) []v1Core.VolumeMount {
	for _, item := range items {
		if item.Name == newItem.Name {
			// The item is already in the slice, so return the original slice.
			return items
		}
	}
	// The item was not found, so add it to the slice.
	return append(items, newItem)
}

func appendVolumeIfNotExists(items []v1Core.Volume, newItem v1Core.Volume) []v1Core.Volume {
	for _, item := range items {
		if item.Name == newItem.Name {
			// The item is already in the slice, so return the original slice.
			return items
		}
	}
	// The item was not found, so add it to the slice.
	return append(items, newItem)
}

func AllEvents(namespaceName string) K8sWorkloadResult {
	result := []v1Core.Event{}

	provider := NewKubeProvider()
	eventList, err := provider.ClientSet.CoreV1().Events(namespaceName).List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system"})
	if err != nil {
		logger.Log.Errorf("AllEvents ERROR: %s", err.Error())
		return WorkloadResult(nil, err)
	}

	for _, event := range eventList.Items {
		if !utils.Contains(utils.CONFIG.Misc.IgnoreNamespaces, event.ObjectMeta.Namespace) {
			result = append(result, event)
		}
	}
	return WorkloadResult(result, nil)
}

func DescribeK8sEvent(namespace string, name string) K8sWorkloadResult {
	cmd := exec.Command("kubectl", "describe", "event", name, "-n", namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log.Errorf("Failed to execute command (%s): %v", cmd.String(), err)
		logger.Log.Errorf("Error: %s", string(output))
		return WorkloadResult(nil, string(output))
	}
	return WorkloadResult(string(output), nil)
}
