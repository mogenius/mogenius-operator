package kubernetes

import (
	"mogenius-k8s-manager/logger"

	punq "github.com/mogenius/punq/kubernetes"
	"github.com/mogenius/punq/utils"
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
	return owner.Kind + "-" + owner.Namespace + "-" + owner.Name
}

var OwnerCache = make(map[string]K8sController)

func ControllerForPod(namespace string, podName string) *K8sController {
	// check if is in cache
	foundOwner, isInCache := OwnerCache[podName]
	if isInCache {
		return utils.Pointer(foundOwner)
	}

	pod := punq.GetPod(namespace, podName, nil)
	if pod == nil {
		logger.Log.Errorf("Pod: '%s/%s' not found.", namespace, podName)
		return nil
	}
	ctlr := OwnerFromReference(pod.Namespace, pod.OwnerReferences)
	if ctlr != nil {
		OwnerCache[pod.Name] = *ctlr
		return ctlr
	}

	logger.Log.Errorf("Pod: '%s/%s' has no owner.", namespace, podName)
	return nil
}

func OwnerFromReference(namespace string, ownerRefs []metav1.OwnerReference) *K8sController {
	if len(ownerRefs) > 0 {
		owner := ownerRefs[0]

		switch owner.Kind {
		case "ReplicaSet":
			data, err := punq.GetReplicaset(namespace, owner.Name, nil)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					return utils.Pointer(NewK8sController("ReplicaSet", data.Name, namespace))
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "Deployment":
			data, err := punq.GetK8sDeployment(namespace, owner.Name, nil)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					return utils.Pointer(NewK8sController("Deployment", data.Name, namespace))
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "StatefulSet":
			data, err := punq.GetStatefulSet(namespace, owner.Name, nil)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					return utils.Pointer(NewK8sController("StatefulSet", data.Name, namespace))
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "DaemonSet":
			data, err := punq.GetK8sDaemonset(namespace, owner.Name, nil)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					return utils.Pointer(NewK8sController("DaemonSet", data.Name, namespace))
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "Job":
			data, err := punq.GetJob(namespace, owner.Name, nil)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					return utils.Pointer(NewK8sController("Job", data.Name, namespace))
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "CronJob":
			data, err := punq.GetCronjob(namespace, owner.Name, nil)
			if err == nil && data != nil {
				if data.OwnerReferences == nil {
					return utils.Pointer(NewK8sController("CronJob", data.Name, namespace))
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		case "Pod":
			data := punq.GetPod(namespace, owner.Name, nil)
			if data != nil {
				if data.OwnerReferences == nil {
					return utils.Pointer(NewK8sController("Pod", data.Name, namespace))
				} else {
					// recurse
					return OwnerFromReference(namespace, data.OwnerReferences)
				}
			}
			return nil
		default:
			logger.Log.Errorf("NOT IMPLEMENTED owner kind: %s", owner.Kind)
			return nil
		}
	}
	return nil
}
