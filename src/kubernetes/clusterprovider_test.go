package kubernetes

import (
	"io"
	"log/slog"
	"testing"

	"mogenius-operator/src/utils"

	"github.com/stretchr/testify/assert"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type nodeSpec struct {
	name        string
	providerID  string
	labels      map[string]string
	annotations map[string]string
	kubelet     string
	osImage     string
	images      []string
}

func nodeList(specs ...nodeSpec) *core.NodeList {
	list := &core.NodeList{}
	for _, spec := range specs {
		images := []core.ContainerImage{}
		for _, image := range spec.images {
			images = append(images, core.ContainerImage{Names: []string{image}})
		}
		list.Items = append(list.Items, core.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        spec.name,
				Labels:      spec.labels,
				Annotations: spec.annotations,
			},
			Spec: core.NodeSpec{ProviderID: spec.providerID},
			Status: core.NodeStatus{
				NodeInfo: core.NodeSystemInfo{KubeletVersion: spec.kubelet, OSImage: spec.osImage},
				Images:   images,
			},
		})
	}
	return list
}

// commonLabels are the labels every node carries; they must never be enough to
// identify a provider on their own.
func commonLabels(hostname string, extra map[string]string) map[string]string {
	labels := map[string]string{
		"beta.kubernetes.io/arch": "amd64",
		"beta.kubernetes.io/os":   "linux",
		"kubernetes.io/arch":      "amd64",
		"kubernetes.io/os":        "linux",
		"kubernetes.io/hostname":  hostname,
	}
	for key, value := range extra {
		labels[key] = value
	}
	return labels
}

func TestGuessClusterProviderFromNodeList(t *testing.T) {
	k8sLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name     string
		nodes    *core.NodeList
		expected utils.KubernetesProvider
	}{
		{
			// Captured from Docker Desktop with the kind-based Kubernetes
			// backend: no vendor label anywhere, only spec.providerID says
			// kind. The node is NOT called "kind-*", it is called "desktop-*".
			name: "docker desktop provisioned via kind",
			nodes: nodeList(
				nodeSpec{
					name:       "desktop-control-plane",
					providerID: "kind://docker/desktop/desktop-control-plane",
					labels: commonLabels("desktop-control-plane", map[string]string{
						"node-role.kubernetes.io/control-plane":                   "",
						"node.kubernetes.io/exclude-from-external-load-balancers": "",
					}),
					annotations: map[string]string{"node.alpha.kubernetes.io/ttl": "0"},
					kubelet:     "v1.36.1",
					osImage:     "Debian GNU/Linux 13 (trixie)",
				},
				nodeSpec{
					name:       "desktop-worker",
					providerID: "kind://docker/desktop/desktop-worker",
					labels:     commonLabels("desktop-worker", nil),
					kubelet:    "v1.36.1",
					osImage:    "Debian GNU/Linux 13 (trixie)",
				},
			),
			expected: utils.KIND,
		},
		{
			name: "plain kind cluster",
			nodes: nodeList(nodeSpec{
				name:       "kind-control-plane",
				providerID: "kind://docker/kind/kind-control-plane",
				labels:     commonLabels("kind-control-plane", nil),
				kubelet:    "v1.31.0",
			}),
			expected: utils.KIND,
		},
		{
			name: "docker desktop with the kubeadm backend",
			nodes: nodeList(nodeSpec{
				name:    "docker-desktop",
				labels:  commonLabels("docker-desktop", nil),
				kubelet: "v1.30.2",
			}),
			expected: utils.DOCKER_DESKTOP,
		},
		{
			// Captured from gke-mo-gke-03.
			name: "gke",
			nodes: nodeList(nodeSpec{
				name:       "gke-mo-gke-03-pool-3-072d5664-9u60",
				providerID: "gce://infra-rhino-376010/europe-west3-b/gke-mo-gke-03-pool-3-072d5664-9u60",
				labels: commonLabels("gke-mo-gke-03-pool-3-072d5664-9u60", map[string]string{
					"cloud.google.com/gke-nodepool": "pool-3",
					"topology.gke.io/zone":          "europe-west3-b",
				}),
				annotations: map[string]string{"node.gke.io/last-applied-node-labels": ""},
				kubelet:     "v1.35.7-gke.1222000",
				osImage:     "Container-Optimized OS from Google",
			}),
			expected: utils.GKE,
		},
		{
			// Captured from home-k3s: the k3s marker is an ANNOTATION, and the
			// providerID scheme is k3s.
			name: "k3s single node",
			nodes: nodeList(nodeSpec{
				name:       "bene-pc",
				providerID: "k3s://bene-pc",
				labels: commonLabels("bene-pc", map[string]string{
					"node.kubernetes.io/instance-type": "k3s",
				}),
				annotations: map[string]string{"k3s.io/hostname": "bene-pc", "k3s.io/node-args": "[]"},
				kubelet:     "v1.36.2+k3s1",
				osImage:     "CachyOS",
			}),
			expected: utils.K3S,
		},
		{
			name: "k3d wins over k3s",
			nodes: nodeList(nodeSpec{
				name:        "k3d-mycluster-server-0",
				providerID:  "k3s://k3d-mycluster-server-0",
				labels:      commonLabels("k3d-mycluster-server-0", nil),
				annotations: map[string]string{"k3s.io/hostname": "k3d-mycluster-server-0"},
				kubelet:     "v1.30.4+k3s1",
			}),
			expected: utils.K3D,
		},
		{
			// The old implementation returned on the first node, so a cluster
			// whose first node lacks the marker was reported as vanilla.
			name: "marker only present on a later node",
			nodes: nodeList(
				nodeSpec{
					name:    "virtual-node-aci-linux",
					labels:  commonLabels("virtual-node-aci-linux", nil),
					kubelet: "v1.19.10-vk-azure-aci-1.4.8",
				},
				nodeSpec{
					name:       "aks-nodepool1-12345678-vmss000000",
					providerID: "azure:///subscriptions/abc/resourceGroups/mc_rg/providers/Microsoft.Compute/virtualMachineScaleSets/aks-nodepool1/virtualMachines/0",
					labels: commonLabels("aks-nodepool1-12345678-vmss000000", map[string]string{
						"kubernetes.azure.com/agentpool": "nodepool1",
					}),
					kubelet: "v1.30.3",
				},
			),
			expected: utils.AKS,
		},
		{
			name: "eks recognised from the kubelet version alone",
			nodes: nodeList(nodeSpec{
				name:       "ip-10-0-1-23.eu-central-1.compute.internal",
				providerID: "aws:///eu-central-1a/i-0abc123",
				labels:     commonLabels("ip-10-0-1-23.eu-central-1.compute.internal", nil),
				kubelet:    "v1.30.0-eks-036c24b",
			}),
			expected: utils.EKS,
		},
		{
			name: "eks recognised from labels",
			nodes: nodeList(nodeSpec{
				name:       "ip-10-0-1-23.eu-central-1.compute.internal",
				providerID: "aws:///eu-central-1a/i-0abc123",
				labels: commonLabels("ip-10-0-1-23.eu-central-1.compute.internal", map[string]string{
					"eks.amazonaws.com/nodegroup": "ng-1",
				}),
				kubelet: "v1.30.0",
			}),
			expected: utils.EKS,
		},
		{
			// OpenShift was only detected when a master node was in the list.
			name: "openshift worker only",
			nodes: nodeList(nodeSpec{
				name: "worker-0",
				labels: commonLabels("worker-0", map[string]string{
					"node.openshift.io/os_id":        "rhcos",
					"node-role.kubernetes.io/worker": "",
				}),
				kubelet: "v1.29.5+aba1e8d",
			}),
			expected: utils.OPEN_SHIFT,
		},
		{
			name: "gke on prem wins over gke",
			nodes: nodeList(nodeSpec{
				name: "onprem-node-1",
				labels: commonLabels("onprem-node-1", map[string]string{
					"cloud.google.com/gke-on-prem-version": "1.28",
					"cloud.google.com/gke-nodepool":        "pool-1",
				}),
				kubelet: "v1.28.3-gke.1",
			}),
			expected: utils.GKE_ON_PREM,
		},
		{
			name: "rke2",
			nodes: nodeList(nodeSpec{
				name:        "rke2-node-1",
				labels:      commonLabels("rke2-node-1", nil),
				annotations: map[string]string{"rke.cattle.io/external-ip": "1.2.3.4"},
				kubelet:     "v1.30.3+rke2r1",
			}),
			expected: utils.RKE,
		},
		{
			name: "minikube multi node",
			nodes: nodeList(
				nodeSpec{
					name: "minikube",
					labels: commonLabels("minikube", map[string]string{
						"minikube.k8s.io/version": "v1.33.1",
					}),
					kubelet: "v1.30.0",
				},
				nodeSpec{name: "minikube-m02", labels: commonLabels("minikube-m02", nil), kubelet: "v1.30.0"},
			),
			expected: utils.MINIKUBE,
		},
		{
			name: "talos",
			nodes: nodeList(nodeSpec{
				name:    "talos-abc-def",
				labels:  commonLabels("talos-abc-def", nil),
				kubelet: "v1.30.1",
				osImage: "Talos (v1.7.5)",
			}),
			expected: utils.TALOS,
		},
		{
			name: "digital ocean managed",
			nodes: nodeList(nodeSpec{
				name:       "pool-abc-123",
				providerID: "digitalocean://123456",
				labels: commonLabels("pool-abc-123", map[string]string{
					"doks.digitalocean.com/node-pool": "pool-abc",
				}),
				kubelet: "v1.30.2",
			}),
			expected: utils.DOKS,
		},
		{
			// Self-managed Kubernetes on cloud VMs: no distro marker, but the
			// providerID still tells us where it runs. Used to be vanilla.
			name: "self managed on hetzner",
			nodes: nodeList(nodeSpec{
				name:       "cx31-node-1",
				providerID: "hcloud://12345",
				labels:     commonLabels("cx31-node-1", nil),
				kubelet:    "v1.30.1",
			}),
			expected: utils.KUBEADM_ON_PREM_HETZNER,
		},
		{
			name: "vanilla bare metal",
			nodes: nodeList(nodeSpec{
				name:    "node-1",
				labels:  commonLabels("node-1", nil),
				kubelet: "v1.30.1",
				osImage: "Ubuntu 22.04.4 LTS",
			}),
			expected: utils.VANILLA_K8S,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider, err := GuessClusterProviderFromNodeList(test.nodes)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, provider)
		})
	}
}

func TestGuessClusterProviderFromEmptyNodeList(t *testing.T) {
	provider, err := GuessClusterProviderFromNodeList(nodeList())
	assert.Error(t, err)
	assert.Equal(t, utils.UNKNOWN, provider)

	provider, err = GuessClusterProviderFromNodeList(nil)
	assert.Error(t, err)
	assert.Equal(t, utils.UNKNOWN, provider)
}
