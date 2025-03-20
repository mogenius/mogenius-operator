package kubernetes

import (
	"context"
	json1 "encoding/json"
	"fmt"
	"mogenius-k8s-manager/src/assert"
	"mogenius-k8s-manager/src/dtos"
	"mogenius-k8s-manager/src/shutdown"
	"mogenius-k8s-manager/src/structs"
	"mogenius-k8s-manager/src/utils"
	"mogenius-k8s-manager/src/version"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	version2 "k8s.io/apimachinery/pkg/version"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/yaml"

	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubectl/pkg/scheme"
)

var (
	DEPLOYMENTNAME  = "mogenius-k8s-manager"
	DEPLOYMENTIMAGE = "ghcr.io/mogenius/mogenius-k8s-manager:" + version.Ver

	SERVICEACCOUNTNAME     = "mogenius-k8s-manager-service-account-app"
	CLUSTERROLENAME        = "mogenius-k8s-manager-cluster-role-app"
	CLUSTERROLEBINDINGNAME = "mogenius-k8s-manager-cluster-role-binding-app"
	RBACRESOURCES          = []string{"pods", "services", "endpoints", "secrets"}
)

const (
	MO_LABEL_CREATED_BY            = "mo-created-by"
	MO_LABEL_APP_NAME              = "mo-app"
	MO_LABEL_NAMESPACE             = "mo-ns"
	MO_LABEL_PROJECT_ID            = "mo-project-id"
	MO_LABEL_NAMESPACE_DISPLAYNAME = "mo-namespace-display-name"
	MO_LABEL_APP_DISPLAYNAME       = "mo-app-display-name"
)

type IngressType int

const (
	NGINX IngressType = iota
	TRAEFIK
	MULTIPLE
	NONE
	UNKNOWN
)

func (i IngressType) String() string {
	return [...]string{"NGINX", "TRAEFIK", "MULTIPLE", "NONE", "UNKNOWN"}[i]
}

type K8sWorkloadResult struct {
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

type MogeniusNfsInstallationStatus struct {
	Error       string `json:"error,omitempty"`
	IsInstalled bool   `json:"isInstalled"`
}

// Define the expected JSON structure
type AuthConfig struct {
	Auths map[string]Credentials `json:"auths"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

func ValidateContainerRegistryAuthString(input string) error {
	var config AuthConfig
	// Try to unmarshal the input string into the struct
	err := json1.Unmarshal([]byte(input), &config)
	if err != nil {
		return fmt.Errorf("invalid JSON structure: %v", err)
	}

	if config.Auths == nil {
		return fmt.Errorf("missing 'auths' field in JSON")
	}

	// removed because for GCP these fields are not needed
	// for _, creds := range config.Auths {
	// 	// if creds.Username == "" || creds.Password == "" || creds.Auth == "" {
	// 	// 	return fmt.Errorf("missing required fields in credentials (username, password, auth)")
	// 	// }
	// }

	return nil
}

func init() {
	// SETUP DOWNFAULT VALUE
	dtos.KubernetesGetSecretValueByPrefixControllerNameAndKey = GetSecretValueByPrefixControllerNameAndKey
}

func WorkloadResult(result interface{}, error interface{}) K8sWorkloadResult {
	return K8sWorkloadResult{
		Result: result,
		Error:  error,
	}
}

func CurrentContextName() string {
	context := config.Get("MO_CLUSTER_NAME")
	if context != "" {
		return context
	}

	var kubeconfig string = ""
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{
			CurrentContext: "",
		}).RawConfig()

	if err != nil {
		return "No context found. Probably running in a cluster."
	}

	return config.CurrentContext
}

// func Hostname() string {
// 	provider, err := NewKubeProvider()
// 	if provider == nil || err != nil {
// 		K8sLogger.Fatal("error creating kubeprovider")
// 	}

// 	return provider.ClientConfig.Host
// }

func ListNodes() []core.Node {
	clientset := clientProvider.K8sClientSet()

	nodeMetricsList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("failed to list nodes", "error", err)
		shutdown.SendShutdownSignal(true)
		select {}
	}
	return nodeMetricsList.Items
}

func KubernetesVersion() *version2.Info {
	clientset := clientProvider.K8sClientSet()
	info, err := clientset.Discovery().ServerVersion()
	if err != nil {
		k8sLogger.Error("Error KubernetesVersion", "error", err)
		return nil
	}
	return info
}

func MoCreateOptions() metav1.CreateOptions {
	return metav1.CreateOptions{
		FieldManager: DEPLOYMENTNAME,
	}
}

func MoUpdateOptions() metav1.UpdateOptions {
	return metav1.UpdateOptions{
		FieldManager: DEPLOYMENTNAME,
	}
}

func MoUpdateLabels(labels *map[string]string, projectId *string, namespace *dtos.K8sNamespaceDto, service *dtos.K8sServiceDto) map[string]string {
	resultingLabels := map[string]string{}

	// transfer existing values
	if labels != nil {
		for k, v := range *labels {
			resultingLabels[k] = v
		}
	}

	// populate with mo labels
	resultingLabels[MO_LABEL_CREATED_BY] = DEPLOYMENTNAME
	if service != nil {
		resultingLabels[MO_LABEL_APP_NAME] = service.ControllerName
	}
	if projectId != nil {
		resultingLabels[MO_LABEL_PROJECT_ID] = *projectId
	}

	return resultingLabels
}

func MoAddLabels(existingLabels *map[string]string, newLabels map[string]string) map[string]string {
	resultingLabels := map[string]string{}

	// transfer existing values
	if existingLabels != nil {
		for k, v := range *existingLabels {
			resultingLabels[k] = v
		}
	}

	// populate with mo labels
	for k, v := range newLabels {
		resultingLabels[k] = v
	}

	return resultingLabels
}

// mount nfs server in k8s-manager
func Mount(volumeNamespace string, volumeName string, nfsService *core.Service) {
	if config.Get("MO_STAGE") == utils.STAGE_LOCAL {
		return
	}

	go func() {
		var service *core.Service = nfsService
		if service == nil {
			service = ServiceForNfsVolume(volumeNamespace, volumeName)
		}
		if service != nil {
			if nfsService != nil {
				time.Sleep(15 * time.Second)
			}
			autoMountNfs, err := strconv.ParseBool(config.Get("MO_AUTO_MOUNT_NFS"))
			assert.Assert(err == nil, err)
			if autoMountNfs && clientProvider.RunsInCluster() {
				title := fmt.Sprintf("Mount [%s] into k8s-manager", volumeName)
				mountDir := fmt.Sprintf("%s/%s_%s", config.Get("MO_DEFAULT_MOUNT_PATH"), volumeNamespace, volumeName)
				shellCmd := fmt.Sprintf("mount.nfs -o nolock %s:/exports %s", service.Spec.ClusterIP, mountDir)
				utils.CreateDirIfNotExist(mountDir)
				utils.ExecuteShellCommandWithResponse(title, shellCmd)
			}
		} else {
			k8sLogger.Warn("No ClusterIP found.", "volumeNamespace", volumeNamespace, "volumeName", volumeName, "resource", "nfs-server-pod-"+volumeName)
		}
	}()
}

func ServiceForNfsVolume(volumeNamespace string, volumeName string) *core.Service {
	services := AllServices(volumeNamespace)
	for _, srv := range services {
		if strings.Contains(srv.Name, fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName)) {
			return &srv
		}
	}
	return nil
}

// umount nfs server in k8s-manager
func Umount(volumeNamespace string, volumeName string) {
	go func() {
		autoMountNfs, err := strconv.ParseBool(config.Get("MO_AUTO_MOUNT_NFS"))
		assert.Assert(err == nil, err)
		if autoMountNfs && clientProvider.RunsInCluster() {
			title := fmt.Sprintf("Unmount [%s] from k8s-manager", volumeName)
			mountDir := fmt.Sprintf("%s/%s_%s", config.Get("MO_DEFAULT_MOUNT_PATH"), volumeNamespace, volumeName)
			shellCmd := fmt.Sprintf("umount %s", mountDir)
			utils.ExecuteShellCommandWithResponse(title, shellCmd)
			utils.DeleteDirIfExist(mountDir)
		}
	}()
}

func IsLocalClusterSetup() bool {
	ips := GetClusterExternalIps()
	if utils.Contains(ips, "192.168.66.1") || utils.Contains(ips, "localhost") {
		return true
	} else {
		return false
	}
}

func GetCustomDeploymentTemplate() *v1.Deployment {
	clientset := clientProvider.K8sClientSet()
	client := clientset.CoreV1().ConfigMaps(config.Get("MO_OWN_NAMESPACE"))
	configmap, err := client.Get(context.TODO(), utils.MOGENIUS_CONFIGMAP_DEFAULT_DEPLOYMENT_NAME, metav1.GetOptions{})
	if err != nil {
		return nil
	} else {
		deployment := v1.Deployment{}
		yamlBytes := []byte(configmap.Data["deployment"])
		s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
		_, _, err = s.Decode(yamlBytes, nil, &deployment)
		if err != nil {
			k8sLogger.Error("GetCustomDeploymentTemplate (unmarshal)", "error", err)
			return nil
		}
		return &deployment
	}
}

func ListNodeMetricss() []v1beta1.NodeMetrics {
	provider, err := NewKubeProviderMetrics()
	if provider == nil || err != nil {
		k8sLogger.Error("ListNodeMetricss", "error", err.Error())
		return []v1beta1.NodeMetrics{}
	}

	nodeMetricsList, err := provider.ClientSet.MetricsV1beta1().NodeMetricses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("ListNodeMetrics", "error", err.Error())
		return []v1beta1.NodeMetrics{}
	}
	return nodeMetricsList.Items
}

func StorageClassForClusterProvider(clusterProvider utils.KubernetesProvider) string {
	var nfsStorageClassStr string = ""

	// 1. WE TRY TO GET THE DEFAULT STORAGE CLASS
	clientset := clientProvider.K8sClientSet()
	storageClasses, err := clientset.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		k8sLogger.Error("StorageClassForClusterProvider List", "error", err)
		return nfsStorageClassStr
	}
	for _, storageClass := range storageClasses.Items {
		if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			nfsStorageClassStr = storageClass.Name
			break
		}
	}

	// 2. SOMETIMES WE KNOW IT BETTER THAN KUBERNETES (REASONS: TO EXPENSIVE OR NOT COMPATIBLE WITH OUR NFS SERVER)
	if nfsStorageClassStr == "" {
		switch clusterProvider {
		case utils.EKS:
			nfsStorageClassStr = "gp2"
		case utils.GKE:
			nfsStorageClassStr = "standard-rwo"
		case utils.AKS:
			nfsStorageClassStr = "default"
		case utils.OTC:
			nfsStorageClassStr = "csi-disk"
		case utils.BRING_YOUR_OWN:
			nfsStorageClassStr = "default"
		case utils.DOCKER_DESKTOP, utils.KIND:
			nfsStorageClassStr = "hostpath"
		case utils.K3S:
			nfsStorageClassStr = "local-path"
		}
	}
	if nfsStorageClassStr == "" {
		k8sLogger.Error("No default storage class found for cluster provider.", "clusterProvider", clusterProvider)
	}

	return nfsStorageClassStr
}

func GatherNamesForIps(ips []string) map[string]string {
	result := map[string]string{}
	pods := AllPods("")
	services := AllServices("")

outerLoop:
	for _, ip := range ips {
		owner := ""
		for _, pod := range pods {
			if pod.Status.PodIP == ip {
				if len(pod.OwnerReferences) > 0 {
					owner = fmt.Sprintf("/%s/%s", pod.OwnerReferences[0].Kind, pod.OwnerReferences[0].Name)
				}
				result[ip] = fmt.Sprintf("%s/%s%s", pod.Namespace, pod.Name, owner)
				continue outerLoop
			}
		}
		for _, service := range services {
			if service.Spec.ClusterIP == ip {
				if len(service.OwnerReferences) > 0 {
					owner = fmt.Sprintf("/%s/%s", service.OwnerReferences[0].Kind, service.OwnerReferences[0].Name)
				}
				result[ip] = fmt.Sprintf("%s/%s%s", service.Namespace, service.Name, owner)
				continue outerLoop
			}
		}
		parsedIP := net.ParseIP(ip)
		if parsedIP != nil {
			if !parsedIP.IsPrivate() {
				result[ip] = "@External"
				continue outerLoop
			}
		}

		result[ip] = ""
	}
	return result
}

func GetLabelValue(labels map[string]string, labelKey string) (string, error) {
	if labels == nil {
		return "", fmt.Errorf("Labels are nil")
	}

	if val, ok := labels[labelKey]; ok {
		return val, nil
	}

	return "", fmt.Errorf("Label value for key:'%s' not found", labelKey)
}

func ContainsLabelKey(labels map[string]string, key string) bool {
	if labels == nil {
		return false
	}

	_, ok := labels[key]
	return ok
}

func FindResourceKind(namespace string, name string) (*dtos.K8sServiceControllerEnum, error) {
	clientset := clientProvider.K8sClientSet()

	if _, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
		return utils.Pointer(dtos.DEPLOYMENT), nil
	}

	if _, err := clientset.AppsV1().ReplicaSets(namespace).Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
		return utils.Pointer(dtos.REPLICA_SET), nil
	}

	if _, err := clientset.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
		return utils.Pointer(dtos.STATEFUL_SET), nil
	}

	if _, err := clientset.AppsV1().DaemonSets(namespace).Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
		return utils.Pointer(dtos.DAEMON_SET), nil
	}

	if _, err := clientset.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
		return utils.Pointer(dtos.JOB), nil
	}

	if _, err := clientset.BatchV1beta1().CronJobs(namespace).Get(context.TODO(), name, metav1.GetOptions{}); err == nil {
		return utils.Pointer(dtos.CRON_JOB), nil
	}

	return nil, fmt.Errorf("Resource not found")
}

func GuessClusterProvider() (utils.KubernetesProvider, error) {
	clientset := clientProvider.K8sClientSet()
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return utils.SELF_HOSTED, err
	}

	return GuessCluserProviderFromNodeList(nodes)
}

func GuessCluserProviderFromNodeList(nodes *core.NodeList) (utils.KubernetesProvider, error) {

	for _, node := range nodes.Items {
		nodeInfo := map[string]string{}
		// TODO: this is broken and deprecated (because it is broken)
		nodeInfo["kubeProxyVersion"] = node.Status.NodeInfo.KubeProxyVersion //nolint:staticcheck
		nodeInfo["kubeletVersion"] = node.Status.NodeInfo.KubeletVersion

		labelsAndAnnotations := utils.MergeMaps(node.GetLabels(), node.GetAnnotations(), nodeInfo)

		if LabelsContain(labelsAndAnnotations, "eks.amazonaws.com/") {
			return utils.EKS, nil
		} else if LabelsContain(labelsAndAnnotations, "docker-desktop") {
			return utils.DOCKER_DESKTOP, nil
		} else if LabelsContain(labelsAndAnnotations, "kubernetes.azure.com/role") {
			return utils.AKS, nil
		} else if LabelsContain(labelsAndAnnotations, "cloud.google.com/gke-nodepool") {
			return utils.GKE, nil
		} else if LabelsContain(labelsAndAnnotations, "k3s.io/hostname") {
			return utils.K3S, nil
		} else if LabelsContain(labelsAndAnnotations, "ibm-cloud.kubernetes.io/worker-version") {
			return utils.IBM, nil
		} else if LabelsContain(labelsAndAnnotations, "doks.digitalocean.com/node-id") {
			return utils.DOKS, nil
		} else if LabelsContain(labelsAndAnnotations, "oke.oraclecloud.com/node-pool") {
			return utils.OKE, nil
		} else if LabelsContain(labelsAndAnnotations, "ack.aliyun.com") {
			return utils.ACK, nil
		} else if LabelsContain(labelsAndAnnotations, "node-role.kubernetes.io/master") && LabelsContain(labelsAndAnnotations, "node.openshift.io/os_id") {
			return utils.OPEN_SHIFT, nil
		} else if LabelsContain(labelsAndAnnotations, "vmware-system-vmware.io/role") || ImagesContain(node.Status.Images, "vmware.com/tkg/kube-apiserver") {
			return utils.VMWARE, nil
		} else if LabelsContain(labelsAndAnnotations, "io.rancher.os/hostname") {
			return utils.RKE, nil
		} else if LabelsContain(labelsAndAnnotations, "linode-lke/") {
			return utils.LINODE, nil
		} else if LabelsContain(labelsAndAnnotations, "scaleway-kapsule/") {
			return utils.SCALEWAY, nil
		} else if LabelsContain(labelsAndAnnotations, "microk8s.io/cluster") {
			return utils.MICROK8S, nil
		} else if strings.ToLower(node.Name) == "minikube" {
			return utils.MINIKUBE, nil
		} else if LabelsContain(labelsAndAnnotations, "io.k8s.sigs.kind/role") {
			return utils.KIND, nil
		} else if LabelsContain(labelsAndAnnotations, "civo-node-pool") {
			return utils.CIVO, nil
		} else if LabelsContain(labelsAndAnnotations, "giantswarm.io/") {
			return utils.GIANTSWARM, nil
		} else if LabelsContain(labelsAndAnnotations, "ovhcloud/") {
			return utils.OVHCLOUD, nil
		} else if LabelsContain(labelsAndAnnotations, "gardener.cloud/role") {
			return utils.GARDENER, nil
		} else if LabelsContain(labelsAndAnnotations, "cce.huawei.com") {
			return utils.HUAWEI, nil
		} else if LabelsContain(labelsAndAnnotations, "nirmata.io") {
			return utils.NIRMATA, nil
		} else if LabelsContain(labelsAndAnnotations, "-CCE") || ImagesContain(node.Status.Images, "cce-addons") {
			return utils.OTC, nil
		} else if LabelsContain(labelsAndAnnotations, "platform9.com/role") {
			return utils.PF9, nil
		} else if LabelsContain(labelsAndAnnotations, "nks.netapp.io") {
			return utils.NKS, nil
		} else if LabelsContain(labelsAndAnnotations, "appscode.com") {
			return utils.APPSCODE, nil
		} else if LabelsContain(labelsAndAnnotations, "loft.sh") {
			return utils.LOFT, nil
		} else if LabelsContain(labelsAndAnnotations, "spectrocloud.com") {
			return utils.SPECTROCLOUD, nil
		} else if LabelsContain(labelsAndAnnotations, "diamanti.com") {
			return utils.DIAMANTI, nil
		} else if strings.HasPrefix(strings.ToLower(node.Name), "k3d-") {
			return utils.K3D, nil
		} else if LabelsContain(labelsAndAnnotations, "cloud.google.com/gke-on-prem") {
			return utils.GKE_ON_PREM, nil
		} else if LabelsContain(labelsAndAnnotations, "rke.cattle.io") {
			return utils.RKE, nil
		} else if ImagesContain(node.Status.Images, "pluscloudopen") {
			return utils.PLUSSERVER, nil
		} else {
			k8sLogger.Error("This cluster's provider is unknown or it might be self-managed.")
			return utils.UNKNOWN, nil
		}
	}
	return utils.UNKNOWN, nil
}

func ImagesContain(images []core.ContainerImage, str string) bool {
	for _, image := range images {
		for _, name := range image.Names {
			if strings.Contains(name, str) {
				return true
			}
		}
	}
	return false
}

func LabelsContain(labels map[string]string, str string) bool {
	// Keys EQUAL
	if _, ok := labels[strings.ToLower(str)]; ok {
		return true
	}

	// Values
	for key, label := range labels {
		if strings.EqualFold(label, str) {
			return true
		}
		// KEY CONTAINS
		if strings.Contains(key, str) {
			return true
		}
	}
	return false
}

func ClusterStatus() dtos.ClusterStatusDto {
	var currentPods = make(map[string]core.Pod)
	pods := AllPods("")
	for _, pod := range pods {
		currentPods[pod.Name] = pod
	}

	result, err := PodStats(currentPods)
	if err != nil {
		k8sLogger.Error("podStats:", "error", err)
	}

	var cpu int64 = 0
	var cpuLimit int64 = 0
	var memory int64 = 0
	var memoryLimit int64 = 0
	var ephemeralStorageLimit int64 = 0
	for _, pod := range result {
		cpu += pod.Cpu
		cpuLimit += pod.CpuLimit
		memory += pod.Memory
		memoryLimit += pod.MemoryLimit
		ephemeralStorageLimit += pod.EphemeralStorageLimit
	}

	kubernetesVersion := ""
	platform := ""

	info := KubernetesVersion()
	if info != nil {
		kubernetesVersion = info.String()
		platform = info.Platform
	}

	country, err := utils.GuessClusterCountry()
	if err != nil {
		k8sLogger.Error("GuessClusterCountry: ", "error", err)
	}

	return dtos.ClusterStatusDto{
		ClusterName:                  config.Get("MO_CLUSTER_NAME"),
		Pods:                         len(result),
		PodCpuUsageInMilliCores:      int(cpu),
		PodCpuLimitInMilliCores:      int(cpuLimit),
		PodMemoryUsageInBytes:        memory,
		PodMemoryLimitInBytes:        memoryLimit,
		EphemeralStorageLimitInBytes: ephemeralStorageLimit,
		KubernetesVersion:            kubernetesVersion,
		Platform:                     platform,
		Country:                      country,
	}
}

func PodStats(pods map[string]core.Pod) ([]structs.Stats, error) {
	provider, err := NewKubeProviderMetrics()
	if provider == nil || err != nil {
		k8sLogger.Error(err.Error())
		return []structs.Stats{}, err
	}

	podMetricsList, err := provider.ClientSet.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []structs.Stats
	// I HATE THIS BUT I DONT SEE ANY OTHER SOLUTION! SPEND HOURS (to find something better) ON THIS UGGLY SHIT!!!!

	for _, podMetrics := range podMetricsList.Items {
		var pod = pods[podMetrics.Name]

		var entry = structs.Stats{}
		entry.Cluster = config.Get("MO_CLUSTER_NAME")
		entry.Namespace = podMetrics.Namespace
		entry.PodName = podMetrics.Name
		if pod.Status.StartTime != nil {
			entry.StartTime = pod.Status.StartTime.Format(time.RFC3339)
		}
		for _, container := range pod.Spec.Containers {
			entry.CpuLimit += container.Resources.Limits.Cpu().MilliValue()
			entry.MemoryLimit += container.Resources.Limits.Memory().Value()
			entry.EphemeralStorageLimit += container.Resources.Limits.StorageEphemeral().Value()
		}
		for _, containerMetric := range podMetrics.Containers {
			entry.Cpu += containerMetric.Usage.Cpu().MilliValue()
			entry.Memory += containerMetric.Usage.Memory().Value()
		}

		result = append(result, entry)
	}

	return result, nil
}

func AllResourcesFrom(namespace string, resourcesToLookFor []string) ([]interface{}, error) {
	ignoredResources := []string{
		"events.k8s.io/v1",
		"events.k8s.io/v1beta1",
		"metrics.k8s.io/v1beta1",
		"discovery.k8s.io/v1",
	}

	result := []interface{}{}

	// Get a list of all resource types in the cluster
	clientset := clientProvider.K8sClientSet()
	resourceList, err := clientset.Discovery().ServerPreferredResources()
	if err != nil {
		return result, err
	}

	// Iterate over each resource type and backup all resources in the namespace
	for _, resource := range resourceList {
		if utils.Contains(ignoredResources, resource.GroupVersion) {
			continue
		}
		gv, _ := schema.ParseGroupVersion(resource.GroupVersion)
		if len(resource.APIResources) <= 0 {
			continue
		}

		for _, aApiResource := range resource.APIResources {
			if !aApiResource.Namespaced {
				continue
			}

			resourceId := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: aApiResource.Name,
			}
			// Get the REST client for this resource type
			restClient := dynamic.New(clientset.RESTClient()).Resource(resourceId).Namespace(namespace)

			// Get a list of all resources of this type in the namespace
			list, err := restClient.List(context.Background(), metav1.ListOptions{})
			if err != nil {
				k8sLogger.Error("error listing namespaces", "resourceId", resourceId.Resource, "error", err.Error())
				continue
			}

			// Iterate over each resource and write it to a file
			for _, obj := range list.Items {
				obj.SetManagedFields(nil)
				delete(obj.Object, "status")
				obj.SetUID("")
				obj.SetResourceVersion("")
				obj.SetCreationTimestamp(metav1.Time{})

				if len(resourcesToLookFor) > 0 {
					if utils.ContainsToLowercase(resourcesToLookFor, obj.GetKind()) {
						result = append(result, obj.Object)
					}
				} else {
					result = append(result, obj.Object)
				}
			}
		}
	}
	return result, nil
}

func AllResourcesFromToCombinedYaml(namespace string, resourcesToLookFor []string) (string, error) {
	result := ""
	resources, err := AllResourcesFrom(namespace, resourcesToLookFor)
	if err != nil {
		return result, err
	}
	for _, res := range resources {
		yamlData, err := yaml.Marshal(res)
		if err != nil {
			return result, err
		}

		// Print the YAML string.
		result += fmt.Sprintf("---\n%s\n", string(yamlData))
	}
	return result, err
}

func ApiVersions() ([]string, error) {
	result := []string{}

	clientset := clientProvider.K8sClientSet()
	groupResources, err := clientset.DiscoveryClient.ServerPreferredResources()
	if err != nil {
		return result, err
	}

	for _, groupList := range groupResources {
		result = append(result, groupList.GroupVersion)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})

	return result, nil
}

func IsMetricsServerAvailable() (bool, string, error) {
	// kube-system would be the right namespace but if somebody installed it in another namespace we want to find it
	deployments := AllDeploymentsIncludeIgnored("")

	for _, deployment := range deployments {
		for _, label := range deployment.Labels {
			if label == "metrics-server" {
				if deployment.Status.UnavailableReplicas > 0 {
					return false, "", fmt.Errorf("metrics-server installed but not running")
				}
				return true, deployment.Spec.Template.Spec.Containers[0].Image, nil
			}
		}
	}

	return false, "", fmt.Errorf("no metrics-server found")
}

func DetermineIngressControllerType() (IngressType, error) {
	ingressClasses := AllIngressClasses()

	if len(ingressClasses) > 1 {
		return MULTIPLE, fmt.Errorf("multiple ingress controllers found")
	}

	if len(ingressClasses) == 0 {
		return NONE, fmt.Errorf("no ingress controller found")
	}

	unknownController := ""
	for _, ingressClass := range ingressClasses {
		if ingressClass.Spec.Controller == "k8s.io/ingress-nginx" || ingressClass.Spec.Controller == "nginx.org/ingress-controller" {
			return NGINX, nil
		} else if ingressClass.Spec.Controller == "traefik.io/ingress-controller" {
			return TRAEFIK, nil
		} else {
			unknownController = ingressClass.Spec.Controller
		}
	}

	return UNKNOWN, fmt.Errorf("unknown ingress controller: %s", unknownController)
}

func IsCertManagerInstalled() (bool, error) {
	deployments, err := GetDeploymentsWithFieldSelector("", "app.kubernetes.io/instance=cert-manager")
	if err != nil {
		return false, err
	}
	if len(deployments) > 0 {
		return true, nil
	}
	return false, nil
}
