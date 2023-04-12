package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/logger"
	"mogenius-k8s-manager/structs"
	"mogenius-k8s-manager/utils"
	"mogenius-k8s-manager/version"
	"path/filepath"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
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
	Result string `json:"result"`
}

type MogeniusNfsInstallationStatus struct {
	Error       string `json:"error,omitempty"`
	IsInstalled bool   `json:"isInstalled"`
}

type KubeProviderMetrics struct {
	ClientSet    *metricsv.Clientset
	ClientConfig rest.Config
}

type KubeProvider struct {
	ClientSet    *kubernetes.Clientset
	ClientConfig rest.Config
}

func init() {
	// SETUP DOWNFAULT VALUE
	if NAMESPACE == "" {
		NAMESPACE = "mogenius"
	}
}

func WorkloadResult(result string) K8sWorkloadResult {
	return K8sWorkloadResult{
		Result: result,
	}
}

func NewKubeProvider() *KubeProvider {
	var kubeProvider *KubeProvider
	var err error
	if utils.CONFIG.Kubernetes.RunInCluster {
		kubeProvider, err = NewKubeProviderInCluster()
	} else {
		kubeProvider, err = NewKubeProviderLocal()
	}

	if err != nil {
		logger.Log.Errorf("ERROR: %s", err.Error())
	}
	return kubeProvider
}

func NewKubeProviderLocal() (*KubeProvider, error) {
	var kubeconfig string = ""
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	restConfig, errConfig := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if errConfig != nil {
		panic(errConfig.Error())
	}

	clientSet, errClientSet := kubernetes.NewForConfig(restConfig)
	if errClientSet != nil {
		panic(errClientSet.Error())
	}

	return &KubeProvider{
		ClientSet:    clientSet,
		ClientConfig: *restConfig,
	}, nil
}

func NewKubeProviderInCluster() (*KubeProvider, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return &KubeProvider{
		ClientSet:    clientset,
		ClientConfig: *config,
	}, nil
}

func NewKubeProviderMetricsLocal() (*KubeProviderMetrics, error) {
	kubeconfig := getKubeConfig()

	restConfig, errConfig := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if errConfig != nil {
		panic(errConfig.Error())
	}

	clientSet, errClientSet := metricsv.NewForConfig(restConfig)
	if errClientSet != nil {
		panic(errClientSet.Error())
	}

	//logger.Log.Debugf("K8s client config (init with .kube/config), host: %s", restConfig.Host)

	return &KubeProviderMetrics{
		ClientSet:    clientSet,
		ClientConfig: *restConfig,
	}, nil
}

func NewKubeProviderMetricsInCluster() (*KubeProviderMetrics, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := metricsv.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	//logger.Log.Debugf("K8s client config (init InCluster), host: %s", config.Host)

	return &KubeProviderMetrics{
		ClientSet:    clientset,
		ClientConfig: *config,
	}, nil
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
	provider, error := NewKubeProviderInCluster()
	if error != nil {
		fmt.Println("Error:", error)
	}

	return provider.ClientConfig.Host
}

func ClusterStatus() dtos.ClusterStatusDto {
	var currentPods = make(map[string]v1.Pod)
	pods := listAllPods()
	for _, pod := range pods {
		currentPods[pod.Name] = pod
	}

	result, err := podStats(currentPods)
	if err != nil {
		logger.Log.Error("podStats:", err)
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

	return dtos.ClusterStatusDto{
		ClusterName:           utils.CONFIG.Kubernetes.ClusterName,
		Pods:                  len(result),
		CpuInMilliCores:       int(cpu),
		CpuLimitInMilliCores:  int(cpuLimit),
		Memory:                utils.BytesToHumanReadable(memory),
		MemoryLimit:           utils.BytesToHumanReadable(memoryLimit),
		EphemeralStorageLimit: utils.BytesToHumanReadable(ephemeralStorageLimit),
	}
}

func listAllPods() []v1.Pod {
	var result []v1.Pod

	kubeProvider := NewKubeProvider()
	pods, err := kubeProvider.ClientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system,metadata.namespace!=default"})

	if err != nil {
		logger.Log.Error("Error listAllPods:", err)
		return result
	}
	return pods.Items
}

func ListNodes() []v1.Node {
	var provider *KubeProvider
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderLocal()
	} else {
		provider, err = NewKubeProviderInCluster()
	}
	if err != nil {
		logger.Log.Errorf("ListNodeMetrics ERROR: %s", err.Error())
		return []v1.Node{}
	}

	nodeMetricsList, err := provider.ClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logger.Log.Errorf("ListNodeMetrics ERROR: %s", err.Error())
		return []v1.Node{}
	}
	return nodeMetricsList.Items
}

func podStats(pods map[string]v1.Pod) ([]structs.Stats, error) {
	var provider *KubeProviderMetrics
	var err error
	if !utils.CONFIG.Kubernetes.RunInCluster {
		provider, err = NewKubeProviderMetricsLocal()
	} else {
		provider, err = NewKubeProviderMetricsInCluster()
	}
	if err != nil {
		panic(err)
	}

	podMetricsList, err := provider.ClientSet.MetricsV1beta1().PodMetricses("").List(context.TODO(), metav1.ListOptions{FieldSelector: "metadata.namespace!=kube-system,metadata.namespace!=default"})
	if err != nil {
		return nil, err
	}

	var result []structs.Stats
	// I HATE THIS BUT I DONT SEE ANY OTHER SOLUTION! SPEND HOURS (to find something better) ON THIS UGGLY SHIT!!!!

	for _, podMetrics := range podMetricsList.Items {
		var pod = pods[podMetrics.Name]

		var entry = structs.Stats{}
		entry.Cluster = utils.CONFIG.Kubernetes.ClusterName
		entry.Namespace = podMetrics.Namespace
		entry.PodName = podMetrics.Name
		entry.StartTime = pod.Status.StartTime.Format(time.RFC3339)
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

func getKubeConfig() string {
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	} else {
		kubeconfig = ""
	}
	return kubeconfig
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

func MoUpdateLabels(labels *map[string]string, namespaceId *string, stage *dtos.K8sStageDto, service *dtos.K8sServiceDto) map[string]string {
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
		resultingLabels["mo-service-id-short"] = service.ShortId
		resultingLabels["mo-service-k8sname"] = service.K8sName
	}
	if stage != nil {
		resultingLabels["mo-stage-id"] = stage.Id
		resultingLabels["mo-stage-k8sname"] = stage.K8sName
	}
	if namespaceId != nil {
		resultingLabels["mo-namespace-id"] = *namespaceId
	}

	return resultingLabels
}
