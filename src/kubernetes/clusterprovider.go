package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"mogenius-operator/src/utils"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// nodeFacts is the normalised view of a single node that the detection rules
// are evaluated against. Everything is lower-cased once here so the rules can
// compare without worrying about casing.
type nodeFacts struct {
	name           string
	providerID     string
	kubeletVersion string
	osImage        string
	// keys holds label AND annotation keys (lower-cased) mapped to their
	// lower-cased value. Distros mark their nodes with either, and which one
	// they pick has changed between versions (k3s moved its markers from
	// labels to annotations), so both are treated the same.
	keys   map[string]string
	images []core.ContainerImage
}

func newNodeFacts(node *core.Node) nodeFacts {
	facts := nodeFacts{
		name:           strings.ToLower(node.GetName()),
		providerID:     strings.ToLower(node.Spec.ProviderID),
		kubeletVersion: strings.ToLower(node.Status.NodeInfo.KubeletVersion),
		osImage:        strings.ToLower(node.Status.NodeInfo.OSImage),
		keys:           map[string]string{},
		images:         node.Status.Images,
	}
	for key, value := range node.GetLabels() {
		facts.keys[strings.ToLower(key)] = strings.ToLower(value)
	}
	for key, value := range node.GetAnnotations() {
		facts.keys[strings.ToLower(key)] = strings.ToLower(value)
	}
	return facts
}

// hasKeyPrefix reports whether any label or annotation key starts with one of
// the given prefixes. This is the usual shape of a vendor marker
// ("eks.amazonaws.com/…"), and anchoring at the start avoids the false
// positives a plain substring search produces.
func (n nodeFacts) hasKeyPrefix(prefixes ...string) bool {
	for key := range n.keys {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		}
	}
	return false
}

// hasKeySubstring is the loose variant, for vendors whose marker is not at the
// start of the key ("kubernetes.civo.com/node-pool").
func (n nodeFacts) hasKeySubstring(substrings ...string) bool {
	for key := range n.keys {
		for _, substring := range substrings {
			if strings.Contains(key, substring) {
				return true
			}
		}
	}
	return false
}

func (n nodeFacts) hasKey(keys ...string) bool {
	for _, key := range keys {
		if _, ok := n.keys[key]; ok {
			return true
		}
	}
	return false
}

func (n nodeFacts) providerIDHasPrefix(prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(n.providerID, prefix) {
			return true
		}
	}
	return false
}

func (n nodeFacts) kubeletContains(substrings ...string) bool {
	for _, substring := range substrings {
		if strings.Contains(n.kubeletVersion, substring) {
			return true
		}
	}
	return false
}

func (n nodeFacts) nameIs(names ...string) bool {
	for _, name := range names {
		if n.name == name {
			return true
		}
	}
	return false
}

func (n nodeFacts) namePrefix(prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(n.name, prefix) {
			return true
		}
	}
	return false
}

func (n nodeFacts) imageContains(substrings ...string) bool {
	for _, image := range n.images {
		for _, imageName := range image.Names {
			lowered := strings.ToLower(imageName)
			for _, substring := range substrings {
				if strings.Contains(lowered, substring) {
					return true
				}
			}
		}
	}
	return false
}

// providerRule is one detection heuristic.
type providerRule struct {
	provider utils.KubernetesProvider
	match    func(n nodeFacts) bool
}

// providerRules is ordered from most specific to least specific: the first
// rule that matches ANY node in the cluster wins. Ordering carries meaning
// wherever one distro is layered on another -- k3d nodes also look like k3s,
// GKE on-prem nodes also carry GKE labels, PSKE is a Gardener flavour -- so the
// narrower rule has to come first.
var providerRules = []providerRule{
	// --- vendor distributions -------------------------------------------
	{utils.OPEN_SHIFT, func(n nodeFacts) bool {
		// The old rule additionally required node-role…/master, which meant a
		// cluster whose master nodes were not visible was never recognised.
		return n.hasKey("node.openshift.io/os_id") ||
			n.hasKeyPrefix("machineconfiguration.openshift.io/", "machine.openshift.io/")
	}},

	// --- managed cloud offerings ----------------------------------------
	{utils.GKE_ON_PREM, func(n nodeFacts) bool {
		return n.hasKeyPrefix("cloud.google.com/gke-on-prem")
	}},
	{utils.GKE, func(n nodeFacts) bool {
		return n.hasKeyPrefix("cloud.google.com/gke-", "node.gke.io/", "topology.gke.io/") ||
			n.kubeletContains("-gke.")
	}},
	{utils.EKS, func(n nodeFacts) bool {
		return n.hasKeyPrefix("eks.amazonaws.com/", "alpha.eksctl.io/") ||
			n.kubeletContains("-eks-")
	}},
	{utils.AKS, func(n nodeFacts) bool {
		// The old rule looked for kubernetes.azure.com/role, which AKS only
		// sets on some node types; the prefix covers agentpool, mode,
		// cluster, os-sku and the virtual-node markers as well.
		return n.hasKeyPrefix("kubernetes.azure.com/") ||
			(n.providerIDHasPrefix("azure://") && n.hasKey("agentpool"))
	}},
	{utils.DOKS, func(n nodeFacts) bool {
		return n.hasKeyPrefix("doks.digitalocean.com/")
	}},
	{utils.LINODE, func(n nodeFacts) bool {
		return n.hasKeyPrefix("lke.linode.com/") || n.hasKeySubstring("linode-lke")
	}},
	{utils.IBM, func(n nodeFacts) bool {
		return n.hasKeyPrefix("ibm-cloud.kubernetes.io/") || n.kubeletContains("+iks")
	}},
	{utils.OKE, func(n nodeFacts) bool {
		return n.hasKeyPrefix("oke.oraclecloud.com/", "oci.oraclecloud.com/") ||
			n.providerIDHasPrefix("ocid1.instance.")
	}},
	{utils.ACK, func(n nodeFacts) bool {
		return n.hasKeySubstring("ack.aliyun.com", "alibabacloud.com/") || n.kubeletContains("-aliyun")
	}},
	{utils.HUAWEI, func(n nodeFacts) bool {
		return n.hasKeySubstring("cce.huawei.com")
	}},
	{utils.OTC, func(n nodeFacts) bool {
		return n.hasKeySubstring("-cce") || n.imageContains("cce-addons")
	}},
	{utils.CIVO, func(n nodeFacts) bool {
		return n.hasKeySubstring("civo.com/", "civo-node-pool")
	}},
	{utils.SCALEWAY, func(n nodeFacts) bool {
		return n.hasKeyPrefix("k8s.scaleway.com/") ||
			n.hasKeySubstring("scaleway-kapsule") ||
			n.providerIDHasPrefix("scaleway://")
	}},
	{utils.OVHCLOUD, func(n nodeFacts) bool {
		return n.hasKeySubstring("ovhcloud")
	}},
	{utils.GIANTSWARM, func(n nodeFacts) bool {
		return n.hasKeySubstring("giantswarm.io")
	}},
	{utils.IONOS, func(n nodeFacts) bool {
		return n.providerIDHasPrefix("ionos://") || n.hasKeySubstring("ionos")
	}},
	{utils.PLUSSERVER, func(n nodeFacts) bool {
		// PSKE is built on Gardener, so it has to be checked before it.
		return n.imageContains("pluscloudopen") || n.hasKeySubstring("pluscloudopen")
	}},
	{utils.GARDENER, func(n nodeFacts) bool {
		return n.hasKeyPrefix("worker.gardener.cloud/", "gardener.cloud/", "topology.gardener.cloud/")
	}},
	{utils.VMWARE, func(n nodeFacts) bool {
		return n.hasKeyPrefix("vmware-system-vmware.io/", "run.tanzu.vmware.com/", "node.vmware.com/") ||
			n.kubeletContains("+vmware") ||
			n.imageContains("vmware.com/tkg/kube-apiserver")
	}},

	// --- platforms / vendors on top of any infrastructure ---------------
	{utils.NIRMATA, func(n nodeFacts) bool { return n.hasKeySubstring("nirmata.io") }},
	{utils.PF9, func(n nodeFacts) bool { return n.hasKeySubstring("platform9.com") }},
	{utils.NKS, func(n nodeFacts) bool { return n.hasKeySubstring("nks.netapp.io") }},
	{utils.APPSCODE, func(n nodeFacts) bool { return n.hasKeySubstring("appscode.com") }},
	{utils.LOFT, func(n nodeFacts) bool { return n.hasKeySubstring("loft.sh") }},
	{utils.SPECTROCLOUD, func(n nodeFacts) bool { return n.hasKeySubstring("spectrocloud.com") }},
	{utils.DIAMANTI, func(n nodeFacts) bool { return n.hasKeySubstring("diamanti.com") }},

	// --- self-installed distributions -----------------------------------
	{utils.K3D, func(n nodeFacts) bool {
		// k3d nodes carry the full k3s marker set, so this has to win over K3S.
		return n.namePrefix("k3d-") || strings.HasPrefix(n.providerID, "k3s://k3d-")
	}},
	{utils.K3S, func(n nodeFacts) bool {
		return n.providerIDHasPrefix("k3s://") ||
			n.hasKeyPrefix("k3s.io/") ||
			n.kubeletContains("+k3s")
	}},
	{utils.RKE, func(n nodeFacts) bool {
		return n.hasKeyPrefix("rke.cattle.io/", "rke2.io/", "node.cattle.io/", "io.rancher.os/") ||
			n.kubeletContains("+rke2")
	}},
	{utils.MICROK8S, func(n nodeFacts) bool {
		return n.hasKey("microk8s.io/cluster") || n.hasKeyPrefix("microk8s.io/")
	}},
	{utils.KIND, func(n nodeFacts) bool {
		// kind sets no vendor label at all -- spec.providerID is the only
		// reliable marker, and it is what Docker Desktop's kind-based
		// Kubernetes shows too (kind://docker/desktop/desktop-control-plane).
		return n.providerIDHasPrefix("kind://") ||
			n.hasKeyPrefix("io.x-k8s.io/") ||
			n.hasKey("io.k8s.sigs.kind/role")
	}},
	{utils.MINIKUBE, func(n nodeFacts) bool {
		return n.hasKeyPrefix("minikube.k8s.io/") ||
			n.nameIs("minikube") ||
			n.namePrefix("minikube-m")
	}},
	{utils.DOCKER_DESKTOP, func(n nodeFacts) bool {
		return n.nameIs("docker-desktop", "docker-for-desktop")
	}},
	{utils.TALOS, func(n nodeFacts) bool {
		return strings.Contains(n.osImage, "talos") || n.hasKeyPrefix("talos.dev/")
	}},
}

// genericInfrastructureRules run only after every distribution rule has failed.
// A bare cloud providerID means "somebody rolled their own Kubernetes on these
// VMs" -- still far more useful than reporting vanilla.
var genericInfrastructureRules = []providerRule{
	{utils.KUBEADM_ON_PREM_AWS, func(n nodeFacts) bool { return n.providerIDHasPrefix("aws://") }},
	{utils.KUBEADM_ON_PREM_GCP, func(n nodeFacts) bool { return n.providerIDHasPrefix("gce://") }},
	{utils.KUBEADM_ON_PREM_AZURE, func(n nodeFacts) bool { return n.providerIDHasPrefix("azure://") }},
	{utils.KUBEADM_ON_PREM_HETZNER, func(n nodeFacts) bool { return n.providerIDHasPrefix("hcloud://") }},
	{utils.KUBEADM_ON_PREM_DIGITALOCEAN, func(n nodeFacts) bool { return n.providerIDHasPrefix("digitalocean://") }},
	{utils.KUBEADM_ON_PREM_LINODE, func(n nodeFacts) bool { return n.providerIDHasPrefix("linode://") }},
	{utils.VMWARE, func(n nodeFacts) bool { return n.providerIDHasPrefix("vsphere://") }},
}

func GuessClusterProvider() (utils.KubernetesProvider, error) {
	clientset := clientProvider.K8sClientSet()
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return utils.UNKNOWN, fmt.Errorf("list nodes: %w", err)
	}

	return GuessClusterProviderFromNodeList(nodes)
}

// GuessClusterProviderFromNodeList determines the flavour of Kubernetes from
// the node objects. Every rule is checked against every node instead of
// deciding on the first node alone: on a mixed cluster the marker often only
// exists on a subset of the nodes (managed control planes that are not listed,
// Fargate and virtual-kubelet nodes, node pools created by different tooling),
// and the old first-node-wins loop reported those clusters as vanilla.
func GuessClusterProviderFromNodeList(nodes *core.NodeList) (utils.KubernetesProvider, error) {
	if nodes == nil || len(nodes.Items) == 0 {
		return utils.UNKNOWN, fmt.Errorf("cannot guess cluster provider: node list is empty")
	}

	facts := make([]nodeFacts, 0, len(nodes.Items))
	for i := range nodes.Items {
		facts = append(facts, newNodeFacts(&nodes.Items[i]))
	}

	for _, ruleSet := range [][]providerRule{providerRules, genericInfrastructureRules} {
		for _, rule := range ruleSet {
			for _, node := range facts {
				if rule.match(node) {
					return rule.provider, nil
				}
			}
		}
	}

	k8sLogger.Info(
		"This cluster's provider is unknown. Falling back to vanilla K8S.",
		"nodeName", facts[0].name,
		"providerID", facts[0].providerID,
		"kubeletVersion", facts[0].kubeletVersion,
		"osImage", facts[0].osImage,
	)
	return utils.VANILLA_K8S, nil
}
