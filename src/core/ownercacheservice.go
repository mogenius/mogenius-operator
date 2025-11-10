package core

import (
	"log/slog"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewK8sController(kind string, name string, namespace string) K8sController {
	return K8sController{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}
}

type K8sController struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type OwnerCacheService interface {
	ControllerForPod(namespace string, podName string) *K8sController
	OwnerFromReference(namespace string, ownerRefs []metav1.OwnerReference) *K8sController
}

type ownerCacheService struct {
	logger *slog.Logger
	config config.ConfigModule
}

func NewOwnerCacheService(
	logger *slog.Logger,
	config config.ConfigModule,
) OwnerCacheService {
	self := &ownerCacheService{}

	self.logger = logger
	self.config = config

	return self
}

var ownerCache = make(map[string]K8sController)

var dataLock sync.Mutex = sync.Mutex{}

func (self *ownerCacheService) ControllerForPod(namespace string, podName string) *K8sController {
	// check if is in cache
	foundOwner, isInCache := ownerCache[podName]
	if isInCache {
		return utils.Pointer(foundOwner)
	}

	pod := store.GetPod(namespace, podName)
	if pod == nil {
		self.logger.Debug("Pod not found.", "namespace", namespace, "pod", podName)
		return nil
	}
	ctlr := self.OwnerFromReference(pod.Namespace, pod.OwnerReferences)
	if ctlr != nil {
		dataLock.Lock()
		ownerCache[pod.Name] = *ctlr
		dataLock.Unlock()
		return ctlr
	}

	// Special case for pods with no owner (often used by system pods)
	if pod.OwnerReferences == nil {
		return utils.Pointer(NewK8sController("Pod", pod.Name, namespace))
	}

	self.logger.Debug("Pod has no owner.", "namespace", namespace, "pod", podName)
	return nil
}

func (self *ownerCacheService) OwnerFromReference(namespace string, ownerRefs []metav1.OwnerReference) *K8sController {
	var lastValidController *K8sController

	if len(ownerRefs) > 0 {
		owner := ownerRefs[0]
		switch owner.Kind {
		case "ReplicaSet":
			data := store.GetReplicaset(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.ReplicaSetResource.Kind, data.Name, namespace))
				if data.OwnerReferences != nil {
					// recurse and update lastValidController if successful
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Deployment":
			data := store.GetDeployment(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.DeploymentResource.Kind, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "StatefulSet":
			data := store.GetStatefulSet(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.StatefulSetResource.Kind, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "DaemonSet":
			data := store.GetDaemonSet(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.DaemonSetResource.Kind, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Job":
			data := store.GetJob(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.JobResource.Kind, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "CronJob":
			data := store.GetCronJob(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.CronJobResource.Kind, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Pod":
			data := store.GetPod(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.PodResource.Kind, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Node":
			data := store.GetNode(owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.NodeResource.Kind, data.Name, ""))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		default:
			self.logger.Debug("NOT IMPLEMENTED owner kind", "owner kind", owner.Kind)
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
