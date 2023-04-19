package kubernetes

import (
	"context"
	"log"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"reflect"
	"strconv"
	"strings"
	"time"

	core "k8s.io/api/core/v1"
	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const RETRYTIMEOUT time.Duration = 3
const CONCURRENTCONNECTIONS = 1

var lastResourceVersion = ""
var lastMountedPaths = []string{}
var eventsFirstStart = true

func WatchEvents() {
	kubeProvider := NewKubeProvider()

	for {
		// Create a watcher for all Kubernetes events
		watcher, err := kubeProvider.ClientSet.CoreV1().Events("").Watch(context.TODO(), v1.ListOptions{Watch: true, ResourceVersion: lastResourceVersion})
		if err != nil {
			log.Printf("Error creating watcher: %v", err)
			time.Sleep(RETRYTIMEOUT * time.Second) // Wait for 5 seconds before retrying
			continue
		}

		// Start watching events
		for event := range watcher.ResultChan() {
			if event.Object != nil {
				eventDto := dtos.CreateEvent(string(event.Type), event.Object)
				datagram := structs.CreateDatagramFrom("KubernetesEvent", eventDto, nil)

				if reflect.TypeOf(event.Object).String() == "*v1.Event" {
					var eventObj *v1Core.Event = event.Object.(*v1Core.Event)
					currentVersion, _ := strconv.Atoi(eventObj.ObjectMeta.ResourceVersion)
					lastVersion, _ := strconv.Atoi(lastResourceVersion)
					if currentVersion > lastVersion {
						lastResourceVersion = eventObj.ObjectMeta.ResourceVersion
						message := eventObj.Message
						kind := eventObj.InvolvedObject.Kind
						reason := eventObj.Reason
						count := eventObj.Count
						if kind == "Pod" && reason == "Started" && strings.HasPrefix(message, "Started container nfs-server") {
							// || (reason == "Killing" && strings.HasPrefix(message, "Stopping container nfs-server"))
							err := UpdateK8sManagerVolumeMounts("", "")
							if err != nil {
								logger.Log.Errorf("UpdateK8sManagerVolumeMounts ERROR: %s", err.Error())
							}
						}
						// if kind == "PersistentVolumeClaim" {
						// 	if reason == "ProvisioningSucceeded" && strings.HasPrefix(message, "Successfully provisioned volume pvc-") {
						// 		// 1. get pvc
						// 		// 2. check if storageclass is "openebs-rwx"

						// 	} else if reason == "ProvisioningSucceeded" && strings.HasPrefix(message, "Successfully provisioned volume pvc-") {

						// 	}
						// 	fmt.Println(eventObj)
						// }
						//fmt.Println(eventObj)

						structs.EventServerSendData(datagram, kind, reason, message, count)
					}
				} else if event.Type == "ERROR" {
					var errObj *v1.Status = event.Object.(*v1.Status)
					logger.Log.Errorf("WATCHER (%d): '%s'", errObj.Code, errObj.Message)
					logger.Log.Error("WATCHER: Reset lastResourceVersion to empty.")
					lastResourceVersion = ""
				}
			}
		}

		// If the watcher channel is closed, wait for 5 seconds before retrying
		logger.Log.Errorf("Watcher channel closed. Waiting before retrying with '%s' ...", lastResourceVersion)
		time.Sleep(RETRYTIMEOUT * time.Second)
	}
}

func UpdateK8sManagerVolumeMounts(deleteVolumeName string, deleteVolumeNamespace string) error {
	allMountedPaths := []string{}

	time.Sleep(2 * time.Second)

	// 1: LIST all matching PersistentVolumes
	allPvcs := AllPersistentVolumeClaims("")
	mogeniusPvs := []v1Core.PersistentVolumeClaim{}
	for _, pvc := range allPvcs {
		if pvc.Spec.StorageClassName != nil {
			if *pvc.Spec.StorageClassName == "openebs-kernel-nfs" && pvc.Status.Phase == v1Core.ClaimBound {
				if pvc.Namespace != deleteVolumeNamespace && pvc.Name != deleteVolumeName {
					mogeniusPvs = append(mogeniusPvs, pvc)
				}
			}
		}
	}

	// 2: Get own deployment for future update
	kubeProvider := NewKubeProvider()
	deploymentClient := kubeProvider.ClientSet.AppsV1().Deployments(NAMESPACE)
	ownDeployment, err := deploymentClient.Get(context.TODO(), DEPLOYMENTNAME, v1.GetOptions{})
	if err != nil {
		return err
	}

	// 3: Update own Deployment
	hasBeenUpdated := false
	if len(mogeniusPvs) > 0 {
		for _, mopvc := range mogeniusPvs {
			mountPath := utils.MountPath(mopvc.Namespace, mopvc.Name, "/")
			allMountedPaths = append(allMountedPaths, mountPath)
			// 3.1 Add VolumeMount
			ownDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(ownDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, core.VolumeMount{
				MountPath: mountPath,
				Name:      mopvc.Name,
			})
			// 3.2 Add Volume
			ownDeployment.Spec.Template.Spec.Volumes = append(ownDeployment.Spec.Template.Spec.Volumes, core.Volume{
				Name: mopvc.Name,
				VolumeSource: core.VolumeSource{
					PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
						ClaimName: mopvc.Name,
					},
				},
			})
		}
	} else {
		ownDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = []core.VolumeMount{}
		ownDeployment.Spec.Template.Spec.Volumes = []core.Volume{}
	}

	// List all mounts marking new ones
	logger.Log.Infof("Currently mounted Volumes (%d):", len(lastMountedPaths))
	for index, currentMountPath := range lastMountedPaths {
		logger.Log.Infof("%d: %s", index+1, currentMountPath)
	}

	// check if something changed
	diff := utils.Diff(allMountedPaths, lastMountedPaths)
	if len(diff) > 0 {
		for index, diffPath := range diff {
			logger.Log.Infof("CHANGED (%d): %s", index+1, diffPath)
		}
		hasBeenUpdated = true
	}

	// 5: Redeploy on up
	if hasBeenUpdated || eventsFirstStart || deleteVolumeName != "" {
		lastMountedPaths = allMountedPaths
		deploymentClient.Update(context.TODO(), ownDeployment, v1.UpdateOptions{})
		eventsFirstStart = false
	}

	// 1. list all pvc
	// 2. generate list of all volumes/volumemounts
	// 3. check if volume has already been mounted
	// 4. redeploy own deployment
	return nil
}
