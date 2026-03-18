package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	cfg "mogenius-operator/src/config"
	"mogenius-operator/src/store"
	"mogenius-operator/src/utils"
	"strconv"
	"strings"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/tools/remotecommand"
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

func ServiceForNfsVolume(volumeNamespace string, volumeName string) *core.Service {
	services := AllServices(volumeNamespace)
	for _, srv := range services {
		if strings.Contains(srv.Name, fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName)) {
			return &srv
		}
	}
	return nil
}

// NfsDiskUsage returns free/used/total bytes of the NFS export directory by
// executing `df -B1 /exports` inside the NFS server pod – no local mount needed.
func NfsDiskUsage(volumeNamespace string, volumeName string) (free, used, total uint64, err error) {
	output, execErr := ExecInNfsPod(volumeNamespace, volumeName, []string{"df", "-B1", "/exports"}, nil)
	if execErr != nil {
		return 0, 0, 0, fmt.Errorf("df exec failed: %w", execErr)
	}

	// df -B1 output (header + data line):
	// Filesystem     1-blocks      Used Available Use% Mounted on
	// overlay        5368709120  102400  5266309120   2% /exports
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return 0, 0, 0, fmt.Errorf("unexpected df output: %q", output)
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 4 {
		return 0, 0, 0, fmt.Errorf("unexpected df output format: %q", lines[len(lines)-1])
	}

	total, err = strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse total: %w", err)
	}
	used, err = strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse used: %w", err)
	}
	free, err = strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse free: %w", err)
	}

	return free, used, total, nil
}

// ExecInNfsPod executes a command inside the NFS server pod for the given volume and
// returns the buffered stdout. stdin may be nil.
func ExecInNfsPod(volumeNamespace, volumeName string, command []string, stdin io.Reader) (string, error) {
	podNames := AllPodNamesForLabel(volumeNamespace, "app", fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName))
	if len(podNames) == 0 {
		return "", fmt.Errorf("NFS server pod not found for %s/%s", volumeNamespace, volumeName)
	}
	var stdout bytes.Buffer
	err := execInNfsPodStream(volumeNamespace, podNames[0], command, stdin, &stdout)
	return stdout.String(), err
}

// ExecInNfsPodToWriter streams exec stdout directly into the provided writer. stdin may be nil.
func ExecInNfsPodToWriter(volumeNamespace, volumeName string, command []string, stdin io.Reader, stdout io.Writer) error {
	podNames := AllPodNamesForLabel(volumeNamespace, "app", fmt.Sprintf("%s-%s", utils.NFS_POD_PREFIX, volumeName))
	if len(podNames) == 0 {
		return fmt.Errorf("NFS server pod not found for %s/%s", volumeNamespace, volumeName)
	}
	return execInNfsPodStream(volumeNamespace, podNames[0], command, stdin, stdout)
}

func execInNfsPodStream(namespace, podName string, command []string, stdin io.Reader, stdout io.Writer) error {
	clientset := clientProvider.K8sClientSet()
	restConfig := clientProvider.ClientConfig()

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Param("container", "nfs-server").
		Param("stdout", "true").
		Param("stderr", "true").
		Param("tty", "false")

	for _, arg := range command {
		req.Param("command", arg)
	}
	if stdin != nil {
		req.Param("stdin", "true")
	}

	executor, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	opts := remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: &stderr,
	}
	if stdin != nil {
		opts.Stdin = stdin
	}

	err = executor.StreamWithContext(context.Background(), opts)
	if err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
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
