package core

import (
	"context"
	"fmt"
	"log/slog"
	"mogenius-operator/src/assert"
	"mogenius-operator/src/config"
	"mogenius-operator/src/dtos"
	"mogenius-operator/src/k8sclient"
	"mogenius-operator/src/kubernetes"
	"mogenius-operator/src/podstatscollector"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"

	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
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
		self.logger.Info("🔑 Created new mogenius secret", "name", result.GetObjectMeta().GetName())
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
			self.logger.Info("🔑 Updated mogenius secret", "name", result.GetObjectMeta().GetName())
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
	if meta, ok := unstructuredContent["metadata"].(map[string]any); ok {
		delete(meta, "managedFields")
	}

	return obj
}

func (self *moKubernetes) getKubeletNodeStats(nodeName string) (*podstatscollector.NodeMetrics, error) {
	restClient := self.clientProvider.K8sClientSet().CoreV1().RESTClient()
	path := fmt.Sprintf("/api/v1/nodes/%s/proxy/stats/summary", nodeName)

	resultData := restClient.Get().AbsPath(path).Do(context.Background())
	if err := resultData.Error(); err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	rawResponse, err := resultData.Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw response: %w", err)
	}

	result := &podstatscollector.NodeMetrics{}
	if err := json.Unmarshal(rawResponse, result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubelet stats for node %s: %w", nodeName, err)
	}

	return result, nil
}

func (self *moKubernetes) GetNodeStats() ([]dtos.NodeStat, error) {
	nodes := store.GetNodes()
	result := make([]dtos.NodeStat, 0, len(nodes))

	// Build a per-node pod index once to avoid O(n²) store scans in the loop.
	allPods := store.GetPods("*")
	podsByNode := make(map[string][]corev1.Pod, len(nodes))
	for _, pod := range allPods {
		pod.Kind = "Pod"
		pod.APIVersion = "v1"
		podsByNode[pod.Spec.NodeName] = append(podsByNode[pod.Spec.NodeName], pod)
	}

	for _, node := range nodes {
		skipNode := false
		for _, taint := range node.Spec.Taints {
			if taint.Effect == corev1.TaintEffectNoSchedule || taint.Key == "CriticalAddonsOnly" {
				skipNode = true
				break
			}
		}
		if skipNode {
			continue
		}
		allPods := podsByNode[node.Name]
		requestCpuCores, limitCpuCores := kubernetes.SumCpuResources(allPods)
		requestMemoryBytes, limitMemoryBytes := kubernetes.SumMemoryResources(allPods)

		utilizedCores := float64(0)
		utilizedMemory := int64(0)

		// Prefer cached value from Valkey (written every 60s by podStatsCollector).
		// Fall back to a direct kubelet call during cold start (first ~60s after boot).
		cachedNodeStats, cacheErr := self.valkeyStatsDb.GetLatestNodeStatsForNode(node.Name)
		if cacheErr == nil && cachedNodeStats != nil {
			utilizedCores = float64(cachedNodeStats.CpuUsageNanoCores) / 1_000_000_000
			utilizedMemory = cachedNodeStats.MemoryWorkingSetBytes
		} else {
			kubeletStats, err := self.getKubeletNodeStats(node.Name)
			if err != nil {
				self.logger.Error("Failed to get kubelet stats for node", "node.name", node.Name, "error", err)
			} else {
				// CPU: nanocores -> float64 cores (1 core = 1,000,000,000 nanocores)
				utilizedCores = float64(kubeletStats.Node.CPU.UsageNanoCores) / 1_000_000_000
				// Memory: WorkingSetBytes matches what metrics-server reported
				utilizedMemory = int64(kubeletStats.Node.Memory.WorkingSetBytes)
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

	// Pre-build sets for O(1) lookups
	serviceNameSet := make(map[string]struct{}, len(workSpaceServices))
	for _, svc := range workSpaceServices {
		serviceNameSet[svc.Name] = struct{}{}
	}

	// Pre-build sets of secret/configmap names referenced by pods and ingresses
	usedSecretNames := make(map[string]struct{})
	usedConfigMapNames := make(map[string]struct{})
	for _, pod := range workspacePods {
		for _, volume := range pod.Spec.Volumes {
			if volume.Secret != nil {
				usedSecretNames[volume.Secret.SecretName] = struct{}{}
			}
			if volume.ConfigMap != nil {
				usedConfigMapNames[volume.ConfigMap.Name] = struct{}{}
			}
		}
		for _, container := range pod.Spec.Containers {
			for _, env := range container.Env {
				if env.ValueFrom != nil {
					if env.ValueFrom.SecretKeyRef != nil {
						usedSecretNames[env.ValueFrom.SecretKeyRef.Name] = struct{}{}
					}
					if env.ValueFrom.ConfigMapKeyRef != nil {
						usedConfigMapNames[env.ValueFrom.ConfigMapKeyRef.Name] = struct{}{}
					}
				}
			}
			for _, envFrom := range container.EnvFrom {
				if envFrom.SecretRef != nil {
					usedSecretNames[envFrom.SecretRef.Name] = struct{}{}
				}
				if envFrom.ConfigMapRef != nil {
					usedConfigMapNames[envFrom.ConfigMapRef.Name] = struct{}{}
				}
			}
		}
		for _, initContainer := range pod.Spec.InitContainers {
			for _, envFrom := range initContainer.EnvFrom {
				if envFrom.SecretRef != nil {
					usedSecretNames[envFrom.SecretRef.Name] = struct{}{}
				}
				if envFrom.ConfigMapRef != nil {
					usedConfigMapNames[envFrom.ConfigMapRef.Name] = struct{}{}
				}
			}
		}
		for _, imagePullSecret := range pod.Spec.ImagePullSecrets {
			usedSecretNames[imagePullSecret.Name] = struct{}{}
		}
	}
	for _, ingress := range workspaceIngresses {
		for _, tls := range ingress.Spec.TLS {
			usedSecretNames[tls.SecretName] = struct{}{}
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
			_, isInUse := usedSecretNames[secret.Name]
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
			_, isInUse := usedConfigMapNames[configMap.Name]
			if configMap.Name == "kube-root-ca.crt" {
				isInUse = true
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
			completions := int32(1)
			if job.Spec.Completions != nil {
				completions = *job.Spec.Completions
			}
			if job.Status.Succeeded == completions && job.Status.Failed == 0 {
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
						if path.Backend.Service != nil {
							if _, ok := serviceNameSet[path.Backend.Service.Name]; ok {
								serviceExists = true
								break
							}
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
