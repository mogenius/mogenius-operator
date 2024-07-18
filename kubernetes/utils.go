package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/version"
	"net"
	"path/filepath"
	"strings"
	"time"

	punqDtos "github.com/mogenius/punq/dtos"
	punq "github.com/mogenius/punq/kubernetes"
	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	appsV1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubectl/pkg/scheme"
)

var K8sLogger = log.WithField("component", structs.ComponentKubernetes)

var (
	NAMESPACE       = utils.CONFIG.Kubernetes.OwnNamespace
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

type K8sWorkloadResult struct {
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

type K8sNewWorkload struct {
	Name        string `json:"name"`
	YamlString  string `json:"yamlString"`
	Description string `json:"description"`
}

type MogeniusNfsInstallationStatus struct {
	Error       string `json:"error,omitempty"`
	IsInstalled bool   `json:"isInstalled"`
}

func init() {
	// SETUP DOWNFAULT VALUE
	if NAMESPACE == "" {
		NAMESPACE = "mogenius"
	}
}
func getCoreClient() coreV1.CoreV1Interface {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatal("Error creating kubeprovider")
	}
	return provider.ClientSet.CoreV1()
}

func GetAppClient() appsV1.AppsV1Interface {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatal("Error creating kubeprovider")
	}
	return provider.ClientSet.AppsV1()
}

func WorkloadResult(result interface{}, error interface{}) K8sWorkloadResult {
	return K8sWorkloadResult{
		Result: result,
		Error:  error,
	}
}

// func NewWorkload(name string, yaml string, description string) K8sNewWorkload {
// 	return K8sNewWorkload{
// 		Name:        name,
// 		YamlString:  yaml,
// 		Description: description,
// 	}
// }

func CurrentContextName() string {
	if utils.CONFIG.Kubernetes.RunInCluster {
		return utils.CONFIG.Kubernetes.ClusterName
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
		return fmt.Sprintf("Error: %v", err)
	}

	return config.CurrentContext
}

// func Hostname() string {
// 	provider, err := punq.NewKubeProvider(nil)
// 	if provider == nil || err != nil {
// 		K8sLogger.Fatal("error creating kubeprovider")
// 	}

// 	return provider.ClientConfig.Host
// }

func ListNodes() []core.Node {
	provider, err := punq.NewKubeProvider(nil)
	if provider == nil || err != nil {
		K8sLogger.Fatal("error creating kubeprovider")
		return []core.Node{}
	}

	nodeMetricsList, err := provider.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		K8sLogger.Errorf("ListNodeMetrics ERROR: %s", err.Error())
		return []core.Node{}
	}
	return nodeMetricsList.Items
}

// TAKEN FROM Kubernetes apimachineryv0.25.1
// func HumanDuration(d time.Duration) string {
// 	// Allow deviation no more than 2 seconds(excluded) to tolerate machine time
// 	// inconsistence, it can be considered as almost now.
// 	if seconds := int(d.Seconds()); seconds < -1 {
// 		return "<invalid>"
// 	} else if seconds < 0 {
// 		return "0s"
// 	} else if seconds < 60*2 {
// 		return fmt.Sprintf("%ds", seconds)
// 	}
// 	minutes := int(d / time.Minute)
// 	if minutes < 10 {
// 		s := int(d/time.Second) % 60
// 		if s == 0 {
// 			return fmt.Sprintf("%dm", minutes)
// 		}
// 		return fmt.Sprintf("%dm%ds", minutes, s)
// 	} else if minutes < 60*3 {
// 		return fmt.Sprintf("%dm", minutes)
// 	}
// 	hours := int(d / time.Hour)
// 	if hours < 8 {
// 		m := int(d/time.Minute) % 60
// 		if m == 0 {
// 			return fmt.Sprintf("%dh", hours)
// 		}
// 		return fmt.Sprintf("%dh%dm", hours, m)
// 	} else if hours < 48 {
// 		return fmt.Sprintf("%dh", hours)
// 	} else if hours < 24*8 {
// 		h := hours % 24
// 		if h == 0 {
// 			return fmt.Sprintf("%dd", hours/24)
// 		}
// 		return fmt.Sprintf("%dd%dh", hours/24, h)
// 	} else if hours < 24*365*2 {
// 		return fmt.Sprintf("%dd", hours/24)
// 	} else if hours < 24*365*8 {
// 		dy := int(hours/24) % 365
// 		if dy == 0 {
// 			return fmt.Sprintf("%dy", hours/24/365)
// 		}
// 		return fmt.Sprintf("%dy%dd", hours/24/365, dy)
// 	}
// 	return fmt.Sprintf("%dy", int(hours/24/365))
// }

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
	if utils.CONFIG.Misc.Stage == utils.STAGE_LOCAL {
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
			if utils.CONFIG.Misc.AutoMountNfs && utils.CONFIG.Kubernetes.RunInCluster {
				title := fmt.Sprintf("Mount [%s] into k8s-manager", volumeName)
				mountDir := fmt.Sprintf("%s/%s_%s", utils.CONFIG.Misc.DefaultMountPath, volumeNamespace, volumeName)
				shellCmd := fmt.Sprintf("mount.nfs -o nolock %s:/exports %s", service.Spec.ClusterIP, mountDir)
				punqUtils.CreateDirIfNotExist(mountDir)
				punqStructs.ExecuteShellCommandWithResponse(title, shellCmd)
			}
		} else {
			K8sLogger.Warningf("No ClusterIP for '%s/%s' nfs-server-pod-%s found.", volumeNamespace, volumeName, volumeName)
		}
	}()
}

func ServiceForNfsVolume(volumeNamespace string, volumeName string) *core.Service {
	services := punq.AllServices(volumeNamespace, nil)
	for _, srv := range services {
		if strings.Contains(srv.Name, fmt.Sprintf("%s-%s", utils.CONFIG.Misc.NfsPodPrefix, volumeName)) {
			return &srv
		}
	}
	return nil
}

// umount nfs server in k8s-manager
func Umount(volumeNamespace string, volumeName string) {
	go func() {
		if utils.CONFIG.Misc.AutoMountNfs && utils.CONFIG.Kubernetes.RunInCluster {
			title := fmt.Sprintf("Unmount [%s] from k8s-manager", volumeName)
			mountDir := fmt.Sprintf("%s/%s_%s", utils.CONFIG.Misc.DefaultMountPath, volumeNamespace, volumeName)
			shellCmd := fmt.Sprintf("umount %s", mountDir)
			punqStructs.ExecuteShellCommandWithResponse(title, shellCmd)
			punqUtils.DeleteDirIfExist(mountDir)
		}
	}()
}

func IsLocalClusterSetup() bool {
	ips := punq.GetClusterExternalIps(nil)
	if punqUtils.Contains(ips, "192.168.66.1") || punqUtils.Contains(ips, "localhost") {
		return true
	} else {
		return false
	}
}

func GetCustomDeploymentTemplate() *v1.Deployment {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		K8sLogger.Error(fmt.Sprintf("GetCustomDeploymentTemplate: %s", err.Error()))
		return nil
	}
	client := provider.ClientSet.CoreV1().ConfigMaps(utils.CONFIG.Kubernetes.OwnNamespace)
	configmap, err := client.Get(context.TODO(), utils.MOGENIUS_CONFIGMAP_DEFAULT_DEPLOYMENT_NAME, metav1.GetOptions{})
	if err != nil {
		return nil
	} else {
		deployment := v1.Deployment{}
		yamlBytes := []byte(configmap.Data["deployment"])
		s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
		_, _, err = s.Decode(yamlBytes, nil, &deployment)
		if err != nil {
			K8sLogger.Error(fmt.Sprintf("GetCustomDeploymentTemplate (unmarshal): %s", err.Error()))
			return nil
		}
		return &deployment
	}
}

func StorageClassForClusterProvider(clusterProvider punqDtos.KubernetesProvider) string {
	var nfsStorageClassStr string = ""

	// 1. WE TRY TO GET THE DEFAULT STORAGE CLASS
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		K8sLogger.Errorf("StorageClassForClusterProvider ERR: %s", err.Error())
		return nfsStorageClassStr
	}
	storageClasses, err := provider.ClientSet.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		K8sLogger.Errorf("StorageClassForClusterProvider List ERR: %s", err.Error())
		return nfsStorageClassStr
	}
	for _, storageClass := range storageClasses.Items {
		if storageClass.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			nfsStorageClassStr = storageClass.Name
			break
		}
	}

	// 2. SOMETIMES WE KNOW IT BETTER THAN KUBERNETES (REASONS: TO EXPENSIVE OR NOT COMPATIBLE WITH OUR NFS SERVER)
	switch clusterProvider {
	case punqDtos.EKS:
		nfsStorageClassStr = "gp2"
	case punqDtos.GKE:
		nfsStorageClassStr = "standard-rwo"
	case punqDtos.AKS:
		nfsStorageClassStr = "default"
	case punqDtos.OTC:
		nfsStorageClassStr = "csi-disk"
	case punqDtos.BRING_YOUR_OWN:
		nfsStorageClassStr = "default"
	case punqDtos.DOCKER_DESKTOP, punqDtos.KIND:
		nfsStorageClassStr = "hostpath"
	case punqDtos.K3S:
		nfsStorageClassStr = "local-path"
	}

	if nfsStorageClassStr == "" {
		K8sLogger.Errorf("No default storage class found for cluster provider '%s'.", clusterProvider)
	}

	return nfsStorageClassStr
}

func GatherNamesForIps(ips []string) map[string]string {
	result := map[string]string{}
	pods := punq.AllPods("", nil)
	services := punq.AllServices("", nil)

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
