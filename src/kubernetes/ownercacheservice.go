package kubernetes

import (
	"mogenius-k8s-manager/src/utils"

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

var OwnerCache = make(map[string]K8sController)

func ControllerForPod(namespace string, podName string) *K8sController {
	// check if is in cache
	foundOwner, isInCache := OwnerCache[podName]
	if isInCache {
		return utils.Pointer(foundOwner)
	}

	pod := GetPod(namespace, podName)
	if pod == nil {
		k8sLogger.Error("Pod not found.", "namespace", namespace, "podname", podName)
		return nil
	}
	ctlr := OwnerFromReference(pod.Namespace, pod.OwnerReferences)
	if ctlr != nil {
		OwnerCache[pod.Name] = *ctlr
		return ctlr
	}

	k8sLogger.Debug("Pod has no owner.", "namespace", namespace, "podname", podName)
	return nil
}

func OwnerFromReference(namespace string, ownerRefs []metav1.OwnerReference) *K8sController {
	if len(ownerRefs) > 0 {
		var lastValidController *K8sController
		owner := ownerRefs[0]

		switch owner.Kind {
		case "ReplicaSet":
			data, err := GetReplicaset(namespace, owner.Name)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					lastValidController = utils.Pointer(NewK8sController("ReplicaSet", data.Name, namespace))
					return lastValidController
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "Deployment":
			data, err := GetK8sDeployment(namespace, owner.Name)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					lastValidController = utils.Pointer(NewK8sController("Deployment", data.Name, namespace))
					return lastValidController
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "StatefulSet":
			data, err := GetStatefulSet(namespace, owner.Name)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					lastValidController = utils.Pointer(NewK8sController("StatefulSet", data.Name, namespace))
					return lastValidController
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "DaemonSet":
			data, err := GetK8sDaemonset(namespace, owner.Name)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					lastValidController = utils.Pointer(NewK8sController("DaemonSet", data.Name, namespace))
					return lastValidController
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "Job":
			data, err := GetJob(namespace, owner.Name)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					lastValidController = utils.Pointer(NewK8sController("Job", data.Name, namespace))
					return lastValidController
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "CronJob":
			data, err := GetCronJob(namespace, owner.Name)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					lastValidController = utils.Pointer(NewK8sController("CronJob", data.Name, namespace))
					return lastValidController
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "Pod":
			data := GetPod(namespace, owner.Name)
			if data != nil {
				if data.OwnerReferences == nil {
					lastValidController = utils.Pointer(NewK8sController("Pod", data.Name, namespace))
					return lastValidController
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "Node":
			data, err := GetK8sNode(owner.Name)
			if err != nil {
				k8sLogger.Error("Error getting node", "error", err)
				return nil
			}
			if data != nil {
				if data.OwnerReferences == nil {
					lastValidController = utils.Pointer(NewK8sController("Node", data.Name, ""))
					return lastValidController
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		default:
			if lastValidController == nil {
				k8sLogger.Error("UNKNOWN owner kind", "owner kind", owner.Kind)
				return nil
			}
			return lastValidController
		}
	}
	return nil
}
