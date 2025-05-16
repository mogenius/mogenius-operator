package core

import (
	"context"
	"log/slog"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/k8sclient"
	"mogenius-k8s-manager/src/kubernetes"
	"mogenius-k8s-manager/src/utils"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	v1metrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/yaml"
)

type MoKubernetes interface {
	Run()
	Link(valkeyStatsDb ValkeyStatsDb)
	GetAvailableResources() ([]utils.SyncResourceEntry, error)
	CreateOrUpdateClusterSecret() (utils.ClusterSecret, error)
	CreateOrUpdateResourceTemplateConfigmap() error
	GetNodeStats() []dtos.NodeStat
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

	existingSecret, getErr := secretClient.Get(context.TODO(), self.config.Get("MO_OWN_NAMESPACE"), metav1.GetOptions{})
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

	if existingSecret == nil || getErr != nil {
		self.logger.Info("🔑 Creating new mogenius secret ...")
		result, err := secretClient.Create(context.TODO(), &secret, self.createOptions())
		if err != nil {
			self.logger.Error("Error creating mogenius secret.", "error", err)
			return clusterSecret, err
		}
		self.logger.Info("🔑 Created new mogenius secret", result.GetObjectMeta().GetName(), ".")
	} else {
		if string(existingSecret.Data["cluster-mfa-id"]) != clusterSecret.ClusterMfaId ||
			string(existingSecret.Data["api-key"]) != clusterSecret.ApiKey ||
			string(existingSecret.Data["cluster-name"]) != clusterSecret.ClusterName {
			self.logger.Info("🔑 Updating existing mogenius secret ...")
			result, err := secretClient.Update(context.TODO(), &secret, self.updateOptions())
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

func (self *moKubernetes) GetAvailableResources() ([]utils.SyncResourceEntry, error) {
	clientset := self.clientProvider.K8sClientSet()

	resources, err := clientset.Discovery().ServerPreferredResources()
	if err != nil {
		self.logger.Error("Error discovering resources", "error", err)
		return nil, err
	}

	var availableResources []utils.SyncResourceEntry
	for _, resourceList := range resources {
		for _, resource := range resourceList.APIResources {
			if slices.Contains(resource.Verbs, "list") && slices.Contains(resource.Verbs, "watch") {
				var namespace *string
				if resource.Namespaced {
					namespace = utils.Pointer("")
				}

				availableResources = append(availableResources, utils.SyncResourceEntry{
					Group:     resourceList.GroupVersion,
					Name:      resource.Name,
					Kind:      resource.Kind,
					Version:   resource.Version,
					Namespace: namespace,
				})
			}
		}
	}

	return availableResources, nil
}

func (self *moKubernetes) getResourceTemplateConfigMap() string {
	return "mogenius-resource-templates"
}

func (self *moKubernetes) CreateOrUpdateResourceTemplateConfigmap() error {
	yamlData := utils.InitResourceTemplatesYaml()

	// Decode YAML data into a generic map
	var decodedData map[string]interface{}
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
	_, err = self.CreateUnstructuredResource("", "v1", "configmaps", utils.Pointer(""), string(updatedYaml))
	if apierrors.IsAlreadyExists(err) {
		_, err = kubernetes.UpdateUnstructuredResource("", "v1", "configmaps", utils.Pointer(""), string(updatedYaml))
		if err != nil {
			self.logger.Error("Resource template configmap failed to update", "error", err)
			return err
		}
		self.logger.Info("Resource template configmap updated")
		return nil
	}

	return err
}

func (self *moKubernetes) CreateUnstructuredResource(group string, version string, name string, namespace *string, yamlData string) (*unstructured.Unstructured, error) {
	dynamicClient := self.clientProvider.DynamicClient()
	obj := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlData), obj)
	if err != nil {
		return nil, err
	}

	if namespace != nil {
		result, err := dynamicClient.Resource(kubernetes.CreateGroupVersionResource(group, version, name)).Namespace(obj.GetNamespace()).Create(context.TODO(), obj, metav1.CreateOptions{})
		return self.removeManagedFields(result), err
	} else {
		result, err := dynamicClient.Resource(kubernetes.CreateGroupVersionResource(group, version, name)).Create(context.TODO(), obj, metav1.CreateOptions{})
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
		delete(unstructuredContent["metadata"].(map[string]interface{}), "managedFields")
	}

	return obj
}

func (self *moKubernetes) GetNodeStats() []dtos.NodeStat {
	result := []dtos.NodeStat{}
	nodes := kubernetes.ListNodes()
	nodeMetrics := kubernetes.ListNodeMetricss()

	for _, node := range nodes {
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

	return result
}
