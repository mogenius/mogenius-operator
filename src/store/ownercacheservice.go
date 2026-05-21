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

var ownerCache sync.Map

// podCacheKey builds the cache key for a pod's owner lookup. Keying by
// pod name alone (the previous behavior) silently returned the wrong
// controller for two pods with the same name in different namespaces -
// a routine occurrence for system pods like metrics-server-xxx or
// when the same Helm release is installed into multiple namespaces.
func podCacheKey(namespace, podName string) string {
	return namespace + "/" + podName
}

// ClearOwnerCachePodEntry removes a pod's cached owner entry.
// Can be called from anywhere without holding an OwnerCacheService reference.
func ClearOwnerCachePodEntry(namespace, podName string) {
	ownerCache.Delete(podCacheKey(namespace, podName))
}

func (self *ownerCacheService) ControllerForPod(namespace string, podName string) *utils.WorkloadSingleRequest {
	cacheKey := podCacheKey(namespace, podName)
	if cached, ok := ownerCache.Load(cacheKey); ok {
		ctlr, ok := cached.(utils.WorkloadSingleRequest)
		if !ok {
			return nil
		}
		return &ctlr
	}

	pod := GetPod(namespace, podName)
	if pod == nil {
		self.logger.Debug("Pod not found.", "namespace", namespace, "pod", podName)
		return nil
	}
	ctlr := self.OwnerFromReference(pod.Namespace, pod.OwnerReferences)
	if ctlr != nil {
		ownerCache.Store(cacheKey, *ctlr)
		return ctlr
	}

	// Special case for pods with no owner (often used by system pods)
	if pod.OwnerReferences == nil {
		return new(NewK8sController(utils.PodResource, pod.Name, namespace))
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
				lastValidController = new(NewK8sController(utils.ReplicaSetResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					// recurse and update lastValidController if successful
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Deployment":
			data := GetDeployment(namespace, owner.Name)
			if data != nil {
				lastValidController = new(NewK8sController(utils.DeploymentResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "StatefulSet":
			data := GetStatefulSet(namespace, owner.Name)
			if data != nil {
				lastValidController = new(NewK8sController(utils.StatefulSetResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "DaemonSet":
			data := GetDaemonSet(namespace, owner.Name)
			if data != nil {
				lastValidController = new(NewK8sController(utils.DaemonSetResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Job":
			data := GetJob(namespace, owner.Name)
			if data != nil {
				lastValidController = new(NewK8sController(utils.JobResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "CronJob":
			data := GetCronJob(namespace, owner.Name)
			if data != nil {
				lastValidController = new(NewK8sController(utils.CronJobResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Pod":
			data := GetPod(namespace, owner.Name)
			if data != nil {
				lastValidController = new(NewK8sController(utils.PodResource, data.Name, namespace))
				if data.OwnerReferences != nil {
					return returnOrUpdated(lastValidController,
						self.OwnerFromReference(namespace, data.OwnerReferences))
				}
			}

		case "Node":
			data := GetNode(owner.Name)
			if data != nil {
				lastValidController = new(NewK8sController(utils.NodeResource, data.Name, ""))
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
