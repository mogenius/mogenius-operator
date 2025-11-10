package core

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	v1metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/yaml"
)

type CleanUpResult struct {
	Pods        []CleanUpResultEntry `json:"pods"`
	ReplicaSets []CleanUpResultEntry `json:"replicaSets"`
	Services    []CleanUpResultEntry `json:"services"`
	Secrets     []CleanUpResultEntry `json:"secrets"`
	ConfigMaps  []CleanUpResultEntry `json:"configMaps"`
	Jobs        []CleanUpResultEntry `json:"jobs"`
	Ingresses   []CleanUpResultEntry `json:"ingresses"`
}
type CleanUpResultEntry struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Reason    string `json:"reason"`
}
type MoKubernetes interface {
	Run()
	Link(valkeyStatsDb ValkeyStatsDb)
	CreateOrUpdateClusterSecret() (utils.ClusterSecret, error)
	CreateOrUpdateResourceTemplateConfigmap() error
	GetNodeStats() ([]dtos.NodeStat, error)
	CleanUp(apiService Api, workspaceName string, dryRun bool, replicaSets bool, pods bool, services bool, secrets bool, configMaps bool, jobs bool, ingresses bool) (CleanUpResult, error)
}

type moKubernetes struct {
	logger         *slog.Logger
	config         config.ConfigModule
	clientProvider k8sclient.K8sClientProvider

	valkeyStatsDb ValkeyStatsDb
}

func NewMoKubernetes(
	logger *slog.Logger,
	configModule config.ConfigModule,
	clientProviderModule k8sclient.K8sClientProvider,
) MoKubernetes {
	self := &moKubernetes{}

	self.logger = logger
	self.config = configModule
	self.clientProvider = clientProviderModule

	return self
}

func (self *moKubernetes) Run() {}

func (self *moKubernetes) Link(valkeyStatsDb ValkeyStatsDb) {
	assert.Assert(valkeyStatsDb != nil)

	self.valkeyStatsDb = valkeyStatsDb
}

func (self *moKubernetes) CreateOrUpdateClusterSecret() (utils.ClusterSecret, error) {
	clientset := self.clientProvider.K8sClientSet()
	secretClient := clientset.CoreV1().Secrets(self.config.Get("MO_OWN_NAMESPACE"))

	existingSecret, getErr := secretClient.Get(context.Background(), self.config.Get("MO_OWN_NAMESPACE"), metav1.GetOptions{})
	return self.writeMogeniusSecret(secretClient, existingSecret, getErr)
}

func (self *moKubernetes) writeMogeniusSecret(secretClient v1.SecretInterface, existingSecret *corev1.Secret, getErr error) (utils.ClusterSecret, error) {
	// CREATE NEW SECRET
	apikey := self.config.Get("MO_API_KEY")
	clusterName := self.config.Get("MO_CLUSTER_NAME")

	// Construct cluster secret object
	clusterSecret := utils.ClusterSecret{
		ApiKey:      apikey,
		ClusterName: clusterName,
	}
	if existingSecret != nil {
		if string(existingSecret.Data["cluster-mfa-id"]) != "" {
			clusterSecret.ClusterMfaId = string(existingSecret.Data["cluster-mfa-id"])
		}
		if string(existingSecret.Data["redis-data-model-version"]) == "" {
			clusterSecret.RedisDataModelVersion = "0"
		} else {
			clusterSecret.RedisDataModelVersion = "1"
		}
	}
	if clusterSecret.ClusterMfaId == "" {
		clusterSecret.ClusterMfaId = utils.NanoId()
	}

	secret := utils.InitSecret()
	secret.ObjectMeta.Name = self.config.Get("MO_OWN_NAMESPACE")
	secret.ObjectMeta.Namespace = self.config.Get("MO_OWN_NAMESPACE")
	delete(secret.StringData, "exampleData") // delete example data
	secret.StringData["cluster-mfa-id"] = clusterSecret.ClusterMfaId
	secret.StringData["api-key"] = clusterSecret.ApiKey
	secret.StringData["cluster-name"] = clusterSecret.ClusterName
	secret.StringData["redis-data-model-version"] = clusterSecret.RedisDataModelVersion

	if existingSecret == nil || getErr != nil {
		self.logger.Info("🔑 Creating new mogenius secret ...")
		result, err := secretClient.Create(context.Background(), &secret, self.createOptions())
		if err != nil {
			self.logger.Error("Error creating mogenius secret.", "error", err)
			return clusterSecret, err
		}
		self.logger.Info("🔑 Created new mogenius secret", result.GetObjectMeta().GetName(), ".")
	} else {
		if string(existingSecret.Data["cluster-mfa-id"]) != clusterSecret.ClusterMfaId ||
			string(existingSecret.Data["api-key"]) != clusterSecret.ApiKey ||
			string(existingSecret.Data["cluster-name"]) != clusterSecret.ClusterName ||
			string(existingSecret.Data["redis-data-model-version"]) != clusterSecret.RedisDataModelVersion {
			self.logger.Info("🔑 Updating existing mogenius secret ...")
			result, err := secretClient.Update(context.Background(), &secret, self.updateOptions())
			if err != nil {
				self.logger.Error("Error updating mogenius secret.", "error", err)
				return clusterSecret, err
			}
			self.logger.Info("🔑 Updated mogenius secret", result.GetObjectMeta().GetName(), ".")
		} else {
			self.logger.Info("🔑 Using existing mogenius secret.")
		}
	}

	return clusterSecret, nil
}

func (self *moKubernetes) getDeploymentName() string {
	return self.config.Get("OWN_DEPLOYMENT_NAME")
}

func (self *moKubernetes) createOptions() metav1.CreateOptions {
	return metav1.CreateOptions{
		FieldManager: self.getDeploymentName(),
	}
}

func (self *moKubernetes) updateOptions() metav1.UpdateOptions {
	return metav1.UpdateOptions{
		FieldManager: self.getDeploymentName(),
	}
}

func (self *moKubernetes) getResourceTemplateConfigMap() string {
	return "mogenius-resource-templates"
}

func (self *moKubernetes) CreateOrUpdateResourceTemplateConfigmap() error {
	yamlData := utils.InitResourceTemplatesYaml()

	// Decode YAML data into a generic map
	var decodedData map[string]any
	err := yaml.Unmarshal([]byte(yamlData), &decodedData)
	if err != nil {
		return err
	}

	cfgMap := unstructured.Unstructured{Object: decodedData}
	cfgMap.SetNamespace(self.config.Get("MO_OWN_NAMESPACE"))
	cfgMap.SetName(self.getResourceTemplateConfigMap())

	// Marshal cfgMap back to YAML
	updatedYaml, err := yaml.Marshal(cfgMap.Object)
	if err != nil {
		return err
	}

	// check if configmap exists
	ownNamespace := self.config.Get("MO_OWN_NAMESPACE")
	_, err = self.CreateUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, utils.Pointer(ownNamespace), string(updatedYaml))
	if apierrors.IsAlreadyExists(err) {
		_, err = kubernetes.UpdateUnstructuredResource(utils.ConfigMapResource.ApiVersion, utils.ConfigMapResource.Plural, utils.ConfigMapResource.Namespaced, string(updatedYaml))
		if err != nil {
			self.logger.Error("Resource template configmap failed to update", "error", err)
			return err
		}
		self.logger.Info("Resource template configmap updated")
		return nil
	}

	return err
}

func (self *moKubernetes) CreateUnstructuredResource(apiVersion string, plural string, namespace *string, yamlData string) (*unstructured.Unstructured, error) {
	dynamicClient := self.clientProvider.DynamicClient()
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespace != nil {
		result, err := dynamicClient.Resource(kubernetes.CreateGroupVersionResource(apiVersion, plural)).Namespace(obj.GetNamespace()).Create(context.Background(), obj, metav1.CreateOptions{})
		return self.removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(kubernetes.CreateGroupVersionResource(apiVersion, plural)).Create(context.Background(), obj, metav1.CreateOptions{})
		return self.removeManagedFields(result), err
	}
}

func (self *moKubernetes) removeManagedFields(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil {
		return obj
	}

	unstructuredContent := obj.Object
	delete(unstructuredContent, "managedFields")
	if unstructuredContent["metadata"] != nil {
		delete(unstructuredContent["metadata"].(map[string]any), "managedFields")
	}

	return obj
}

func (self *moKubernetes) GetNodeStats() ([]dtos.NodeStat, error) {
	result := []dtos.NodeStat{}
	nodes := store.GetNodes()
	nodeMetrics := kubernetes.ListNodeMetricss()

	if len(nodeMetrics) == 0 {
		self.logger.Error("CRITICAL: No node metrics found. Make sure the metrics-server is installed and running.")
		return result, fmt.Errorf("no metrics-server found")
	}

	for _, node := range nodes {
		for _, taint := range node.Spec.Taints {
			if taint.Effect == corev1.TaintEffectNoSchedule || taint.Key == "CriticalAddonsOnly" {
				continue // Skip nodes with NoSchedule/CriticalAddonsOnly taints
			}
		}
		allPods := kubernetes.AllPodsOnNode(node.Name)
		requestCpuCores, limitCpuCores := kubernetes.SumCpuResources(allPods)
		requestMemoryBytes, limitMemoryBytes := kubernetes.SumMemoryResources(allPods)

		utilizedCores := float64(0)
		utilizedMemory := int64(0)
		if len(nodeMetrics) > 0 {
			// Find the corresponding node metrics
			var nodeMetric *v1metrics.NodeMetrics
			for _, nm := range nodeMetrics {
				if nm.Name == node.Name {
					nodeMetric = &nm
					break
				}
			}
			if nodeMetric == nil {
				self.logger.Error("Failed to find node metrics for node", "node.name", node.Name)
				continue
			}

			// CPU
			cpuUsage, works := nodeMetric.Usage.Cpu().AsDec().Unscaled()
			if !works {
				self.logger.Error("Failed to get CPU usage for node", "node.name", node.Name)
			}
			if cpuUsage == 0 {
				cpuUsage = 1
			}
			utilizedCores = float64(cpuUsage) / 1000000000

			// Memory
			utilizedMemory, works = nodeMetric.Usage.Memory().AsInt64()
			if !works {
				self.logger.Error("Failed to get MEMORY usage for node", "node.name", node.Name)
			}
		}

		mem, _ := node.Status.Capacity.Memory().AsInt64()
		cpu, _ := node.Status.Capacity.Cpu().AsInt64()
		maxPods, _ := node.Status.Capacity.Pods().AsInt64()
		ephemeral, _ := node.Status.Capacity.StorageEphemeral().AsInt64()

		machineStats, err := self.valkeyStatsDb.GetMachineStatsForNode(node.Name)
		if err != nil {
			machineStats = nil
			self.logger.Warn("failed to get machines stats for node", "node", node.Name, "error", err)
		}

		nodeStat := dtos.NodeStat{
			Name:                   node.Name,
			MaschineId:             node.Status.NodeInfo.MachineID,
			CpuInCores:             cpu,
			CpuInCoresUtilized:     utilizedCores,
			CpuInCoresRequested:    requestCpuCores,
			CpuInCoresLimited:      limitCpuCores,
			MachineStats:           machineStats,
			MemoryInBytes:          mem,
			MemoryInBytesUtilized:  utilizedMemory,
			MemoryInBytesRequested: requestMemoryBytes,
			MemoryInBytesLimited:   limitMemoryBytes,
			EphemeralInBytes:       ephemeral,
			MaxPods:                maxPods,
			TotalPods:              int64(len(allPods)),
			KubletVersion:          node.Status.NodeInfo.KubeletVersion,
			OsType:                 node.Status.NodeInfo.OperatingSystem,
			OsImage:                node.Status.NodeInfo.OSImage,
			OsKernelVersion:        node.Status.NodeInfo.KernelVersion,
			Architecture:           node.Status.NodeInfo.Architecture,
		}
		result = append(result, nodeStat)
	}

	return result, nil
}

func (self *moKubernetes) CleanUp(apiService Api, workspaceName string, dryRun bool, replicaSets bool, pods bool, services bool, secrets bool, configMaps bool, jobs bool, ingresses bool) (CleanUpResult, error) {
	result := CleanUpResult{}

	entries, err := apiService.GetWorkspaceResources(workspaceName, nil, nil, nil)
	if err != nil {
		self.logger.Error("failed to get workspace resources", "error", err)
		return result, err
	}

	workspacePods := []corev1.Pod{}
	workspaceIngresses := []netv1.Ingress{}
	workSpaceServices := []corev1.Service{}

	// Filter pods,ingresses,services in workspace first because we need them for later checks
	for _, entry := range entries {
		// PODS
		if entry.GetKind() == "Pod" && pods {
			var pod corev1.Pod
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(entry.UnstructuredContent(), &pod)
			if err != nil {
				continue
			}
			workspacePods = append(workspacePods, pod)
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodUnknown {
				result.Pods = append(result.Pods, createCURE(pod.Name, pod.Namespace, fmt.Sprintf("pod is in %s (%s) state", pod.Status.Phase, pod.Status.Reason)))
				if !dryRun {
					resName, err := kubernetes.GetResourcesNameForKind(entry.GetKind())
					if err != nil {
						continue
					}
					err = kubernetes.DeleteResource(entry.GroupVersionKind().Group, entry.GroupVersionKind().Version, resName, entry.GetName(), entry.GetNamespace(), false)
					if err != nil {
						self.logger.Error("failed to delete pod", "pod", pod.Name, "error", err)
					}
				}
			}
			continue
		}

		// Ingresses
		if entry.GetKind() == "Ingress" {
			var ingress netv1.Ingress
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(entry.UnstructuredContent(), &ingress)
			if err != nil {
				continue
			}
			workspaceIngresses = append(workspaceIngresses, ingress)
			continue
		}

		// Services
		if entry.GetKind() == "Service" {
			var service corev1.Service
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(entry.UnstructuredContent(), &service)
			if err != nil {
				continue
			}
			workSpaceServices = append(workSpaceServices, service)
			continue
		}
	}

	for _, entry := range entries {
		// REPLICASETS
		if entry.GetKind() == "ReplicaSet" && replicaSets {
			var replicaSet appsv1.ReplicaSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(entry.UnstructuredContent(), &replicaSet)
			if err != nil {
				continue
			}
			if replicaSet.Status.Replicas == 0 && int(*replicaSet.Spec.Replicas) == 0 {
				result.ReplicaSets = append(result.ReplicaSets, createCURE(replicaSet.Name, replicaSet.Namespace, "replicaset unused. (replicas == 0 and status.replicas == 0)"))
				if !dryRun {
					resName, err := kubernetes.GetResourcesNameForKind(entry.GetKind())
					if err != nil {
						continue
					}
					err = kubernetes.DeleteResource(entry.GroupVersionKind().Group, entry.GroupVersionKind().Version, resName, entry.GetName(), entry.GetNamespace(), false)
					if err != nil {
						self.logger.Error("failed to delete replicaset", "replicaset", replicaSet.Name, "error", err)
					}
				}
			}
			continue
		}

		// SERVICES
		if entry.GetKind() == "Service" && services {
			var service corev1.Service
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(entry.UnstructuredContent(), &service)
			if err != nil {
				continue
			}

			// Determine if service is orphaned
			if len(service.Spec.Selector) == 0 {
				continue
			}
			matchingPods := 0
			for _, pod := range workspacePods {
				if podMatchesSelector(&pod, service.Spec.Selector) {
					matchingPods++
					break
				}
			}
			// Check if service is not referenced by any ingress
			ingressExists := false
			for _, ingress := range workspaceIngresses {
				if ingress.Namespace != service.Namespace {
					continue // Only match ingresses in the same namespace
				}
				for _, rule := range ingress.Spec.Rules {
					if rule.HTTP != nil {
						for _, path := range rule.HTTP.Paths {
							if path.Backend.Service != nil && path.Backend.Service.Name == service.Name {
								ingressExists = true
								break
							}
						}
					}
					if ingressExists {
						break
					}
				}
				if ingressExists {
					break
				}
			}
			if matchingPods == 0 && !ingressExists {
				result.Services = append(result.Services, createCURE(service.Name, service.Namespace, "service not used by any running pod or ingress"))
				if !dryRun {
					resName, err := kubernetes.GetResourcesNameForKind(entry.GetKind())
					if err != nil {
						continue
					}
					err = kubernetes.DeleteResource(entry.GroupVersionKind().Group, entry.GroupVersionKind().Version, resName, entry.GetName(), entry.GetNamespace(), false)
					if err != nil {
						self.logger.Error("failed to delete service", "service", service.Name, "error", err)
					}
				}
			}
			continue
		}

		// SECRETS
		if entry.GetKind() == "Secret" && secrets {
			var secret corev1.Secret
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(entry.UnstructuredContent(), &secret)
			if err != nil {
				continue
			}
			isInUse := false
			for _, pod := range workspacePods {
				// check if configmap is used by any pod
				if pod.Spec.Volumes != nil {
					for _, volume := range pod.Spec.Volumes {
						if volume.ConfigMap != nil && volume.ConfigMap.Name == secret.Name {
							isInUse = true
							break
						}
					}
				}
				if pod.Spec.Containers != nil {
					for _, container := range pod.Spec.Containers {
						if container.VolumeMounts != nil {
							for _, volumeMount := range container.VolumeMounts {
								if volumeMount.Name == secret.Name {
									isInUse = true
									break
								}
							}
						}
						for _, env := range container.Env {
							if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == secret.Name {
								isInUse = true
								break
							}
						}
					}
				}
				if pod.Spec.InitContainers != nil {
					for _, initContainer := range pod.Spec.InitContainers {
						if initContainer.VolumeMounts != nil {
							for _, volumeMount := range initContainer.VolumeMounts {
								if volumeMount.Name == secret.Name {
									isInUse = true
									break
								}
							}
						}
					}
				}
				if pod.Spec.ImagePullSecrets != nil {
					for _, imagePullSecret := range pod.Spec.ImagePullSecrets {
						if imagePullSecret.Name == secret.Name {
							isInUse = true
							break
						}
					}
				}
			}
			for _, ingress := range workspaceIngresses {
				for _, tls := range ingress.Spec.TLS {
					if tls.SecretName == secret.Name {
						isInUse = true
						break
					}
				}
			}
			if !isInUse {
				result.Secrets = append(result.Secrets, createCURE(secret.Name, secret.Namespace, "secret is not used by any pod or ingress"))
				if !dryRun {
					resName, err := kubernetes.GetResourcesNameForKind(entry.GetKind())
					if err != nil {
						continue
					}
					err = kubernetes.DeleteResource(entry.GroupVersionKind().Group, entry.GroupVersionKind().Version, resName, entry.GetName(), entry.GetNamespace(), false)
					if err != nil {
						self.logger.Error("failed to delete secret", "secret", secret.Name, "error", err)
					}
				}
			}
			continue
		}

		// CONFIGMAPS
		if entry.GetKind() == "ConfigMap" && configMaps {
			var configMap corev1.ConfigMap
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(entry.UnstructuredContent(), &configMap)
			if err != nil {
				continue
			}
			isInUse := false
			if configMap.Name == "kube-root-ca.crt" {
				isInUse = true
			} else {
				for _, pod := range workspacePods {
					// check if configmap is used by any pod
					if pod.Spec.Volumes != nil {
						for _, volume := range pod.Spec.Volumes {
							if volume.ConfigMap != nil && volume.ConfigMap.Name == configMap.Name {
								isInUse = true
								break
							}
						}
					}
					if pod.Spec.Containers != nil {
						for _, container := range pod.Spec.Containers {
							if container.VolumeMounts != nil {
								for _, volumeMount := range container.VolumeMounts {
									if volumeMount.Name == configMap.Name {
										isInUse = true
										break
									}
								}
							}
							for _, env := range container.Env {
								if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == configMap.Name {
									isInUse = true
									break
								}
							}
						}
					}
					if pod.Spec.InitContainers != nil {
						for _, initContainer := range pod.Spec.InitContainers {
							if initContainer.VolumeMounts != nil {
								for _, volumeMount := range initContainer.VolumeMounts {
									if volumeMount.Name == configMap.Name {
										isInUse = true
										break
									}
								}
							}
						}
					}
				}
			}
			if !isInUse {
				result.ConfigMaps = append(result.ConfigMaps, createCURE(configMap.Name, configMap.Namespace, "configmap not used by any pod"))
				if !dryRun {
					resName, err := kubernetes.GetResourcesNameForKind(entry.GetKind())
					if err != nil {
						continue
					}
					err = kubernetes.DeleteResource(entry.GroupVersionKind().Group, entry.GroupVersionKind().Version, resName, entry.GetName(), entry.GetNamespace(), false)
					if err != nil {
						self.logger.Error("failed to delete configmap", "configmap", configMap.Name, "error", err)
					}
				}
			}
			continue
		}

		// JOBS
		if entry.GetKind() == "Job" && jobs {
			var job batchv1.Job
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(entry.UnstructuredContent(), &job)
			if err != nil {
				continue
			}
			if job.Status.Succeeded == *job.Spec.Completions && job.Status.Failed == 0 {
				result.Jobs = append(result.Jobs, createCURE(job.Name, job.Namespace, "job completed"))
				if !dryRun {
					resName, err := kubernetes.GetResourcesNameForKind(entry.GetKind())
					if err != nil {
						continue
					}
					err = kubernetes.DeleteResource(entry.GroupVersionKind().Group, entry.GroupVersionKind().Version, resName, entry.GetName(), entry.GetNamespace(), false)
					if err != nil {
						self.logger.Error("failed to delete job", "job", job.Name, "error", err)
					}
				}
			}
			continue
		}

		// INGRESSES
		if entry.GetKind() == "Ingress" && ingresses {
			var ingress netv1.Ingress
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(entry.UnstructuredContent(), &ingress)
			if err != nil {
				continue
			}
			serviceExists := false
			for _, v := range ingress.Spec.Rules {
				if v.HTTP != nil {
					for _, path := range v.HTTP.Paths {
						for _, service := range workSpaceServices {
							if path.Backend.Service.Name == service.Name {
								serviceExists = true
								break
							}
						}
						if serviceExists {
							break
						}
					}
				}
				if serviceExists {
					break
				}
			}
			if !serviceExists {
				result.Ingresses = append(result.Ingresses, createCURE(ingress.Name, ingress.Namespace, "ingress not used by any service"))
				if !dryRun {
					resName, err := kubernetes.GetResourcesNameForKind(entry.GetKind())
					if err != nil {
						continue
					}
					err = kubernetes.DeleteResource(entry.GroupVersionKind().Group, entry.GroupVersionKind().Version, resName, entry.GetName(), entry.GetNamespace(), false)
					if err != nil {
						self.logger.Error("failed to delete ingress", "ingress", ingress.Name, "error", err)
					}
				}
			}
		}
		continue
	}
	return result, nil
}

func createCURE(name, namespace, reason string) CleanUpResultEntry {
	return CleanUpResultEntry{
		Name:      name,
		Namespace: namespace,
		Reason:    reason,
	}
}

func podMatchesSelector(pod *corev1.Pod, selector map[string]string) bool {
	labels := pod.GetLabels()
	for key, value := range selector {
		if podValue, exists := labels[key]; !exists || podValue != value {
			return false
		}
	}
	return true
}
