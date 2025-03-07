package kubernetes

import (
	"mogenius-k8s-manager/src/utils"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8sController struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func NewK8sController(kind string, name string, namespace string) K8sController {
	return K8sController{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}
}

func (owner *K8sController) Identifier() string {
	return owner.Namespace
}

var ownerCache = make(map[string]K8sController)

var dataLock sync.Mutex = sync.Mutex{}

func ControllerForPod(namespace string, podName string) *K8sController {
	// check if is in cache
	foundOwner, isInCache := ownerCache[podName]
	if isInCache {
		return utils.Pointer(foundOwner)
	}

	pod := GetPod(namespace, podName)
	if pod == nil {
		k8sLogger.Error("Pod not found.", "namespace", namespace, "pod", podName)
		return nil
	}
	ctlr := OwnerFromReference(pod.Namespace, pod.OwnerReferences)
	if ctlr != nil {
		dataLock.Lock()
		ownerCache[pod.Name] = *ctlr
		dataLock.Unlock()
		return ctlr
	}

	k8sLogger.Debug("Pod has no owner.", "namespace", namespace, "pod", podName)
	return nil
}

func OwnerFromReference(namespace string, ownerRefs []metav1.OwnerReference) *K8sController {
	var lastValidController *K8sController

	if len(ownerRefs) > 0 {
		owner := ownerRefs[0]
		switch owner.Kind {
		case "ReplicaSet":
			data, err := GetReplicaset(namespace, owner.Name)
			if err == nil && data != nil {
				lastValidController = utils.Pointer(NewK8sController("ReplicaSet", data.Name, namespace))
				if data.OwnerReferences != nil {
					// recurse and update lastValidController if successful
					return returnOrUpdated(lastValidController,
						OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Deployment":
			data, err := GetK8sDeployment(namespace, owner.Name)
			if err == nil && data != nil {
				lastValidController = utils.Pointer(NewK8sController("Deployment", data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "StatefulSet":
			data, err := GetStatefulSet(namespace, owner.Name)
			if err == nil && data != nil {
				lastValidController = utils.Pointer(NewK8sController("StatefulSet", data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "DaemonSet":
			data, err := GetK8sDaemonset(namespace, owner.Name)
			if err == nil && data != nil {
				lastValidController = utils.Pointer(NewK8sController("DaemonSet", data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Job":
			data, err := GetJob(namespace, owner.Name)
			if err == nil && data != nil {
				lastValidController = utils.Pointer(NewK8sController("Job", data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "CronJob":
			data, err := GetCronJob(namespace, owner.Name)
			if err == nil && data != nil {
				lastValidController = utils.Pointer(NewK8sController("CronJob", data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Pod":
			data := GetPod(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController("Pod", data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Node":
			data, err := GetK8sNode(owner.Name)
			if err == nil && data != nil {
				lastValidController = utils.Pointer(NewK8sController("Node", data.Name, ""))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		default:
			if lastValidController == nil {
				k8sLogger.Error("NOT IMPLEMENTED owner kind", "owner kind", owner.Kind)
			}
		}
	}

	return lastValidController
}

func returnOrUpdated(lastValid *K8sController, result *K8sController) *K8sController {
	if result != nil {
		return result
	}
	return lastValid
}
