package kubernetes

import (
	"mogenius-k8s-manager/logger"

	punq "github.com/mogenius/punq/kubernetes"
	"github.com/mogenius/punq/utils"
)

type K8sController struct {
	Kind      string
	Name      string
	NameSpace string
}

func NewK8sController(kind string, name string, namespace string) K8sController {
	return K8sController{
		Kind:      kind,
		Name:      name,
		NameSpace: namespace,
	}
}

func (owner *K8sController) Identifier() string {
	return owner.Kind + "-" + owner.NameSpace + "-" + owner.Name
}

var OwnerCache = make(map[string]K8sController)

func ControllerForPod(namespace string, podName string) *K8sController {
	// check if is in cache
	foundOwner, isInCache := OwnerCache[podName]
	if isInCache {
		return utils.Pointer(foundOwner)
	}

	pod := punq.GetPod(namespace, podName, nil)

	if pod.OwnerReferences == nil && len(pod.OwnerReferences) > 0 {
		owner := pod.OwnerReferences[0]

		switch owner.Kind {
		case "ReplicaSet":
			data, err := punq.GetReplicaset(owner.Name, pod.Namespace, nil)
			if err == nil && data != nil {
				if pod.OwnerReferences == nil && len(pod.OwnerReferences) > 0 {
					OwnerCache[pod.Name] = NewK8sController(data.Kind, data.Name, pod.Namespace)
					return utils.Pointer(OwnerCache[pod.Name])
				}
			}
			return nil
		case "Deployment":
			data, err := punq.GetK8sDeployment(owner.Name, pod.Namespace, nil)
			if err == nil && data != nil {
				if pod.OwnerReferences == nil && len(pod.OwnerReferences) > 0 {
					OwnerCache[pod.Name] = NewK8sController(data.Kind, data.Name, pod.Namespace)
					return utils.Pointer(OwnerCache[pod.Name])
				}
			}
			return nil
		case "StatefulSet":
			data, err := punq.GetStatefulSet(owner.Name, pod.Namespace, nil)
			if err == nil && data != nil {
				if pod.OwnerReferences == nil && len(pod.OwnerReferences) > 0 {
					OwnerCache[pod.Name] = NewK8sController(data.Kind, data.Name, pod.Namespace)
					return utils.Pointer(OwnerCache[pod.Name])
				}
			}
			return nil
		case "DaemonSet":
			data, err := punq.GetK8sDaemonset(owner.Name, pod.Namespace, nil)
			if err == nil && data != nil {
				if pod.OwnerReferences == nil && len(pod.OwnerReferences) > 0 {
					OwnerCache[pod.Name] = NewK8sController(data.Kind, data.Name, pod.Namespace)
					return utils.Pointer(OwnerCache[pod.Name])
				}
			}
			return nil
		case "Job":
			data, err := punq.GetJob(owner.Name, pod.Namespace, nil)
			if err == nil && data != nil {
				if pod.OwnerReferences == nil && len(pod.OwnerReferences) > 0 {
					OwnerCache[pod.Name] = NewK8sController(data.Kind, data.Name, pod.Namespace)
					return utils.Pointer(OwnerCache[pod.Name])
				}
			}
			return nil
		case "CronJob":
			data, err := punq.GetCronjob(owner.Name, pod.Namespace, nil)
			if err == nil && data != nil {
				if pod.OwnerReferences == nil && len(pod.OwnerReferences) > 0 {
					OwnerCache[pod.Name] = NewK8sController(data.Kind, data.Name, pod.Namespace)
					return utils.Pointer(OwnerCache[pod.Name])
				}
			}
			return nil
		case "Pod":
			data := punq.GetPod(pod.Namespace, owner.Name, nil)
			if data != nil {
				if pod.OwnerReferences == nil && len(pod.OwnerReferences) > 0 {
					OwnerCache[pod.Name] = NewK8sController(data.Kind, data.Name, pod.Namespace)
					return utils.Pointer(OwnerCache[pod.Name])
				}
			}
			return nil
		default:
			logger.Log.Errorf("NOT IMPLEMENTED owner kind: %s", owner.Kind)
			return nil
		}
	}
	logger.Log.Errorf("Pod: '%s' has no owner.", pod.Name)
	return nil
}
