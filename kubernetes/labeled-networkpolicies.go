package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/utils"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	punqUtils "github.com/mogenius/punq/utils"
	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DenyAllNetPolName string = "deny-all"
	MarkerLabel              = "using-" + DenyAllNetPolName
)

func CreateNetworkPolicyWithLabel(namespace dtos.K8sNamespaceDto, labelPolicy dtos.K8sLabeledNetworkPolicies) error {
	netpol := punqUtils.InitNetPolService()
	// clean traffic rules
	netpol.Spec.Ingress = []v1.NetworkPolicyIngressRule{}
	netpol.Spec.Egress = []v1.NetworkPolicyEgressRule{}

	netpol.ObjectMeta.Name = labelPolicy.Name
	netpol.ObjectMeta.Namespace = namespace.Name
	label := fmt.Sprintf("mo-netpol-%s-%s", labelPolicy.Name, labelPolicy.Type)
	netpol.Spec.PodSelector.MatchLabels[label] = "true"

	// this label is marking all netpols that "need" a deny-all rule
	netpol.ObjectMeta.Labels = map[string]string{MarkerLabel: "true"}

	for _, aPort := range labelPolicy.Ports {
		port := intstr.FromInt32(int32(aPort.Port))
		var proto v1Core.Protocol

		switch aPort.PortType {
		case "UDP":
			proto = v1Core.ProtocolUDP
		case "SCTP":
			proto = v1Core.ProtocolSCTP
		default:
			proto = v1Core.ProtocolTCP
		}

		if labelPolicy.Type == dtos.Ingress {
			var rule v1.NetworkPolicyIngressRule = v1.NetworkPolicyIngressRule{}
			rule.From = append(rule.From, v1.NetworkPolicyPeer{
				IPBlock: &v1.IPBlock{
					CIDR: "0.0.0.0/0",
				},
			})
			rule.Ports = append(rule.Ports, v1.NetworkPolicyPort{
				Port: &port, Protocol: &proto,
			})
			netpol.Spec.Ingress = append(netpol.Spec.Ingress, rule)
		} else {
			var rule v1.NetworkPolicyEgressRule = v1.NetworkPolicyEgressRule{}
			rule.To = append(rule.To, v1.NetworkPolicyPeer{
				IPBlock: &v1.IPBlock{
					CIDR: "0.0.0.0/0",
				},
			})
			rule.Ports = append(rule.Ports, v1.NetworkPolicyPort{
				Port: &port, Protocol: &proto,
			})
			netpol.Spec.Egress = append(netpol.Spec.Egress, rule)
		}
	}

	netPolClient := GetNetworkingClient().NetworkPolicies(namespace.Name)

	_, err := netPolClient.Get(context.TODO(), DenyAllNetPolName, metav1.GetOptions{})
	if err != nil {
		K8sLogger.Infof("%s not found, it will be created.", DenyAllNetPolName)

		err = CreateDenyAllNetworkPolicy(namespace)
		if err != nil {
			K8sLogger.Errorf("ERROR creating: %s:  %v , abort NetPol creation!", DenyAllNetPolName, err.Error())
			return err
		}
	}
	_, err = netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
	if err != nil {
		K8sLogger.Errorf("CreateNetworkPolicyServiceWithLabel ERROR: %s, trying to create labelPolicy %v ", err.Error(), labelPolicy)
		return err
	}
	return nil
}

func CreateDenyAllNetworkPolicy(namespace dtos.K8sNamespaceDto) error {
	netpol := punqUtils.InitNetPolService()
	netpol.ObjectMeta.Name = DenyAllNetPolName
	netpol.ObjectMeta.Namespace = namespace.Name
	netpol.Spec.PodSelector = metav1.LabelSelector{} // An empty podSelector matches all pods in this namespace.
	netpol.Spec.Ingress = []v1.NetworkPolicyIngressRule{}

	netPolClient := GetNetworkingClient().NetworkPolicies(namespace.Name)
	_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
	if err != nil {
		K8sLogger.Errorf("CreateDenyAllNetworkPolicy ERROR: %s", err)
		return err
	}
	return nil
}

func cleanupUnusedDenyAll(namespace dtos.K8sNamespaceDto) {
	client := GetNetworkingClient()
	netPolClient := client.NetworkPolicies(namespace.Name)

	// list all network policies and find all that have the marker label
	netpols, err := netPolClient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: MarkerLabel + "=true",
	})
	if err != nil {
		K8sLogger.Errorf("cleanupNetworkPolicies ERROR: %s", err)
		return
	}

	if len(netpols.Items) == 0 {
		err = netPolClient.Delete(context.TODO(), DenyAllNetPolName, metav1.DeleteOptions{})
		if err != nil {
			K8sLogger.Errorf("cleanupNetworkPolicies ERROR: %s", err)
		}
	}
}

func InitNetworkPolicyConfigMap() error {
	configMap := readDefaultConfigMap()

	return EnsureConfigMapExists(configMap.Namespace, *configMap)
	// return WriteConfigMap(configMap.Namespace, configMap.Name, yamlString, configMap.Labels)
}

func readDefaultConfigMap() *v1Core.ConfigMap {
	yamlString := utils.InitNetworkPolicyDefaultsYaml()

	// marshal yaml to struct
	var configMap v1Core.ConfigMap
	err := yaml.Unmarshal([]byte(yamlString), &configMap)
	if err != nil {
		K8sLogger.Errorf("InitNetworkPolicyConfigMap ERROR: %s", err)
		return nil
	}
	return &configMap
}

type Port struct {
	Protocol string `yaml:"protocol"`
	Port     int    `yaml:"port"`
}

type NetworkPolicy struct {
	Name  string `yaml:"name"`
	Ports []Port `yaml:"ports"`
}

func ReadNetworkPolicyPorts() []dtos.K8sLabeledNetworkPolicies {
	configMap := readDefaultConfigMap()

	ClusterConfigMap := GetConfigMap(configMap.Namespace, configMap.Name)

	var result []dtos.K8sLabeledNetworkPolicies
	for key, valueYaml := range ClusterConfigMap.Data {
		var policies []NetworkPolicy
		err := yaml.Unmarshal([]byte(valueYaml), &policies)
		if err != nil {
			fmt.Printf("Error unmarshalling YAML: %s\n", err)
		}
		for _, policy := range policies {
			for _, port := range policy.Ports {
				result = append(result, dtos.K8sLabeledNetworkPolicies{
					Name: policy.Name,
					Type: dtos.K8sNetworkPolicyType(key),
					Ports: []dtos.K8sLabeledPortDto{
						{
							Port:     int32(port.Port),
							PortType: dtos.PortTypeEnum(port.Protocol),
						},
					},
				})
			}
		}
	}

	return result
}
