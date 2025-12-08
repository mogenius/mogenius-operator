package store

import (
	"log/slog"
	"mogenius-operator/src/config"
	"mogenius-operator/src/utils"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewK8sController(resource utils.ResourceDescriptor, name string, namespace string) utils.WorkloadSingleRequest {
	return utils.WorkloadSingleRequest{
		ResourceDescriptor: resource,
		Namespace:          namespace,
		ResourceName:       name,
	}
}

type OwnerCacheService interface {
	ControllerForPod(namespace string, podName string) *utils.WorkloadSingleRequest
	OwnerFromReference(namespace string, ownerRefs []metav1.OwnerReference) *utils.WorkloadSingleRequest
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

var ownerCache = make(map[string]utils.WorkloadSingleRequest)

var dataLock sync.Mutex = sync.Mutex{}

func (self *ownerCacheService) ControllerForPod(namespace string, podName string) *utils.WorkloadSingleRequest {
	// check if is in cache
	foundOwner, isInCache := ownerCache[podName]
	if isInCache {
		return utils.Pointer(foundOwner)
	}

	pod := GetPod(namespace, podName)
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
		return utils.Pointer(NewK8sController(utils.PodResource, pod.Name, namespace))
	}

	self.logger.Debug("Pod has no owner.", "namespace", namespace, "pod", podName)
	return nil
}

func (self *ownerCacheService) OwnerFromReference(namespace string, ownerRefs []metav1.OwnerReference) *utils.WorkloadSingleRequest {
	var lastValidController *utils.WorkloadSingleRequest

	if len(ownerRefs) > 0 {
		owner := ownerRefs[0]
		switch owner.Kind {
		case "ReplicaSet":
			data := GetReplicaset(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.ReplicaSetResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					// recurse and update lastValidController if successful
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Deployment":
			data := GetDeployment(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.DeploymentResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "StatefulSet":
			data := GetStatefulSet(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.StatefulSetResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "DaemonSet":
			data := GetDaemonSet(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.DaemonSetResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Job":
			data := GetJob(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.JobResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "CronJob":
			data := GetCronJob(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.CronJobResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Pod":
			data := GetPod(namespace, owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.PodResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Node":
			data := GetNode(owner.Name)
			if data != nil {
				lastValidController = utils.Pointer(NewK8sController(utils.NodeResource, data.Name, ""))
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

func returnOrUpdated(lastValid *utils.WorkloadSingleRequest, result *utils.WorkloadSingleRequest) *utils.WorkloadSingleRequest {
	if result != nil {
		return result
	}
	return lastValid
}
