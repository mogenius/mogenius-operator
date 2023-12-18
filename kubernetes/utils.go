package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/version"
	"path/filepath"
	"strings"
	"time"

	punq "github.com/mogenius/punq/kubernetes"
	punqStructs "github.com/mogenius/punq/structs"
	punqUtils "github.com/mogenius/punq/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	NAMESPACE       = utils.CONFIG.Kubernetes.OwnNamespace
	DEPLOYMENTNAME  = "mogenius-k8s-manager"
	DEPLOYMENTIMAGE = "ghcr.io/mogenius/mogenius-k8s-manager:" + version.Ver

	SERVICEACCOUNTNAME     = "mogenius-k8s-manager-service-account-app"
	CLUSTERROLENAME        = "mogenius-k8s-manager-cluster-role-app"
	CLUSTERROLEBINDINGNAME = "mogenius-k8s-manager-cluster-role-binding-app"
	RBACRESOURCES          = []string{"pods", "services", "endpoints", "secrets"}
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

func WorkloadResult(result interface{}, error interface{}) K8sWorkloadResult {
	return K8sWorkloadResult{
		Result: result,
		Error:  error,
	}
}

func NewWorkload(name string, yaml string, description string) K8sNewWorkload {
	return K8sNewWorkload{
		Name:        name,
		YamlString:  yaml,
		Description: description,
	}
}

func CurrentContextName() string {
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

func Hostname() string {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return ""
	}
	if provider == nil || err != nil {
		logger.Log.Fatal("error creating kubeprovider")
	}

	return provider.ClientConfig.Host
}

func ListNodes() []v1.Node {
	provider, err := punq.NewKubeProvider(nil)
	if err != nil {
		return []v1.Node{}
	}
	if provider == nil || err != nil {
		logger.Log.Fatal("error creating kubeprovider")
		return []v1.Node{}
	}

	nodeMetricsList, err := provider.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("ListNodeMetrics ERROR: %s", err.Error())
		return []v1.Node{}
	}
	return nodeMetricsList.Items
}

// TAKEN FROM Kubernetes apimachineryv0.25.1
func HumanDuration(d time.Duration) string {
	// Allow deviation no more than 2 seconds(excluded) to tolerate machine time
	// inconsistence, it can be considered as almost now.
	if seconds := int(d.Seconds()); seconds < -1 {
		return "<invalid>"
	} else if seconds < 0 {
		return "0s"
	} else if seconds < 60*2 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := int(d / time.Minute)
	if minutes < 10 {
		s := int(d/time.Second) % 60
		if s == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm%ds", minutes, s)
	} else if minutes < 60*3 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d / time.Hour)
	if hours < 8 {
		m := int(d/time.Minute) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, m)
	} else if hours < 48 {
		return fmt.Sprintf("%dh", hours)
	} else if hours < 24*8 {
		h := hours % 24
		if h == 0 {
			return fmt.Sprintf("%dd", hours/24)
		}
		return fmt.Sprintf("%dd%dh", hours/24, h)
	} else if hours < 24*365*2 {
		return fmt.Sprintf("%dd", hours/24)
	} else if hours < 24*365*8 {
		dy := int(hours/24) % 365
		if dy == 0 {
			return fmt.Sprintf("%dy", hours/24/365)
		}
		return fmt.Sprintf("%dy%dd", hours/24/365, dy)
	}
	return fmt.Sprintf("%dy", int(hours/24/365))
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

func MoUpdateLabels(labels *map[string]string, projectId string, namespace *dtos.K8sNamespaceDto, service *dtos.K8sServiceDto) map[string]string {
	resultingLabels := map[string]string{}

	// transfer existing values
	if labels != nil {
		for k, v := range *labels {
			resultingLabels[k] = v
		}
	}

	// populate with mo labels
	resultingLabels["createdBy"] = DEPLOYMENTNAME
	if service != nil {
		resultingLabels["mo-service-id"] = service.Id
		resultingLabels["mo-service-name"] = service.Name
	}
	if namespace != nil {
		resultingLabels["mo-namespace-id"] = namespace.Id
		resultingLabels["mo-namespace-name"] = namespace.Name
	}

	resultingLabels["mo-project-id"] = projectId

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
func Mount(volumeNamespace string, volumeName string, nfsService *v1.Service) {
	if utils.CONFIG.Misc.Stage == "local" {
		return
	}

	go func() {
		var service *v1.Service = nfsService
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
				punqStructs.ExecuteBashCommandWithResponse(title, shellCmd)
			}
		} else {
			logger.Log.Warningf("No ClusterIP for '%s/%s' nfs-server-pod-%s found.", volumeNamespace, volumeName, volumeName)
		}
	}()
}

func ServiceForNfsVolume(volumeNamespace string, volumeName string) *v1.Service {
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
			punqStructs.ExecuteBashCommandWithResponse(title, shellCmd)
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
