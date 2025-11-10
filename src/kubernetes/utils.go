package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/src/assert"
	cfg "mogenius-k8s-manager/src/config"
	"mogenius-k8s-manager/src/store"
	"mogenius-k8s-manager/src/utils"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	version2 "k8s.io/apimachinery/pkg/version"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
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

func KubernetesVersion() *version2.Info {
	clientset := clientProvider.K8sClientSet()
	info, err := clientset.Discovery().ServerVersion()
	if err != nil {
		k8sLogger.Error("Error KubernetesVersion", "error", err)
		return nil
	}
	return info
}

func MoCreateOptions(config cfg.ConfigModule) metav1.CreateOptions {
	return metav1.CreateOptions{
		FieldManager: GetOwnDeploymentName(config),
	}
}

func GetOwnDeploymentName(config cfg.ConfigModule) string {
	return config.Get("OWN_DEPLOYMENT_NAME")
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

func ListNodeMetricss() []metricsv1beta1.NodeMetrics {
	provider, err := NewKubeProviderMetrics()
	if provider == nil || err != nil {
		k8sLogger.Error("ListNodeMetricss", "error", err.Error())
		return []metricsv1beta1.NodeMetrics{}
	}

	nodeMetricsList, err := provider.ClientSet.MetricsV1beta1().NodeMetricses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return []metricsv1beta1.NodeMetrics{}
	}
	return nodeMetricsList.Items
}

func StorageClassForClusterProvider(clusterProvider utils.KubernetesProvider) string {
	var nfsStorageClassStr string = ""

	// 1. WE TRY TO GET THE DEFAULT STORAGE CLASS
	clientset := clientProvider.K8sClientSet()
	storageClasses, err := clientset.StorageV1().StorageClasses().List(context.Background(), metav1.ListOptions{})
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

func GetLabelValue(labels map[string]string, labelKey string) (string, error) {
	if labels == nil {
		return "", fmt.Errorf("labels are nil")
	}

	if val, ok := labels[labelKey]; ok {
		return val, nil
	}

	return "", fmt.Errorf("label value for key:'%s' not found", labelKey)
}

func ContainsLabelKey(labels map[string]string, key string) bool {
	if labels == nil {
		return false
	}

	_, ok := labels[key]
	return ok
}

func GuessClusterProvider() (utils.KubernetesProvider, error) {
	clientset := clientProvider.K8sClientSet()
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return utils.SELF_HOSTED, err
	}

	return GuessCluserProviderFromNodeList(nodes)
}

func GuessCluserProviderFromNodeList(nodes *core.NodeList) (utils.KubernetesProvider, error) {

	for _, node := range nodes.Items {
		nodeInfo := map[string]string{}
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
		} else if strings.HasPrefix(strings.ToLower(node.Name), "k3d-") {
			return utils.K3D, nil
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
		} else if LabelsContain(labelsAndAnnotations, "cloud.google.com/gke-on-prem") {
			return utils.GKE_ON_PREM, nil
		} else if LabelsContain(labelsAndAnnotations, "rke.cattle.io") {
			return utils.RKE, nil
		} else if ImagesContain(node.Status.Images, "pluscloudopen") {
			return utils.PLUSSERVER, nil
		} else {
			k8sLogger.Info("This cluster's provider is unknown. Falling back to vanilla K8S.")
			return utils.VANILLA_K8S, nil
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
	deployments := store.GetDeployments("*", "*")

	for _, deployment := range deployments {
		for _, label := range deployment.Labels {
			if label == "metrics-server" {
				if deployment.Status.UnavailableReplicas > 0 {
					return false, "", fmt.Errorf("metrics-server installed but not running")
				}
				return true, deployment.Spec.Template.Spec.Containers[0].Image, nil
			}
		}
		if deployment.Name == "metrics-server" {
			if deployment.Status.UnavailableReplicas > 0 {
				return false, "", fmt.Errorf("metrics-server installed but not running")
			}
			return true, deployment.Spec.Template.Spec.Containers[0].Image, nil
		}
	}

	return false, "", fmt.Errorf("no metrics-server found")
}

func DetermineIngressControllerType() (IngressType, error) {
	ingressClasses := store.GetIngressClasses()

	if len(ingressClasses) > 1 {
		return MULTIPLE, fmt.Errorf("multiple ingress controllers found")
	}

	if len(ingressClasses) == 0 {
		return NONE, fmt.Errorf("no ingress controller found")
	}

	unknownController := ""
	for _, ingressClass := range ingressClasses {
		switch ingressClass.Spec.Controller {
		case "k8s.io/ingress-nginx", "nginx.org/ingress-controller":
			return NGINX, nil
		case "traefik.io/ingress-controller":
			return TRAEFIK, nil
		default:
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
