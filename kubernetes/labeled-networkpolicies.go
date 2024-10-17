package kubernetes

import (
	"context"
	"fmt"
	"mogenius-k8s-manager/dtos"
	"mogenius-k8s-manager/utils"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	punqUtils "github.com/mogenius/punq/utils"

	v1Core "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// all policies
	NetpolLabel string = "mogenius-network-policy"

	// deny policy
	DenyAllNetPolName string = "deny-all"
	MarkerLabel              = "using-" + DenyAllNetPolName

	// allow policies
	PoliciesLabelPrefix string = "mo-netpol"
)

func AttachLabeledNetworkPolicy(controllerName string,
	controllerType dtos.K8sServiceControllerEnum,
	namespaceName string,
	labelPolicy dtos.K8sLabeledNetworkPolicyDto,
) error {
	client := GetAppClient()
	label := getNetworkPolicyLabel(labelPolicy)

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err := client.Deployments(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		if deployment.Spec.Template.ObjectMeta.Labels == nil {
			deployment.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		if deployment.ObjectMeta.Labels == nil {
			deployment.ObjectMeta.Labels = make(map[string]string)
		}
		deployment.Spec.Template.ObjectMeta.Labels[label] = "true"
		deployment.ObjectMeta.Labels[label] = "true"

		_, err = client.Deployments(namespaceName).Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonset, err := client.DaemonSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		if daemonset.Spec.Template.ObjectMeta.Labels == nil {
			daemonset.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		if daemonset.ObjectMeta.Labels == nil {
			daemonset.ObjectMeta.Labels = make(map[string]string)
		}
		daemonset.Spec.Template.ObjectMeta.Labels[label] = "true"
		daemonset.ObjectMeta.Labels[label] = "true"

		_, err = client.DaemonSets(namespaceName).Update(context.TODO(), daemonset, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulset, err := client.StatefulSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		if statefulset.Spec.Template.ObjectMeta.Labels == nil {
			statefulset.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		if statefulset.ObjectMeta.Labels == nil {
			statefulset.ObjectMeta.Labels = make(map[string]string)
		}
		statefulset.Spec.Template.ObjectMeta.Labels[label] = "true"
		statefulset.ObjectMeta.Labels[label] = "true"

		_, err = client.StatefulSets(namespaceName).Update(context.TODO(), statefulset, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	default:
		return fmt.Errorf("unsupported controller type %s", controllerType)
	}
	return nil
}

func DetachLabeledNetworkPolicy(controllerName string,
	controllerType dtos.K8sServiceControllerEnum,
	namespaceName string,
	labelPolicy dtos.K8sLabeledNetworkPolicyDto,
) error {
	client := GetAppClient()
	label := getNetworkPolicyLabel(labelPolicy)

	switch controllerType {
	case dtos.DEPLOYMENT:
		deployment, err := client.Deployments(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		delete(deployment.Spec.Template.ObjectMeta.Labels, label)
		_, err = client.Deployments(namespaceName).Update(context.TODO(), deployment, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.DAEMON_SET:
		daemonset, err := client.DaemonSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		delete(daemonset.Spec.Template.ObjectMeta.Labels, label)
		_, err = client.DaemonSets(namespaceName).Update(context.TODO(), daemonset, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	case dtos.STATEFUL_SET:
		statefulset, err := client.StatefulSets(namespaceName).Get(context.TODO(), controllerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
		delete(statefulset.Spec.Template.ObjectMeta.Labels, label)
		_, err = client.StatefulSets(namespaceName).Update(context.TODO(), statefulset, MoUpdateOptions())
		if err != nil {
			return fmt.Errorf("AttachLabeledNetworkPolicy ERROR: %s", err)
		}
	default:
		return fmt.Errorf("unsupported controller type %s", controllerType)
	}
	return nil
}

func EnsureLabeledNetworkPolicy(namespaceName string, labelPolicy dtos.K8sLabeledNetworkPolicyDto) error {
	netpol := punqUtils.InitNetPolService()
	// clean traffic rules
	netpol.Spec.Ingress = []v1.NetworkPolicyIngressRule{}
	netpol.Spec.Egress = []v1.NetworkPolicyEgressRule{}

	netpol.ObjectMeta.Name = labelPolicy.Name
	netpol.ObjectMeta.Namespace = namespaceName
	label := getNetworkPolicyLabel(labelPolicy)
	netpol.Spec.PodSelector.MatchLabels[label] = "true"

	// this label is marking all netpols that "need" a deny-all rule
	netpol.ObjectMeta.Labels = map[string]string{MarkerLabel: "true"}
	// general label for all mogenius netpols
	netpol.ObjectMeta.Labels[NetpolLabel] = "true"

	port := intstr.FromInt32(int32(labelPolicy.Port))
	var proto v1Core.Protocol

	switch labelPolicy.PortType {
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

	err := ensureDenyAllRule(namespaceName, netpol, labelPolicy)
	if err != nil {
		return err
	}
	return nil
}

func getNetworkPolicyLabel(labelPolicy dtos.K8sLabeledNetworkPolicyDto) string {
	return strings.ToLower(
		fmt.Sprintf("%s-%s-%s", PoliciesLabelPrefix, labelPolicy.Name, labelPolicy.Type),
	)
}

func ensureDenyAllRule(namespaceName string, netpol v1.NetworkPolicy, labelPolicy dtos.K8sLabeledNetworkPolicyDto) error {
	netPolClient := GetNetworkingClient().NetworkPolicies(namespaceName)

	_, err := netPolClient.Get(context.TODO(), DenyAllNetPolName, metav1.GetOptions{})
	if err != nil {
		K8sLogger.Infof("%s not found, it will be created.", DenyAllNetPolName)

		err = CreateDenyAllNetworkPolicy(namespaceName)
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

func CreateDenyAllNetworkPolicy(namespaceName string) error {
	netpol := punqUtils.InitNetPolService()
	netpol.ObjectMeta.Name = DenyAllNetPolName
	netpol.ObjectMeta.Namespace = namespaceName
	netpol.Spec.PodSelector = metav1.LabelSelector{} // An empty podSelector matches all pods in this namespace.
	netpol.Spec.Ingress = []v1.NetworkPolicyIngressRule{}

	// general label for all mogenius netpols
	netpol.ObjectMeta.Labels = make(map[string]string)
	netpol.ObjectMeta.Labels[NetpolLabel] = "true"

	netPolClient := GetNetworkingClient().NetworkPolicies(namespaceName)
	_, err := netPolClient.Create(context.TODO(), &netpol, MoCreateOptions())
	if err != nil {
		K8sLogger.Errorf("CreateDenyAllNetworkPolicy ERROR: %s", err)
		return err
	}
	return nil
}

func cleanupUnusedDenyAll(namespaceName string) {
	client := GetNetworkingClient()
	netPolClient := client.NetworkPolicies(namespaceName)

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

type NetworkPolicy struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	Port     uint16 `yaml:"port"`
}

func ReadNetworkPolicyPorts() ([]dtos.K8sLabeledNetworkPolicyDto, error) {
	configMap := readDefaultConfigMap()
	ClusterConfigMap := GetConfigMap(configMap.Namespace, configMap.Name)

	var result []dtos.K8sLabeledNetworkPolicyDto
	var policies []NetworkPolicy
	policiesRaw := ClusterConfigMap.Data["network-ports"]
	err := yaml.Unmarshal([]byte(policiesRaw), &policies)
	if err != nil {
		K8sLogger.Errorf("Error unmarshalling YAML: %s\n", err)
		return nil, err
	}
	for _, policy := range policies {
		result = append(result, dtos.K8sLabeledNetworkPolicyDto{
			Name:     policy.Name,
			Type:     dtos.Ingress, // TODO should maybe be deleted
			Port:     uint16(policy.Port),
			PortType: dtos.PortTypeEnum(policy.Protocol),
		})
	}

	return result, nil
}

func RemoveAllConflictingNetworkPolicies(namespaceName string) error {
	netpols, err := ListAllConflictingNetworkPolicies(namespaceName)
	if err != nil {
		return fmt.Errorf("failed to list all network policies: %v", err)
	}

	client := GetNetworkingClient()
	netPolClient := client.NetworkPolicies(namespaceName)

	errors := []error{}
	for _, netpol := range netpols.Items {
		err = netPolClient.Delete(context.TODO(), netpol.Name, metav1.DeleteOptions{})
		if err != nil {
			K8sLogger.Errorf("cleanupNetworkPolicies ERROR: %s", err)
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("failed to remove all network policies: %v", errors)
	}
	return nil
}

func ListAllConflictingNetworkPolicies(namespaceName string) (*v1.NetworkPolicyList, error) {
	client := GetNetworkingClient()
	netPolClient := client.NetworkPolicies(namespaceName)

	netpols, err := netPolClient.List(context.TODO(), metav1.ListOptions{
		LabelSelector: NetpolLabel + "!=true",
	})
	if err != nil {
		K8sLogger.Errorf("cleanupNetworkPolicies ERROR: %s", err)
		return nil, nil
	}
	return netpols, err
}
